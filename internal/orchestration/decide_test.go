package orchestration

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
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

// TestDecideWaitReasons pins spec R3.1/R3.2: the two wait conditions carry
// distinct reasons, each naming its unblock command exactly.
func TestDecideWaitReasons(t *testing.T) {
	frontier := []core.FrontierTask{{ID: "T1", Role: "craftsman"}}
	cases := []struct {
		name     string
		snapshot Snapshot
		limits   DecisionLimits
		want     string
	}{
		{
			name:     "authority-absent",
			snapshot: Snapshot{Frontier: frontier},
			limits:   DecisionLimits{AllowDispatch: false},
			want:     "waiting: dispatch authority absent; grant it with `specd brain run <slug> --authority`",
		},
		{
			name:     "authority-absent-and-frontier-empty",
			snapshot: Snapshot{},
			limits:   DecisionLimits{AllowDispatch: false},
			want:     "waiting: dispatch authority absent; grant it with `specd brain run <slug> --authority`",
		},
		{
			name:     "frontier-empty",
			snapshot: Snapshot{},
			limits:   DecisionLimits{AllowDispatch: true},
			want:     "waiting: frontier empty (no task has all dependencies resolved); inspect with `specd status <slug> --guide`",
		},
		{
			name:     "worker-absent",
			snapshot: Snapshot{Frontier: frontier},
			limits:   DecisionLimits{AllowDispatch: true, Workers: workerPresence(false)},
			want:     "waiting: no worker definition for active harness; repair with `specd init --repair`",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			decision := Decide(tc.snapshot, tc.limits)
			if decision.Action != ActionWait {
				t.Fatalf("Decide() = %#v, want wait", decision)
			}
			if decision.Reason != tc.want {
				t.Fatalf("reason = %q, want %q", decision.Reason, tc.want)
			}
		})
	}
}

type workerPresence bool

func (present workerPresence) WorkerAvailable() bool { return bool(present) }

func TestSense(t *testing.T) {
	now := time.Unix(200, 0).UTC()
	state := core.State{Revision: 7, Phase: "tasks", Records: map[string]json.RawMessage{"x": []byte(`{"ok":true}`)}}
	frontier := []core.FrontierTask{{ID: "T1", Verify: "go test"}}
	leases := []Lease{{TaskID: "T0", WorkerID: "w", ExpiresAt: now.Add(time.Minute)}}

	snapshot := Sense(state, frontier, leases, Telemetry{}, now)
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
	cost := EvaluateBrakes(Snapshot{Now: now, TelemetryKnown: true, TelemetryTrusted: true, CostMicros: 11}, DecisionLimits{MaxCostMicros: 10})
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

// TestControllerApprovalHandoff pins R4.1 and R4.4 in the decision package: an
// exhausted frontier at a lifecycle gate is a distinct decision that names the
// gate and the route, ordinary frontier waits are untouched, and no code here
// can create or spend approval authority.
func TestControllerApprovalHandoff(t *testing.T) {
	now := time.Unix(100, 0).UTC()
	dispatchable := DecisionLimits{AllowDispatch: true}

	t.Run("emptyfrontierwithoutagatestillwaits", func(t *testing.T) {
		decision := Decide(Snapshot{Now: now}, dispatchable)
		if decision.Action != ActionWait || decision.Reason != ReasonWaitFrontierEmpty {
			t.Fatalf("decision = %#v, want the ordinary frontier wait", decision)
		}
	})

	t.Run("humanroute", func(t *testing.T) {
		limits := dispatchable
		limits.Approval = &ApprovalHandoff{Gate: "tasks", Route: "specd approve demo", Actor: "human"}
		decision := Decide(Snapshot{Now: now}, limits)
		if decision.Action != ActionWaitApproval {
			t.Fatalf("action = %q, want waiting_approval", decision.Action)
		}
		if decision.Handoff == nil || decision.Handoff.Gate != "tasks" {
			t.Fatalf("decision carries no handoff: %#v", decision)
		}
		for _, want := range []string{"tasks", "human", "specd approve demo"} {
			if !strings.Contains(decision.Reason, want) {
				t.Errorf("reason %q omits %q", decision.Reason, want)
			}
		}
	})

	t.Run("delegatedroute", func(t *testing.T) {
		limits := dispatchable
		limits.Approval = &ApprovalHandoff{Gate: "tasks", Route: "specd delegate approve demo --grant nightly", Actor: "operator"}
		decision := Decide(Snapshot{Now: now}, limits)
		if decision.Action != ActionWaitApproval || !strings.Contains(decision.Reason, "delegate approve") {
			t.Fatalf("decision = %#v, want the delegated route named", decision)
		}
	})

	t.Run("blockedgatesnameneitherapproval", func(t *testing.T) {
		limits := dispatchable
		limits.Approval = &ApprovalHandoff{Gate: "tasks", Route: "specd check demo", Actor: "agent", Blocked: true}
		decision := Decide(Snapshot{Now: now}, limits)
		if !strings.Contains(decision.Reason, "readiness gates refuse") || !strings.Contains(decision.Reason, "specd check demo") {
			t.Fatalf("blocked reason = %q", decision.Reason)
		}
		if strings.Contains(decision.Reason, "requires") {
			t.Fatalf("a blocked gate was still presented as approvable: %q", decision.Reason)
		}
	})

	// A brake still outranks the handoff: an approval gate is not a way around
	// a cost, token, or deadline stop.
	t.Run("brakesoutrankthehandoff", func(t *testing.T) {
		limits := dispatchable
		limits.Approval = &ApprovalHandoff{Gate: "tasks", Route: "specd approve demo", Actor: "human"}
		limits.Deadline = now
		if decision := Decide(Snapshot{Now: now}, limits); decision.Action != ActionTimeout {
			t.Fatalf("action = %q, want the deadline brake", decision.Action)
		}
	})

	// A frontier with work is unaffected: the handoff only ever fires when
	// there is nothing left to dispatch.
	t.Run("readyfrontierstilldispatches", func(t *testing.T) {
		limits := dispatchable
		limits.Approval = &ApprovalHandoff{Gate: "tasks", Route: "specd approve demo", Actor: "human"}
		decision := Decide(Snapshot{Now: now, Frontier: []core.FrontierTask{{ID: "T1"}}}, limits)
		if decision.Action != ActionDispatch || decision.TaskID != "T1" {
			t.Fatalf("decision = %#v, want dispatch", decision)
		}
	})

	// R4.4: the controller cannot self-grant because it has no code that could.
	// This walks the package's own source, so wiring issuance or consumption in
	// later fails here rather than in review.
	t.Run("packageneverspendsapprovalauthority", func(t *testing.T) {
		forbidden := map[string]bool{
			"IssueDelegationGrant": true, "RevokeDelegationGrant": true, "ReserveGrantUse": true,
			"ConsumeGrantUse": true, "ReleaseGrantUse": true, "NewDelegationToken": true,
			"DelegationTokenDigest": true, "AuthorizeDelegatedTransition": true, "DelegationGrantV1": true,
		}
		entries, err := os.ReadDir(".")
		if err != nil {
			t.Fatal(err)
		}
		scanned := 0
		for _, entry := range entries {
			name := entry.Name()
			if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			scanned++
			ast.Inspect(file, func(node ast.Node) bool {
				if ident, ok := node.(*ast.Ident); ok && forbidden[ident.Name] {
					t.Errorf("%s reaches %s: the controller must never create, widen, or spend approval authority", name, ident.Name)
				}
				return true
			})
		}
		if scanned == 0 {
			t.Fatal("no source scanned — the assertion proves nothing")
		}
	})
}
