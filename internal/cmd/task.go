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

		evidence := args.Str("evidence")
		reason := args.Str("reason")
		stamp := core.NowISO()

		switch newStatus {
		case core.TaskComplete:
			for _, d := range ts.Depends {
				if dep, ok := state.Tasks[d]; !ok || dep.Status != core.TaskComplete {
					return specdExit(core.GateError(fmt.Sprintf("task %s: cannot complete — dependencies not complete: %s", id, d))), nil
				}
			}
			unverified := args.Bool("unverified")
			if unverified {
				if evidence == "" {
					return specdExit(core.GateError(fmt.Sprintf("task %s: --status complete --unverified requires non-empty --evidence", id))), nil
				}
			} else {
				verifyLine := ""
				if v, ok := docTask.Meta["verify"]; ok {
					verifyLine = v
				}
				rec := ts.Verification
				if rec == nil || !rec.Verified {
					return specdExit(core.GateError(fmt.Sprintf("task %s: --status complete requires a passing `specd verify %s %s` (exit 0) first — or pass --unverified with --evidence for a manual proof", id, slug, id))), nil
				}
				if rec.Command != verifyLine {
					return specdExit(core.GateError(fmt.Sprintf("task %s: verification is stale — recorded command (%s) ≠ current verify line (%s); re-run `specd verify %s %s`", id, rec.Command, verifyLine, slug, id))), nil
				}
			}
			ts.Status = core.TaskComplete
			derived := ""
			if ts.Verification != nil && ts.Verification.Verified {
				gitHead := ""
				if ts.Verification.GitHead != nil {
					gitHead = *ts.Verification.GitHead
				} else {
					gitHead = "no-git"
				}
				derived = fmt.Sprintf("verified: `%s` → exit 0 @ %s (%s)", ts.Verification.Command, gitHead, ts.Verification.RanAt)
			}
			ev := evidence
			if ev == "" {
				ev = derived
			}
			ts.Evidence = &ev
			ts.FinishedAt = &stamp
			if ts.StartedAt == nil {
				ts.StartedAt = &stamp
			}
			ts.Blocker = nil
			var blockers []core.Blocker
			for _, b := range state.Blockers {
				if b.Task != id {
					blockers = append(blockers, b)
				}
			}
			state.Blockers = blockers
			docTask.Checked = true
			docTask.Annotation = &core.Annotation{Kind: core.AnnotComplete, Evidence: ev, Ts: stamp}

		case core.TaskBlocked:
			if reason == "" {
				return specdExit(core.GateError(fmt.Sprintf("task %s: --status blocked requires --reason", id))), nil
			}
			ts.Status = core.TaskBlocked
			ts.Blocker = &reason
			var blockers []core.Blocker
			for _, b := range state.Blockers {
				if b.Task != id {
					blockers = append(blockers, b)
				}
			}
			state.Blockers = append(blockers, core.Blocker{Task: id, Reason: reason, Since: fmt.Sprintf("Turn %d", state.Turn)})
			docTask.Checked = false
			docTask.Annotation = &core.Annotation{Kind: core.AnnotBlocked, Reason: reason}

		case core.TaskRunning:
			ts.Status = core.TaskRunning
			if ts.StartedAt == nil {
				ts.StartedAt = &stamp
			}
			ts.Blocker = nil
			var blockers []core.Blocker
			for _, b := range state.Blockers {
				if b.Task != id {
					blockers = append(blockers, b)
				}
			}
			state.Blockers = blockers
			docTask.Checked = false
			docTask.Annotation = nil

		case core.TaskPending:
			ts.Status = core.TaskPending
			ts.Blocker = nil
			var blockers []core.Blocker
			for _, b := range state.Blockers {
				if b.Task != id {
					blockers = append(blockers, b)
				}
			}
			state.Blockers = blockers
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
