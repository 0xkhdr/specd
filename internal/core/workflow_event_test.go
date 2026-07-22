package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowEventReplay(t *testing.T) {
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	eventPath := filepath.Join(dir, "workflow-events.jsonl")
	baseline := InitialState("demo")
	if err := SaveStateCAS(statePath, 0, baseline); err != nil {
		t.Fatal(err)
	}
	baseline.Revision = 1
	next := baseline
	next.Status = StatusDesign
	next.Phase = PhaseForStatus(StatusDesign)
	event, err := NewWorkflowEvent(WorkflowEventV1{
		EntityKind: "spec", EntityID: "demo", BeforeEntityVersion: 1, AfterEntityVersion: 2,
		ExpectedRevision: 1, Transition: "approve_requirements", Actor: "human:alice",
		AuthorityDigest: strings.Repeat("a", 64), Reason: "requirements accepted",
		InputDigests:     map[string]string{"requirements.md": strings.Repeat("b", 64)},
		ImpactedEntities: []string{"spec:demo"}, Timestamp: "2026-07-22T00:00:00Z", Projection: next,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := AppendWorkflowEvent(eventPath, event); err != nil {
		t.Fatal(err)
	}
	replayed, err := ReplayWorkflowEvents(baseline, []WorkflowEventV1{event})
	if err != nil || stateJSON(replayed) != stateJSON(event.Projection) {
		t.Fatalf("replay = %+v, err = %v", replayed, err)
	}
	recovered, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil || stateJSON(recovered) != stateJSON(event.Projection) {
		t.Fatalf("recovery = %+v, err = %v", recovered, err)
	}
	again, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil || stateJSON(again) != stateJSON(recovered) {
		t.Fatalf("idempotent recovery = %+v, err = %v", again, err)
	}
}

func stateJSON(state State) string {
	raw, _ := json.Marshal(state)
	return string(raw)
}

func TestWorkflowEventReplayRejectsCorruption(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow-events.jsonl")
	if err := os.WriteFile(path, []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	if events, err := ReadWorkflowEvents(path); err != nil || len(events) != 0 {
		t.Fatalf("torn tail = %+v, err = %v", events, err)
	}
	if err := os.WriteFile(path, []byte("{bad}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadWorkflowEvents(path); err == nil {
		t.Fatal("complete corrupt line accepted")
	}

	baseline := InitialState("demo")
	future := WorkflowEventV1{SchemaVersion: 2}
	if _, err := ReplayWorkflowEvents(baseline, []WorkflowEventV1{future}); err == nil {
		t.Fatal("future event schema accepted")
	}
}

func TestWorkflowEventReplayRepairsTornAppend(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "workflow-events.jsonl")
	baseline := InitialState("demo")
	next := baseline
	event, err := NewWorkflowEvent(WorkflowEventV1{
		EntityKind: "spec", EntityID: "demo", BeforeEntityVersion: 0, AfterEntityVersion: 1,
		ExpectedRevision: 0, Transition: "baseline", Actor: "migration",
		AuthorityDigest: strings.Repeat("a", 64), Reason: "v1 baseline",
		Timestamp: "2026-07-22T00:00:00Z", Projection: next,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{torn"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := AppendWorkflowEvent(path, event); err != nil {
		t.Fatal(err)
	}
	events, err := ReadWorkflowEvents(path)
	if err != nil || len(events) != 1 || events[0].ID != event.ID {
		t.Fatalf("events = %+v, err = %v", events, err)
	}
}
