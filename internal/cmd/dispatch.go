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
	Count int    `json:"count"`
	Kind  string `json:"kind"`
	// Assets is the shared role-asset map (`role/<name>` -> path) emitted once per
	// response so a multi-task wave on one role carries the role prompt bytes at
	// most once. Omitted under --inline-roles (packets carry full text instead).
	Assets  map[string]string `json:"assets,omitempty"`
	Packets []dispatchPacket  `json:"packets"`
}

type dispatchPacket struct {
	ID   string `json:"id"`
	Wave int    `json:"wave"`
	Role string `json:"role"`
	// RolePath references the role asset by path (resolve via the response-level
	// `assets` map). RolePrompt is only populated under --inline-roles, where the
	// full role text is inlined per packet for hosts that cannot resolve paths.
	RolePath        string                       `json:"rolePath"`
	RolePrompt      string                       `json:"rolePrompt,omitempty"`
	Title           string                       `json:"title"`
	Why             string                       `json:"why"`
	Contract        string                       `json:"contract"`
	Files           string                       `json:"files"`
	Acceptance      string                       `json:"acceptance"`
	Verify          string                       `json:"verify"`
	Depends         []string                     `json:"depends"`
	Requirements    []int                        `json:"requirements"`
	ContextManifest *core.MissionContextManifest `json:"contextManifest,omitempty"`
	Completion      string                       `json:"completion"`
}

// manifestRolePath returns the role asset path the context engine resolved for
// this packet (the first "role"-kind item). It is the canonical path packets
// reference instead of inlining the role text.
func manifestRolePath(m core.MissionContextManifest) string {
	for _, it := range m.Items {
		if it.Kind == "role" {
			return it.Path
		}
	}
	return ""
}

func RunDispatch(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd dispatch <slug> [--json] [--inline-roles]")
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

	inlineRoles := args.Bool("inline-roles")
	reader := core.SpecArtifactReader(root, slug)
	assets := map[string]string{}
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
		mf := core.BuildContextManifest(core.ContextRequest{
			Slug: slug, Status: state.Status, TaskID: f.ID, Role: v.Role,
			Files: core.SplitCSV(v.Meta["files"]), Requirements: reqs,
			Mode: core.ContextModeDispatch, ReadArtifact: reader,
		})
		rolePath := manifestRolePath(mf)
		assets["role/"+v.Role] = rolePath
		p := dispatchPacket{
			ID: f.ID, Wave: f.Wave, Role: v.Role, RolePath: rolePath,
			Title: v.Title, Why: v.Meta["why"], Contract: v.Meta["contract"], Files: v.Meta["files"],
			Acceptance: v.Meta["acceptance"], Verify: v.Meta["verify"],
			Depends: depends, Requirements: reqs, ContextManifest: &mf,
			Completion: fmt.Sprintf("specd task %s %s --status complete --evidence \"<proof>\"", slug, f.ID),
		}
		if inlineRoles {
			// Back-compat escape hatch: inline the full role text per packet for
			// hosts that cannot resolve the shared asset path.
			p.RolePrompt = rolePromptFor(v.Role)
		}
		packets[i] = p
	}

	if jsonOut {
		out := frontierOut{Count: len(packets), Kind: "frontier", Assets: assets, Packets: packets}
		if inlineRoles {
			// Roles are inlined per packet; drop the shared map to reproduce the
			// pre-dedupe response shape.
			out.Assets = nil
		}
		if err := core.PrintJSON(out); err != nil {
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
	fmt.Printf("Full packets (shared role assets + context manifest + contract + files): specd dispatch %s --json\n", slug)
	fmt.Printf("Inline role text per packet (back-compat): specd dispatch %s --json --inline-roles\n", slug)
	return core.ExitOK
}
