package orchestration

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func TestDecidePure(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	snapshot := Snapshot{
		Now: now,
		Frontier: []core.FrontierTask{
			{ID: "T2", Role: "craftsman"},
			{ID: "T1", Role: "craftsman"},
		},
	}
	limits := DecisionLimitsForAuthority(Authority{Enabled: true}, DecisionLimits{MaxRetries: 2})

	first := Decide(snapshot, limits)
	second := Decide(snapshot, limits)
	if first != second {
		t.Fatalf("Decide not deterministic: %#v != %#v", first, second)
	}
	if first.Action != ActionDispatch || first.TaskID != "T1" {
		t.Fatalf("Decide() = %#v, want dispatch T1", first)
	}
	if len(snapshot.Frontier) != 2 || snapshot.Frontier[0].ID != "T2" {
		t.Fatalf("Decide mutated snapshot frontier: %#v", snapshot.Frontier)
	}
}

func TestSense(t *testing.T) {
	now := time.Unix(200, 0).UTC()
	state := core.State{Revision: 7, Phase: "tasks", Records: map[string]json.RawMessage{"x": []byte(`{"ok":true}`)}}
	frontier := []core.FrontierTask{{ID: "T1", Verify: "go test"}}
	leases := []Lease{{TaskID: "T0", WorkerID: "w", ExpiresAt: now.Add(time.Minute)}}

	snapshot := Sense(state, frontier, leases, now)
	frontier[0].ID = "changed"
	leases[0].TaskID = "changed"
	state.Records["x"][2] = 'X'

	if snapshot.Revision != 7 || snapshot.Phase != "tasks" || snapshot.Frontier[0].ID != "T1" || snapshot.Leases[0].TaskID != "T0" {
		t.Fatalf("snapshot not stable: %#v", snapshot)
	}
	if string(snapshot.Records["x"]) != `{"ok":true}` {
		t.Fatalf("record alias leaked: %s", snapshot.Records["x"])
	}
}

func TestBrakes(t *testing.T) {
	now := time.Unix(300, 0).UTC()
	cost := EvaluateBrakes(Snapshot{Now: now, Cost: 11}, DecisionLimits{MaxCost: 10})
	if cost.Action != ActionHalt {
		t.Fatalf("cost brake = %#v, want halt", cost)
	}
	deadline := EvaluateBrakes(Snapshot{Now: now}, DecisionLimits{Deadline: now})
	if deadline.Action != ActionTimeout {
		t.Fatalf("deadline brake = %#v, want timeout", deadline)
	}
}

func TestACPRoundtrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "acp.jsonl")
	now := time.Unix(400, 0).UTC()
	if err := AppendACP(path, ACPEvent{Time: now, Kind: "claim", TaskID: "T1"}); err != nil {
		t.Fatal(err)
	}
	if err := AppendACP(path, ACPEvent{Time: now.Add(time.Second), Kind: "report", TaskID: "T1", Payload: "ok"}); err != nil {
		t.Fatal(err)
	}
	events, err := ReadACP(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Seq != 1 || events[1].Seq != 2 || events[1].Payload != "ok" {
		t.Fatalf("events = %#v", events)
	}
}

func TestLeaseReclaim(t *testing.T) {
	now := time.Unix(500, 0).UTC()
	leases := []Lease{
		{TaskID: "live", ExpiresAt: now.Add(time.Second)},
		{TaskID: "retry", ExpiresAt: now.Add(-time.Second), Retries: 1},
		{TaskID: "escalate", ExpiresAt: now.Add(-time.Second), Retries: 2},
	}
	reclaimed := ReclaimExpired(leases, now, 2)
	if len(reclaimed) != 2 || !reclaimed[0].Retry || reclaimed[1].Retry {
		t.Fatalf("reclaimed = %#v", reclaimed)
	}
	escalation := Escalation(leases, 2, now)
	if escalation.TaskID != "escalate" || escalation.Reason != "retry limit exceeded" {
		t.Fatalf("escalation = %#v", escalation)
	}
}

func TestSessionCAS(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "session.json")
	if err := SaveSessionCAS(root, path, 0, Session{Leases: []Lease{{TaskID: "T1"}}}); err != nil {
		t.Fatal(err)
	}
	session, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if session.Revision != 1 || session.Leases[0].TaskID != "T1" {
		t.Fatalf("session = %#v", session)
	}
	if err := SaveSessionCAS(root, path, 0, Session{}); !errors.Is(err, ErrSessionRevisionConflict) {
		t.Fatalf("SaveSessionCAS stale err = %v, want conflict", err)
	}
}

func TestBrainDriverDispatchesFrontier(t *testing.T) {
	dispatcher := &recordingDispatcher{}
	snapshot := Snapshot{Frontier: []core.FrontierTask{{ID: "T2"}, {ID: "T1"}}}
	decision, err := DispatchFrontier(snapshot, DecisionLimitsForAuthority(Authority{Enabled: true}, DecisionLimits{}), dispatcher)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != ActionDispatch || dispatcher.task.ID != "T1" {
		t.Fatalf("decision=%#v dispatched=%#v", decision, dispatcher.task)
	}
}

func TestFailClosedAuthority(t *testing.T) {
	disabled := Authority{}
	if disabled.CanDispatch() || disabled.CanClearGate(GateLow) {
		t.Fatal("disabled authority granted permission")
	}
	enabled := Authority{Enabled: true}
	if !enabled.CanClearGate(GateMedium) {
		t.Fatal("enabled authority should clear medium gate")
	}
	if enabled.CanClearGate(GateHigh) || enabled.CanClearGate(GateCritical) {
		t.Fatal("authority cleared high or critical gate")
	}
	decision := Decide(Snapshot{Frontier: []core.FrontierTask{{ID: "T1"}}}, DecisionLimitsForAuthority(disabled, DecisionLimits{}))
	if decision.Action != ActionWait {
		t.Fatalf("disabled authority decision = %#v, want wait", decision)
	}
}

func TestNoLLM(t *testing.T) {
	files := []string{"decide.go", "sense.go", "brakes.go", "authority.go", "driver.go"}
	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, forbidden := range []string{"net/http", "openai", "anthropic", "llm", "model"} {
			if contains(string(data), forbidden) {
				t.Fatalf("%s imports or references forbidden decision-path token %q", file, forbidden)
			}
		}
	}
}

type recordingDispatcher struct {
	task core.FrontierTask
}

func (dispatcher *recordingDispatcher) Dispatch(task core.FrontierTask) error {
	dispatcher.task = task
	return nil
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
