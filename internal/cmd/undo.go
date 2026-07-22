package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// runUndo compensates the latest workflow event of a spec. It never accepts an
// event id: the target is always the ledger tail, so an operator cannot aim
// undo at history that later work already depends on (spec 04 R2).
func runUndo(root string, args []string, flags map[string]string) error {
	if len(args) != 1 {
		return usageError("undo")
	}
	slug := args[0]
	if err := core.ValidateSlug(slug); err != nil {
		return core.Refusef("SPEC_INVALID", "%v", err)
	}
	reason := strings.TrimSpace(flags["reason"])
	if reason == "" {
		return fmt.Errorf("%w: undo requires --reason <text>", ErrUsage)
	}
	raw := strings.TrimSpace(flags["expect-revision"])
	if raw == "" {
		return fmt.Errorf("%w: undo requires --expect-revision <n>", ErrUsage)
	}
	expected, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || expected < 0 {
		return fmt.Errorf("%w: --expect-revision must be a non-negative integer, got %q", ErrUsage, raw)
	}

	plan, err := core.WithSpecLock(root, func() (core.UndoPlan, error) {
		statePath, eventPath := core.StatePath(root, slug), core.WorkflowEventPath(root, slug)
		state, err := core.RecoverWorkflowState(statePath, eventPath)
		if err != nil {
			return core.UndoPlan{}, err
		}
		events, err := core.ReadWorkflowEvents(eventPath)
		if err != nil {
			return core.UndoPlan{}, err
		}
		req := core.UndoRequest{ExpectedRevision: expected, Reason: reason}
		if len(events) > 0 {
			target := events[len(events)-1]
			req.TargetEventID = target.ID
			req.Consumptions = undoConsumptions(root, slug, target)
		}
		preview := core.PlanUndo(req, events, state.Revision)
		if !preview.Eligible {
			return preview, preview.Refusal(slug)
		}
		return core.CommitUndo(statePath, eventPath, req, preview)
	})
	if err != nil {
		return err
	}
	return writeJSON(plan)
}

// undoConsumptions resolves what has consumed the target event from durable
// records only. It is deliberately conservative: an immutable delivery ledger
// that exists at all blocks in-place undo, and evidence recorded at or after
// the target is treated as consuming it.
func undoConsumptions(root, slug string, target core.WorkflowEventV1) []core.ImpactConsumption {
	var consumptions []core.ImpactConsumption
	for _, external := range []struct{ kind, path string }{
		{"submission", core.SubmissionsPath(root, slug)},
		{"release", core.ReleaseLedgerPath(root, slug)},
		{"deployment", core.DeploymentLedgerPath(root, slug)},
		{"archive", core.ArchiveRecordPath(root, slug)},
	} {
		if info, err := os.Stat(external.path); err == nil && info.Size() > 0 {
			consumptions = append(consumptions, core.ImpactConsumption{Record: external.path, Kind: external.kind, External: true})
		}
	}
	evidence, err := core.LoadEvidence(core.EvidencePath(root, slug))
	if err != nil {
		// Unreadable evidence cannot prove the target unconsumed.
		return append(consumptions, core.ImpactConsumption{Record: core.EvidencePath(root, slug), Kind: "evidence"})
	}
	for task, record := range evidence {
		if record.ExitCode == 0 && record.Timestamp >= target.Timestamp {
			consumptions = append(consumptions, core.ImpactConsumption{Record: "task:" + task, Kind: "evidence"})
		}
	}
	return consumptions
}
