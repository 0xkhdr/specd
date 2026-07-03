package cmd

import (
	"fmt"
	"strconv"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// ensureTelemetry lazily allocates a task's Telemetry record so callers can set
// a field without a nil check, keeping it omitted for tasks that never touch it.
func ensureTelemetry(ts *core.TaskState) *core.Telemetry {
	if ts.Telemetry == nil {
		ts.Telemetry = &core.Telemetry{}
	}
	return ts.Telemetry
}

// applyTelemetryAnnotations stores the operator-supplied --tokens/--cost values
// verbatim. specd records these as evidence; it never computes or prices them.
// Absent flags leave telemetry untouched (no empty record is created).
func applyTelemetryAnnotations(args cli.Args, ts *core.TaskState) {
	if args.Has("tokens") {
		if n, err := strconv.Atoi(args.Str("tokens")); err == nil {
			ensureTelemetry(ts).Tokens = n
		}
	}
	if args.Has("cost") {
		ensureTelemetry(ts).Cost = args.Str("cost")
	}
}

var validTaskStatuses = map[core.TaskStatus]bool{
	core.TaskComplete: true,
	core.TaskBlocked:  true,
	core.TaskRunning:  true,
	core.TaskPending:  true,
}

// RunTask implements `specd task`: it validates the requested --status
// transition for a task, then either delegates "complete" to
// core.CompleteTask's integrity-checked path or, under the spec lock,
// mutates the task's status/blocker/annotation directly, persisting both
// state.json and tasks.md.
//
//nolint:gocyclo // pre-existing complexity debt, out of scope for spec S3 — tracked for a future cleanup pass
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

	// Completion goes through the single core integrity path (the same one the
	// Pinky evidence reconciler uses), so dependency/verification/gate rules and
	// the state.json + tasks.md mutation live in exactly one place.
	if newStatus == core.TaskComplete {
		return runTaskComplete(root, slug, id, args)
	}

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

		switch newStatus {
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
				stamp := core.NowISO()
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

		// Operator-annotated cost/usage (stored verbatim, never computed).
		applyTelemetryAnnotations(args, &ts)

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
		cfg := core.LoadConfig(root)
		if routing, _, err := core.ResolveRoutingStamps(cfg.Routing, state.Tasks); err != nil {
			return specdExit(err), err
		} else if len(routing) > 0 {
			state.Routing = routing
		}
		core.DeriveSpecStatus(state)
		if err := core.SaveState(root, slug, state); err != nil {
			return specdExit(err), err
		}

		fmt.Printf("task %s → %s\n", id, newStatus)
		if newStatus == core.TaskBlocked {
			fmt.Printf("  blocked: %s\n", reason)
		}
		return core.ExitOK, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}

// runTaskComplete drives the complete transition through core.CompleteTask and
// renders the same output the inline path used to. Telemetry flags are forwarded
// as verbatim annotations.
func runTaskComplete(root, slug, id string, args cli.Args) int {
	req := core.CompleteTaskRequest{
		Evidence:   args.Str("evidence"),
		Unverified: args.Bool("unverified"),
		Force:      args.Bool("force"),
	}
	if args.Has("tokens") {
		if n, err := strconv.Atoi(args.Str("tokens")); err == nil {
			req.Tokens = &n
		}
	}
	if args.Has("cost") {
		c := args.Str("cost")
		req.Cost = &c
	}

	res, err := core.CompleteTask(root, slug, id, req)
	if err != nil {
		return specdExit(err)
	}
	fmt.Printf("task %s → %s\n", id, core.TaskComplete)
	fmt.Printf("  evidence: %s\n", res.Evidence)
	return core.ExitOK
}
