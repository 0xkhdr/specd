package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func deriveStatus(state *core.State) {
	vals := make([]core.TaskState, 0, len(state.Tasks))
	for _, t := range state.Tasks {
		vals = append(vals, t)
	}
	if len(vals) == 0 {
		return
	}
	started := false
	for _, t := range vals {
		if t.Status != core.TaskPending {
			started = true
			break
		}
	}
	if !started {
		return
	}
	allComplete := true
	for _, t := range vals {
		if t.Status != core.TaskComplete {
			allComplete = false
			break
		}
	}
	if allComplete {
		if state.Status != core.StatusComplete {
			state.Status = core.StatusVerifying
		}
	} else {
		next := core.NextRunnable(core.DagTasksFromState(state))
		if next.Kind == core.NextAllBlocked {
			state.Status = core.StatusBlocked
		} else {
			state.Status = core.StatusExecuting
		}
	}
	state.Phase = core.PhaseForStatus(state.Status)
}

// validateComplete enforces the complete-status evidence gate and returns the
// evidence string to record. It mutates nothing; on any gate failure it returns
// a *core.SpecdError (GateError) with the same message the inline gate emitted.
func validateComplete(state *core.State, ts core.TaskState, docTask *core.ParsedTask, slug, id string, args cli.Args) (string, *core.SpecdError) {
	for _, d := range ts.Depends {
		if dep, ok := state.Tasks[d]; !ok || dep.Status != core.TaskComplete {
			return "", core.GateError(fmt.Sprintf("task %s: cannot complete — dependencies not complete: %s", id, d))
		}
	}
	evidence := args.Str("evidence")
	if args.Bool("unverified") {
		if evidence == "" {
			return "", core.GateError(fmt.Sprintf("task %s: --status complete --unverified requires non-empty --evidence", id))
		}
	} else {
		verifyLine := ""
		if v, ok := docTask.Meta["verify"]; ok {
			verifyLine = v
		}
		rec := ts.Verification
		if rec == nil || !rec.Verified {
			return "", core.GateError(fmt.Sprintf("task %s: --status complete requires a passing `specd verify %s %s` (exit 0) first — or pass --unverified with --evidence for a manual proof", id, slug, id))
		}
		if rec.Command != verifyLine {
			return "", core.GateError(fmt.Sprintf("task %s: verification is stale — recorded command (%s) ≠ current verify line (%s); re-run `specd verify %s %s`", id, rec.Command, verifyLine, slug, id))
		}
	}
	derived := ""
	if ts.Verification != nil && ts.Verification.Verified {
		gitHead := "no-git"
		if ts.Verification.GitHead != nil {
			gitHead = *ts.Verification.GitHead
		}
		derived = fmt.Sprintf("verified: `%s` → exit 0 @ %s (%s)", ts.Verification.Command, gitHead, ts.Verification.RanAt)
	}
	if evidence == "" {
		evidence = derived
	}
	return evidence, nil
}

var validTaskStatuses = map[core.TaskStatus]bool{
	core.TaskComplete: true,
	core.TaskBlocked:  true,
	core.TaskRunning:  true,
	core.TaskPending:  true,
}

func RunTask(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	id := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if len(args.Pos) > 1 {
		id = args.Pos[1]
	}
	if slug == "" || id == "" {
		return usageExit("usage: specd task <slug> <id> --status <complete|blocked|running|pending> [flags]")
	}
	statusStr := args.Str("status")
	if !validTaskStatuses[core.TaskStatus(statusStr)] {
		return usageExit("--status must be one of: complete, blocked, running, pending")
	}
	newStatus := core.TaskStatus(statusStr)

	rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		loaded, err := core.LoadSpec(root, slug)
		if err != nil {
			return specdExit(err), err
		}
		state := loaded.State
		doc := loaded.Doc

		if state.Gate == core.GateAwaitingApproval && !args.Bool("force") {
			return specdExit(core.GateError(fmt.Sprintf("spec '%s' is gated (awaiting-approval) — run `specd approve %s` after the revised plan is approved, or pass --force", slug, slug))), nil
		}

		ts, hasTsState := state.Tasks[id]
		docTask := core.FindTask(doc, id)
		if !hasTsState || docTask == nil {
			return specdExit(core.NotFoundError(fmt.Sprintf("task '%s' not found in spec '%s'", id, slug))), nil
		}

		reason := args.Str("reason")
		stamp := core.NowISO()

		switch newStatus {
		case core.TaskComplete:
			ev, serr := validateComplete(state, ts, docTask, slug, id, args)
			if serr != nil {
				return specdExit(serr), nil
			}
			ts.Status = core.TaskComplete
			ts.Evidence = &ev
			ts.FinishedAt = &stamp
			if ts.StartedAt == nil {
				ts.StartedAt = &stamp
			}
			ts.Blocker = nil
			core.RemoveBlocker(state, id)
			docTask.Checked = true
			docTask.Annotation = &core.Annotation{Kind: core.AnnotComplete, Evidence: ev, Ts: stamp}

		case core.TaskBlocked:
			if reason == "" {
				return specdExit(core.GateError(fmt.Sprintf("task %s: --status blocked requires --reason", id))), nil
			}
			ts.Status = core.TaskBlocked
			ts.Blocker = &reason
			core.AddBlocker(state, id, reason, state.Turn)
			docTask.Checked = false
			docTask.Annotation = &core.Annotation{Kind: core.AnnotBlocked, Reason: reason}

		case core.TaskRunning:
			ts.Status = core.TaskRunning
			if ts.StartedAt == nil {
				ts.StartedAt = &stamp
			}
			ts.Blocker = nil
			core.RemoveBlocker(state, id)
			docTask.Checked = false
			docTask.Annotation = nil

		case core.TaskPending:
			ts.Status = core.TaskPending
			ts.Blocker = nil
			core.RemoveBlocker(state, id)
			docTask.Checked = false
			docTask.Annotation = nil
		}

		state.Tasks[id] = ts

		tasksPath := core.ArtifactPath(root, slug, "tasks.md")
		raw := core.ReadOrDefault(tasksPath, "")
		updated, err := core.ApplyTaskAnnotation(raw, id, docTask.Checked, docTask.Annotation)
		if err != nil {
			return specdExit(err), err
		}
		if err := core.AtomicWrite(tasksPath, updated); err != nil {
			return specdExit(err), err
		}
		deriveStatus(state)
		if err := core.SaveState(root, slug, state); err != nil {
			return specdExit(err), err
		}

		fmt.Printf("task %s → %s\n", id, newStatus)
		if newStatus == core.TaskComplete && ts.Evidence != nil {
			fmt.Printf("  evidence: %s\n", *ts.Evidence)
		}
		if newStatus == core.TaskBlocked {
			fmt.Printf("  blocked: %s\n", reason)
		}
		return 0, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}
