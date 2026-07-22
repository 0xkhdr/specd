package core

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// undoLedger seeds a three-event ledger over a fresh spec and returns the state
// path, event path, and the durable events.
func undoLedger(t *testing.T) (string, string, []WorkflowEventV1) {
	t.Helper()
	dir := t.TempDir()
	statePath := filepath.Join(dir, "state.json")
	eventPath := filepath.Join(dir, "workflow-events.jsonl")
	baseline := InitialState("demo")
	if err := SaveState(statePath, baseline); err != nil {
		t.Fatal(err)
	}
	appendUndoEvent(t, eventPath, baseline, "state.migrate.v1", StatusRequirements)
	events, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	appendUndoEvent(t, eventPath, events[0].Projection, "approve_requirements", StatusDesign)
	if events, err = ReadWorkflowEvents(eventPath); err != nil {
		t.Fatal(err)
	}
	appendUndoEvent(t, eventPath, events[1].Projection, "approve_design", StatusTasks)
	if events, err = ReadWorkflowEvents(eventPath); err != nil {
		t.Fatal(err)
	}
	if _, err := RecoverWorkflowState(statePath, eventPath); err != nil {
		t.Fatal(err)
	}
	return statePath, eventPath, events
}

func appendUndoEvent(t *testing.T, eventPath string, prior State, transition string, status Status) {
	t.Helper()
	next := prior
	next.Status = status
	next.Stage = Stage(status)
	next.Phase = PhaseForStatus(status)
	event, err := NewWorkflowEvent(WorkflowEventV1{
		EntityKind: "spec", EntityID: "demo",
		BeforeEntityVersion: prior.Revision, AfterEntityVersion: prior.Revision + 1,
		ExpectedRevision: prior.Revision, Transition: transition, Actor: "operator:alice",
		AuthorityDigest: strings.Repeat("a", 64), Reason: "seed " + transition,
		ImpactedEntities: []string{"spec:demo"}, Timestamp: "2026-07-22T00:00:00Z", Projection: next,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := AppendWorkflowEvent(eventPath, event); err != nil {
		t.Fatal(err)
	}
}

func undoRequest(events []WorkflowEventV1, revision int64, consumptions ...ImpactConsumption) UndoRequest {
	return UndoRequest{
		TargetEventID: events[len(events)-1].ID, ExpectedRevision: revision,
		Reason: "approved the wrong stage", Consumptions: consumptions,
	}
}

func TestUndoCompensationAppendsPriorProjection(t *testing.T) {
	statePath, eventPath, events := undoLedger(t)
	req := undoRequest(events, 3)
	preview := PlanUndo(req, events, 3)
	if !preview.Eligible || len(preview.Blockers) != 0 {
		t.Fatalf("preview = %+v, want eligible", preview)
	}
	plan, err := CommitUndo(statePath, eventPath, req, preview)
	if err != nil {
		t.Fatalf("commit undo: %v", err)
	}
	if plan.NewRevision != 4 || plan.PriorRevision != 2 || plan.CompensationID == "" {
		t.Fatalf("plan = %+v, want prior 2 restored at revision 4", plan)
	}

	after, err := ReadWorkflowEvents(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) != 4 || after[2].ID != events[2].ID {
		t.Fatalf("ledger = %d events, want the original preserved and one appended", len(after))
	}
	compensation := after[3]
	if compensation.Transition != UndoCompensationPrefix+events[2].Transition {
		t.Fatalf("transition = %q", compensation.Transition)
	}
	if compensation.InputDigests["undone_event"] != events[2].ID || compensation.InputDigests["impact_plan"] != plan.ImpactDigest {
		t.Fatalf("input digests = %+v, want the undone event and impact plan pinned", compensation.InputDigests)
	}
	for _, guard := range []string{"child-event", "reversible-transition", "state-revision", "consumption.evidence", "consumption.external", "consumption.delegation", "consumption.other"} {
		if compensation.InputDigests["guard."+guard] == "" {
			t.Fatalf("guard %q not recorded on the compensation event", guard)
		}
	}
	if compensation.Reason != req.Reason || compensation.AuthorityDigest == "" || len(compensation.ImpactedEntities) == 0 {
		t.Fatalf("compensation = %+v, want reason, authority, and impacted identities", compensation)
	}

	state, err := LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if state.Revision != 4 || state.Status != events[1].Projection.Status {
		t.Fatalf("state = %+v, want the prior effective status at revision 4", state)
	}
}

func TestUndoCompensationRefusesConsumedTarget(t *testing.T) {
	_, _, events := undoLedger(t)
	cases := map[string]struct {
		consumption ImpactConsumption
		code        string
	}{
		"evidence-consumed":   {ImpactConsumption{Record: "task:T1", Kind: "evidence"}, "UNDO_CONSUMED"},
		"external-effect":     {ImpactConsumption{Record: "releases.jsonl", Kind: "release", External: true}, "UNDO_CONSUMED_EXTERNALLY"},
		"delegation-used":     {ImpactConsumption{Record: "lease-7", Kind: "lease"}, "UNDO_CONSUMED"},
		"unknown-consumption": {ImpactConsumption{Record: "mystery", Kind: "widget"}, "UNDO_CONSUMED"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			plan := PlanUndo(undoRequest(events, 3, tc.consumption), events, 3)
			if plan.Eligible || plan.Blockers[0].Code != tc.code {
				t.Fatalf("plan = %+v, want %s", plan, tc.code)
			}
			refusal, ok := AsRefusal(plan.Refusal("demo"))
			if !ok || refusal.ActorRequired != RefusalActorOperator || !strings.Contains(refusal.Blocker, tc.consumption.Record) {
				t.Fatalf("refusal = %+v, want an operator refusal naming the consuming record", refusal)
			}
		})
	}
}

func TestUndoCompensationRefusesNonLatestAndIrreversible(t *testing.T) {
	statePath, eventPath, events := undoLedger(t)
	t.Run("child-consumed", func(t *testing.T) {
		req := UndoRequest{TargetEventID: events[1].ID, ExpectedRevision: 3, Reason: "too late"}
		plan := PlanUndo(req, events, 3)
		if plan.Eligible || plan.Blockers[0].Code != "UNDO_NOT_LATEST" || !strings.Contains(plan.Blockers[0].Message, events[2].ID) {
			t.Fatalf("plan = %+v, want UNDO_NOT_LATEST naming the child event", plan)
		}
		if _, err := CommitUndo(statePath, eventPath, req, plan); err == nil {
			t.Fatal("commit accepted a non-latest target")
		}
		after, err := ReadWorkflowEvents(eventPath)
		if err != nil || len(after) != len(events) {
			t.Fatalf("ledger mutated by a refused undo: %d events, err %v", len(after), err)
		}
	})
	t.Run("baseline-irreversible", func(t *testing.T) {
		single := events[:1]
		plan := PlanUndo(undoRequest(single, 1), single, 1)
		codes := []string{}
		for _, blocker := range plan.Blockers {
			codes = append(codes, blocker.Code)
		}
		if plan.Eligible || !strings.Contains(strings.Join(codes, ","), "UNDO_IRREVERSIBLE") || !strings.Contains(strings.Join(codes, ","), "UNDO_NO_PRIOR_STATE") {
			t.Fatalf("blockers = %v, want irreversible baseline refusal", codes)
		}
	})
	t.Run("reason-required", func(t *testing.T) {
		plan := PlanUndo(UndoRequest{TargetEventID: events[2].ID, ExpectedRevision: 3}, events, 3)
		if plan.Eligible || plan.Blockers[0].Code != "UNDO_REASON_REQUIRED" {
			t.Fatalf("plan = %+v, want UNDO_REASON_REQUIRED", plan)
		}
	})
}

// TestUndoCompensationSurvivesAppendCrash pins R2.3: a crash between the event
// append and the projection CAS leaves history and projection mutually
// recoverable, never a partial restore.
func TestUndoCompensationSurvivesAppendCrash(t *testing.T) {
	statePath, eventPath, events := undoLedger(t)
	req := undoRequest(events, 3)
	plan := PlanUndo(req, events, 3)
	event, err := BuildUndoCompensation(plan, req, events)
	if err != nil {
		t.Fatal(err)
	}
	if err := AppendWorkflowEvent(eventPath, event); err != nil {
		t.Fatal(err)
	}
	crashed, err := LoadState(statePath)
	if err != nil || crashed.Revision != 3 {
		t.Fatalf("state before recovery = %+v, err %v, want the pre-undo projection intact", crashed, err)
	}
	recovered, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if recovered.Revision != 4 || recovered.Status != events[1].Projection.Status {
		t.Fatalf("recovered = %+v, want the compensation completed at revision 4", recovered)
	}
	again, err := RecoverWorkflowState(statePath, eventPath)
	if err != nil || again.Revision != 4 {
		t.Fatalf("recovery is not idempotent: %+v, err %v", again, err)
	}
}

// TestUndoCompensationCASRace pins R6.2: two undos previewed against the same
// revision produce exactly one compensation; the loser gets a legal re-preview
// route instead of a second mutation.
func TestUndoCompensationCASRace(t *testing.T) {
	statePath, eventPath, events := undoLedger(t)
	req := undoRequest(events, 3)
	preview := PlanUndo(req, events, 3)
	if _, err := CommitUndo(statePath, eventPath, req, preview); err != nil {
		t.Fatalf("first commit: %v", err)
	}
	loser, err := CommitUndo(statePath, eventPath, req, preview)
	if !errors.Is(err, ErrImpactStale) {
		t.Fatalf("second commit = %v, want a stale-preview refusal", err)
	}
	if !strings.Contains(err.Error(), "--expect-revision 4") {
		t.Fatalf("loser error %q does not name the fresh preview revision", err)
	}
	if loser.CompensationID != "" {
		t.Fatalf("loser committed %q", loser.CompensationID)
	}
	after, err := ReadWorkflowEvents(eventPath)
	if err != nil || len(after) != 4 {
		t.Fatalf("ledger = %d events, err %v, want exactly one compensation", len(after), err)
	}
}

func TestUndoCompensationRefusesMissingLedger(t *testing.T) {
	dir := t.TempDir()
	eventPath := filepath.Join(dir, "workflow-events.jsonl")
	if _, err := os.Stat(eventPath); !os.IsNotExist(err) {
		t.Fatalf("fixture ledger already exists: %v", err)
	}
	plan := PlanUndo(UndoRequest{ExpectedRevision: 0, Reason: "nothing to undo"}, nil, 0)
	if plan.Eligible || plan.Blockers[0].Code != "UNDO_LEDGER_EMPTY" {
		t.Fatalf("plan = %+v, want UNDO_LEDGER_EMPTY", plan)
	}
}
