package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// undoSeedLedger gives a fresh demo spec a two-event workflow ledger so the
// latest event is a reversible transition with a predecessor to restore.
func undoSeedLedger(t *testing.T, root string) []core.WorkflowEventV1 {
	t.Helper()
	statePath, eventPath := core.StatePath(root, "demo"), core.WorkflowEventPath(root, "demo")
	state, err := core.LoadState(statePath)
	if err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		transition string
		status     core.Status
	}{{"approve_requirements", core.StatusDesign}, {"approve_design", core.StatusTasks}} {
		next := state
		next.Status = step.status
		next.Stage = core.Stage(step.status)
		next.Phase = core.PhaseForStatus(step.status)
		event, err := core.NewWorkflowEvent(core.WorkflowEventV1{
			EntityKind: "spec", EntityID: "demo",
			BeforeEntityVersion: state.Revision, AfterEntityVersion: state.Revision + 1,
			ExpectedRevision: state.Revision, Transition: step.transition, Actor: "operator:alice",
			AuthorityDigest: strings.Repeat("a", 64), Reason: "seed " + step.transition,
			ImpactedEntities: []string{"spec:demo"}, Timestamp: "2026-07-22T00:00:00Z", Projection: next,
		})
		if err != nil {
			t.Fatal(err)
		}
		if err := core.AppendWorkflowEvent(eventPath, event); err != nil {
			t.Fatal(err)
		}
		state = event.Projection
	}
	events, err := core.ReadWorkflowEvents(eventPath)
	if err != nil {
		t.Fatal(err)
	}
	return events
}

func TestUndoCompensationCLICompensatesLatestEvent(t *testing.T) {
	root := newDemoSpec(t)
	events := undoSeedLedger(t, root)
	out, err := captureStdout(t, func() error {
		return Run(root, "undo", []string{"demo"}, map[string]string{"reason": "approved the wrong stage", "expect-revision": "2"})
	})
	if err != nil {
		t.Fatalf("undo: %v", err)
	}
	var plan core.UndoPlan
	if err := json.Unmarshal([]byte(out), &plan); err != nil {
		t.Fatalf("undo json: %v (out=%q)", err, out)
	}
	if !plan.Eligible || plan.TargetEventID != events[1].ID || plan.NewRevision != 3 || plan.CompensationID == "" {
		t.Fatalf("plan = %+v, want the latest event compensated at revision 3", plan)
	}
	after, err := core.ReadWorkflowEvents(core.WorkflowEventPath(root, "demo"))
	if err != nil || len(after) != 3 || after[1].ID != events[1].ID {
		t.Fatalf("ledger = %d events, err %v, want history preserved plus one compensation", len(after), err)
	}
	state, err := core.LoadState(core.StatePath(root, "demo"))
	if err != nil || state.Revision != 3 || state.Status != events[0].Projection.Status {
		t.Fatalf("state = %+v, err %v, want the prior effective state at a higher revision", state, err)
	}
}

func TestUndoCompensationCLIRefusesStaleAndConsumed(t *testing.T) {
	t.Run("stale-revision", func(t *testing.T) {
		root := newDemoSpec(t)
		undoSeedLedger(t, root)
		err := Run(root, "undo", []string{"demo"}, map[string]string{"reason": "late", "expect-revision": "1"})
		refusal, ok := core.AsRefusal(err)
		if !ok || refusal.Code != "UNDO_REVISION_STALE" || !strings.Contains(refusal.Blocker, "--expect-revision 2") {
			t.Fatalf("undo = %v, want a stale-revision refusal naming the fresh revision", err)
		}
	})
	t.Run("external-effect", func(t *testing.T) {
		root := newDemoSpec(t)
		undoSeedLedger(t, root)
		if err := os.WriteFile(core.ReleaseLedgerPath(root, "demo"), []byte("{}\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		err := Run(root, "undo", []string{"demo"}, map[string]string{"reason": "late", "expect-revision": "2"})
		refusal, ok := core.AsRefusal(err)
		if !ok || refusal.Code != "UNDO_CONSUMED_EXTERNALLY" || !strings.Contains(refusal.Blocker, "successor") {
			t.Fatalf("undo = %v, want an external-consumption refusal with a successor route", err)
		}
		after, err := core.ReadWorkflowEvents(core.WorkflowEventPath(root, "demo"))
		if err != nil || len(after) != 2 {
			t.Fatalf("refused undo mutated the ledger: %d events, err %v", len(after), err)
		}
	})
	t.Run("empty-ledger", func(t *testing.T) {
		root := newDemoSpec(t)
		err := Run(root, "undo", []string{"demo"}, map[string]string{"reason": "nothing", "expect-revision": "0"})
		refusal, ok := core.AsRefusal(err)
		if !ok || refusal.Code != "UNDO_LEDGER_EMPTY" {
			t.Fatalf("undo = %v, want UNDO_LEDGER_EMPTY", err)
		}
	})
}

func TestUndoCompensationCLIRequiresReasonAndRevision(t *testing.T) {
	root := newDemoSpec(t)
	for name, flags := range map[string]map[string]string{
		"missing-reason":   {"expect-revision": "2"},
		"missing-revision": {"reason": "why"},
		"invalid-revision": {"reason": "why", "expect-revision": "latest"},
	} {
		t.Run(name, func(t *testing.T) {
			if err := Run(root, "undo", []string{"demo"}, flags); err == nil || !strings.Contains(err.Error(), "usage") {
				t.Fatalf("undo %v = %v, want a usage rejection", flags, err)
			}
		})
	}
}
