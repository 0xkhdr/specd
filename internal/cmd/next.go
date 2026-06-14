package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func taskJSON(doc core.ParsedTasks, state *core.State, id string) map[string]interface{} {
	t := core.FindTask(doc, id)
	if t != nil {
		out := map[string]interface{}{
			"id": t.ID, "title": t.Title, "wave": t.Wave,
		}
		for k, v := range t.Meta {
			out[k] = v
		}
		return out
	}
	ts := state.Tasks[id]
	return map[string]interface{}{
		"id": id, "title": ts.Title, "wave": ts.Wave, "role": ts.Role, "depends": ts.Depends,
	}
}

func RunNext(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if slug == "" {
		return usageExit("usage: specd next <slug> [--json]")
	}
	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		return specdExit(err)
	}
	state := loaded.State
	doc := loaded.Doc
	jsonOut := args.Bool("json")

	if state.Gate == core.GateAwaitingApproval && !args.Bool("force") {
		if jsonOut {
			if err := core.PrintJSON(map[string]interface{}{"kind": "gated", "gate": state.Gate}); err != nil {
				return specdExit(err)
			}
		} else {
			errLine("⛔ gate awaiting-approval — present the revised plan, then `specd approve %s` (override: --force).", slug)
		}
		return core.ExitGate
	}

	if args.Bool("all") {
		frontier := core.RunnableFrontier(core.DagTasksFromState(state))
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
			r := core.NextRunnable(core.DagTasksFromState(state))
			switch r.Kind {
			case core.NextAllComplete:
				fmt.Println("✓ all tasks complete — nothing runnable.")
			case core.NextAllBlocked:
				fmt.Printf("⚠ all remaining tasks blocked: %v\n", r.Blocked)
			case core.NextWaiting:
				fmt.Printf("… waiting — frontier gated by incomplete deps: %v\n", r.Blocking)
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

	result := core.NextRunnable(core.DagTasksFromState(state))

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
