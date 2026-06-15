package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// frontierOut is the typed schema for the non-empty dispatch frontier response.
// Field order matches the previous map output (encoder sorts map keys) so the
// emitted bytes are unchanged.
type frontierOut struct {
	Count   int              `json:"count"`
	Kind    string           `json:"kind"`
	Packets []dispatchPacket `json:"packets"`
}

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
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd dispatch <slug> [--json]")
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

	if code, blocked := approvalGateBlocked(args, state, slug, jsonOut); blocked {
		return code
	}

	// Derive the DAG view once and reuse it for both scheduling queries below.
	dag := core.DagTasksFromState(state)
	frontier := core.RunnableFrontier(dag)

	if len(frontier) == 0 {
		r := core.NextRunnable(dag)
		if jsonOut {
			out := map[string]interface{}{"kind": "frontier", "count": 0, "reason": r.Kind, "packets": []interface{}{}}
			if err := core.PrintJSON(out); err != nil {
				return specdExit(err)
			}
			return core.ExitOK
		}
		if msg := frontierStuckReason(r, "✓ all tasks complete — nothing to dispatch."); msg != "" {
			fmt.Println(msg)
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
		v := core.ResolveTaskView(doc, state, f.ID)
		depends := v.Depends
		if depends == nil {
			depends = []string{}
		}
		reqs := v.Requirements
		if reqs == nil {
			reqs = []int{}
		}
		packets[i] = dispatchPacket{
			ID: f.ID, Wave: f.Wave, Role: v.Role, RolePrompt: rolePromptFor(v.Role),
			Title: v.Title, Why: v.Meta["why"], Contract: v.Meta["contract"], Files: v.Meta["files"],
			Acceptance: v.Meta["acceptance"], Verify: v.Meta["verify"],
			Depends: depends, Requirements: reqs,
			Completion: fmt.Sprintf("specd task %s %s --status complete --evidence \"<proof>\"", slug, f.ID),
		}
	}

	if jsonOut {
		if err := core.PrintJSON(frontierOut{Count: len(packets), Kind: "frontier", Packets: packets}); err != nil {
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
