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

func TestReopenTransactionRecoveryAtCrashBoundaries(t *testing.T) {
	setup := func(t *testing.T) (string, string, string, State, WorkflowEventV1, transitionJournal) {
		t.Helper()
		dir := t.TempDir()
		statePath := filepath.Join(dir, "state.json")
		eventPath := filepath.Join(dir, "workflow-events.jsonl")
		artifactPath := filepath.Join(dir, "tasks.md")
		if err := AtomicWrite(artifactPath, "before\n"); err != nil {
			t.Fatal(err)
		}
		if err := SaveState(statePath, InitialState("demo")); err != nil {
			t.Fatal(err)
		}
		state, err := LoadState(statePath)
		if err != nil {
			t.Fatal(err)
		}
		next := state
		next.TaskStatus = map[string]TaskRunStatus{"T1": TaskPending}
		event, err := NewWorkflowEvent(WorkflowEventV1{
			EntityKind: "task", EntityID: "T1", BeforeEntityVersion: 1, AfterEntityVersion: 2,
			ExpectedRevision: state.Revision, Transition: "reopen.task.T1", Actor: "operator:alice",
			AuthorityDigest: strings.Repeat("a", 64), Reason: "repair",
			Timestamp: "2026-07-22T00:00:00Z", Projection: next,
		})
		if err != nil {
			t.Fatal(err)
		}
		journal := transitionJournal{EventID: event.ID, Artifact: TransitionArtifact{
			Path: artifactPath, Before: "before\n", After: "after\n",
		}}
		return statePath, eventPath, artifactPath, state, event, journal
	}
	writeJournal := func(t *testing.T, eventPath string, journal transitionJournal) {
		t.Helper()
		raw, err := json.Marshal(journal)
		if err != nil {
			t.Fatal(err)
		}
		if err := AtomicWrite(transitionJournalPath(eventPath), string(raw)+"\n"); err != nil {
			t.Fatal(err)
		}
		if err := AtomicWrite(journal.Artifact.Path, journal.Artifact.After); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("artifact-written-before-event-rolls-back", func(t *testing.T) {
		statePath, eventPath, artifactPath, beforeState, _, journal := setup(t)
		writeJournal(t, eventPath, journal)
		recovered, err := RecoverWorkflowState(statePath, eventPath)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(artifactPath)
		if string(raw) != "before\n" || stateJSON(recovered) != stateJSON(beforeState) {
			t.Fatalf("artifact/state = %q/%+v, want rollback", raw, recovered)
		}
		if _, err := os.Stat(transitionJournalPath(eventPath)); !os.IsNotExist(err) {
			t.Fatalf("journal remains after rollback: %v", err)
		}
	})

	t.Run("event-written-before-state-rolls-forward", func(t *testing.T) {
		statePath, eventPath, artifactPath, _, event, journal := setup(t)
		writeJournal(t, eventPath, journal)
		if err := AppendWorkflowEvent(eventPath, event); err != nil {
			t.Fatal(err)
		}
		recovered, err := RecoverWorkflowState(statePath, eventPath)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(artifactPath)
		if string(raw) != "after\n" || recovered.LastEventID != event.ID {
			t.Fatalf("artifact/state = %q/%+v, want roll-forward", raw, recovered)
		}
		again, err := RecoverWorkflowState(statePath, eventPath)
		if err != nil || stateJSON(again) != stateJSON(recovered) {
			t.Fatalf("idempotent recovery = %+v, err %v", again, err)
		}
	})

	t.Run("ordinary-commit-clears-journal", func(t *testing.T) {
		statePath, eventPath, artifactPath, _, event, journal := setup(t)
		if err := CommitWorkflowTransition(TransitionCommit{
			StatePath: statePath, EventPath: eventPath, Event: event, Artifact: &journal.Artifact,
		}); err != nil {
			t.Fatal(err)
		}
		raw, _ := os.ReadFile(artifactPath)
		state, _ := LoadState(statePath)
		if string(raw) != "after\n" || state.LastEventID != event.ID {
			t.Fatalf("artifact/state = %q/%+v, want committed pair", raw, state)
		}
		if _, err := os.Stat(transitionJournalPath(eventPath)); !os.IsNotExist(err) {
			t.Fatalf("journal remains after commit: %v", err)
		}
	})
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
