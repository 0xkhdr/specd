package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

type dispatchPacket struct {
	ID           string   `json:"id"`
	Wave         int      `json:"wave"`
	Role         string   `json:"role"`
	RolePrompt   string   `json:"rolePrompt"`
	Title        string   `json:"title"`
	Why          string   `json:"why"`
	Contract     string   `json:"contract"`
	Files        string   `json:"files"`
	Acceptance   string   `json:"acceptance"`
	Verify       string   `json:"verify"`
	Depends      []string `json:"depends"`
	Requirements []int    `json:"requirements"`
	Completion   string   `json:"completion"`
}

func RunDispatch(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if slug == "" {
		return usageExit("usage: specd dispatch <slug> [--json]")
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

	frontier := core.RunnableFrontier(core.DagTasksFromState(state))

	if len(frontier) == 0 {
		r := core.NextRunnable(core.DagTasksFromState(state))
		if jsonOut {
			out := map[string]interface{}{"kind": "frontier", "count": 0, "reason": r.Kind, "packets": []interface{}{}}
			if err := core.PrintJSON(out); err != nil {
				return specdExit(err)
			}
			return core.ExitOK
		}
		switch r.Kind {
		case core.NextAllComplete:
			fmt.Println("✓ all tasks complete — nothing to dispatch.")
		case core.NextAllBlocked:
			fmt.Printf("⚠ all remaining tasks blocked: %v\n", r.Blocked)
		case core.NextWaiting:
			fmt.Printf("… waiting — frontier gated by incomplete deps: %v\n", r.Blocking)
		}
		return core.ExitOK
	}

	roleCache := make(map[string]string)
	rolePromptFor := func(role string) string {
		if v, ok := roleCache[role]; ok {
			return v
		}
		rp := core.ReadRole(root, role)
		s := ""
		if rp != nil {
			s = *rp
		}
		roleCache[role] = s
		return s
	}

	packets := make([]dispatchPacket, len(frontier))
	for i, f := range frontier {
		t := core.FindTask(doc, f.ID)
		ts := state.Tasks[f.ID]
		role := ts.Role
		if t != nil && t.Meta["role"] != "" {
			role = t.Meta["role"]
		}
		m := map[string]string{}
		title := ts.Title
		if t != nil {
			m = t.Meta
			title = t.Title
		}
		depends := ts.Depends
		if depends == nil {
			depends = []string{}
		}
		reqs := ts.Requirements
		if reqs == nil {
			reqs = []int{}
		}
		packets[i] = dispatchPacket{
			ID: f.ID, Wave: f.Wave, Role: role, RolePrompt: rolePromptFor(role),
			Title: title, Why: m["why"], Contract: m["contract"], Files: m["files"],
			Acceptance: m["acceptance"], Verify: m["verify"],
			Depends: depends, Requirements: reqs,
			Completion: fmt.Sprintf("specd task %s %s --status complete --evidence \"<proof>\"", slug, f.ID),
		}
	}

	if jsonOut {
		if err := core.PrintJSON(map[string]interface{}{"kind": "frontier", "count": len(packets), "packets": packets}); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}

	fmt.Printf("=== DISPATCH FRONTIER (%d) — fan out to parallel subagents ===\n", len(packets))
	for _, p := range packets {
		v := p.Verify
		if v == "" {
			v = "—"
		}
		fmt.Printf("  %s  [wave %d]  %s  (%s)\n", p.ID, p.Wave, p.Title, p.Role)
		fmt.Printf("      verify: %s\n", v)
		fmt.Printf("      done:   %s\n", p.Completion)
	}
	fmt.Println("==============================")
	fmt.Printf("Full packets (role prompt + contract + files + acceptance): specd dispatch %s --json\n", slug)
	return core.ExitOK
}
