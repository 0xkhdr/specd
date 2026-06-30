package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func taskJSON(doc core.ParsedTasks, state *core.State, id string) map[string]interface{} {
	v := core.ResolveTaskView(doc, state, id)
	if v.FromDoc {
		out := map[string]interface{}{
			"id": v.ID, "title": v.Title, "wave": v.Wave,
		}
		for k, val := range v.Meta {
			out[k] = val
		}
		return out
	}
	return map[string]interface{}{
		"id": v.ID, "title": v.Title, "wave": v.Wave, "role": v.Role, "depends": v.Depends,
	}
}

func RunNext(args cli.Args) int {
	if args.Bool("dispatch") {
		return runDispatch(args)
	}
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd next <slug> [--all] [--dispatch] [--json]")
	if !ok {
		return code
	}
	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		return specdExit(err)
	}
	state := loaded.State
	doc := loaded.Doc
	jsonOut := args.Bool("json")
	// Derive the DAG view once; RunNext never mutates state.Tasks afterward, so
	// every scheduling query below reuses this slice (Stage 06 F2).
	dag := core.DagTasksFromState(state)

	if code, blocked := approvalGateBlocked(args, state, slug, jsonOut); blocked {
		return code
	}

	if args.Bool("all") {
		frontier := core.RunnableFrontier(dag)
		if jsonOut {
			tasks := make([]interface{}, len(frontier))
			for i, f := range frontier {
				tasks[i] = taskJSON(doc, state, f.ID)
			}
			if err := core.PrintJSON(map[string]interface{}{"kind": "frontier", "count": len(frontier), "tasks": tasks}); err != nil {
				return specdExit(err)
			}
			return core.ExitOK
		}
		if len(frontier) == 0 {
			r := core.NextRunnable(dag)
			if msg := frontierStuckReason(r, "✓ all tasks complete — nothing runnable."); msg != "" {
				fmt.Println(msg)
			}
			return core.ExitOK
		}
		fmt.Printf("=== RUNNABLE FRONTIER (%d) — dispatch in parallel ===\n", len(frontier))
		for _, f := range frontier {
			t := core.FindTask(doc, f.ID)
			title := state.Tasks[f.ID].Title
			role := state.Tasks[f.ID].Role
			if t != nil {
				title = t.Title
				role = t.Meta["role"]
			}
			fmt.Printf("  %s  [wave %d]  %s  (%s)\n", f.ID, f.Wave, title, role)
		}
		fmt.Println("==============================")
		fmt.Printf("Each: specd next %s (focused) or complete with specd task %s <id> --status complete --evidence \"<proof>\"\n", slug, slug)
		return core.ExitOK
	}

	result := core.NextRunnable(dag)

	if jsonOut {
		var payload any = result
		if result.Kind == core.NextTask {
			payload = map[string]interface{}{
				"kind": result.Kind,
				"id":   result.ID,
				"task": taskJSON(doc, state, result.ID),
			}
		}
		if err := core.PrintJSON(payload); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}

	switch result.Kind {
	case core.NextAllComplete:
		if state.Status == core.StatusVerifying {
			fmt.Printf("✓ all tasks complete — VERIFY the spec, then `specd approve %s` to accept and finish (→ REFLECT).\n", slug)
		} else {
			fmt.Println("✓ all tasks complete — nothing runnable. Move to REFLECT.")
		}
	case core.NextAllBlocked:
		fmt.Printf("⚠ all remaining tasks blocked: %v\n", result.Blocked)
		for _, b := range state.Blockers {
			fmt.Printf("  %s: %s\n", b.Task, b.Reason)
		}
	case core.NextWaiting:
		fmt.Printf("… waiting — frontier gated by incomplete deps: %v\n", result.Blocking)
	case core.NextTask:
		t := core.FindTask(doc, result.ID)
		ts := state.Tasks[result.ID]
		m := map[string]string{}
		title := ts.Title
		role := ts.Role
		if t != nil {
			m = t.Meta
			title = t.Title
			role = m["role"]
		}
		fmt.Printf("=== NEXT TASK: %s ===\n", result.ID)
		fmt.Printf("title:        %s\n", title)
		fmt.Printf("role:         %s\n", role)
		fmt.Printf("why:          %s\n", m["why"])
		fmt.Printf("files:        %s\n", m["files"])
		fmt.Printf("contract:     %s\n", m["contract"])
		fmt.Printf("acceptance:   %s\n", m["acceptance"])
		fmt.Printf("verify:       %s\n", m["verify"])
		dep := m["depends"]
		if dep == "" {
			dep = "—"
		}
		fmt.Printf("depends:      %s\n", dep)
		if reqs, ok := m["requirements"]; ok {
			fmt.Printf("requirements: %s\n", reqs)
		}
		fmt.Println("==============================")
		fmt.Printf("When done: specd task %s %s --status complete --evidence \"<proof>\"\n", slug, result.ID)
	}
	return core.ExitOK
}
