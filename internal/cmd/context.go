package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	contextpkg "github.com/0xkhdr/specd/internal/context"
	"github.com/0xkhdr/specd/internal/core"
)

var baseSteering = []string{".specd/steering/reasoning.md", ".specd/steering/workflow.md"}

type brief struct {
	phaseLabel string
	purpose    string
	load       []string
	focus      string
	next       string
}

// phaseSkill names the stage skill the agent should load for the current spec
// status — the harness points at the right knowledge without being it.
func phaseSkill(status core.SpecStatus) string {
	switch status {
	case core.StatusRequirements:
		return ".specd/skills/specd-requirements/SKILL.md"
	case core.StatusDesign:
		return ".specd/skills/specd-design/SKILL.md"
	case core.StatusTasks:
		return ".specd/skills/specd-tasks/SKILL.md"
	case core.StatusExecuting, core.StatusBlocked, core.StatusVerifying:
		return ".specd/skills/specd-execute/SKILL.md"
	default:
		return ".specd/skills/specd-foundations/SKILL.md"
	}
}

func sp(slug string, files ...string) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = fmt.Sprintf(".specd/specs/%s/%s", slug, f)
	}
	return out
}

func buildBrief(state *core.State, slug, defaultVerify string) brief {
	next := core.NextSummary(state)
	switch state.Status {
	case core.StatusRequirements:
		return brief{"ANALYZE", "Pin down what must be true, in EARS.",
			append(sp(slug, "requirements.md"), ".specd/steering/product.md"),
			"Write/refine requirements.md acceptance criteria in EARS form.",
			fmt.Sprintf("specd check %s  →  specd approve %s", slug, slug)}
	case core.StatusDesign:
		return brief{"PLAN (design)", "Decide how the requirements get satisfied.",
			append(sp(slug, "requirements.md", "design.md"), ".specd/steering/tech.md", ".specd/steering/structure.md"),
			"Fill every design.md section (overview…risks); no TODOs, no empty sections.",
			fmt.Sprintf("specd check %s  →  specd approve %s", slug, slug)}
	case core.StatusTasks:
		return brief{"PLAN (tasks)", "Decompose the design into an ordered wave DAG.",
			sp(slug, "design.md", "tasks.md"),
			"Author tasks.md: each task carries why/role/files/contract/acceptance/verify/depends/requirements.",
			fmt.Sprintf("specd check %s  →  specd approve %s", slug, slug)}
	case core.StatusExecuting:
		return brief{"EXECUTE", "Build one task at a time, evidence-gated.",
			sp(slug, "tasks.md", "memory.md"),
			fmt.Sprintf("Run the next runnable task only: %s", next),
			fmt.Sprintf("specd next %s", slug)}
	case core.StatusBlocked:
		focus := "All remaining tasks blocked."
		if len(state.Blockers) > 0 {
			focus = "Resolve the blockers listed under SIGNALS."
		}
		return brief{"EXECUTE (blocked)", "Frontier is stuck — surface and resolve.",
			sp(slug, "tasks.md"),
			focus,
			fmt.Sprintf("specd status %s", slug)}
	case core.StatusVerifying:
		return brief{"VERIFY", "All tasks done — confirm the spec actually works.",
			sp(slug, "tasks.md", "requirements.md"),
			fmt.Sprintf("Run the spec-level verification (config defaultVerify: `%s`) and confirm acceptance criteria hold.", defaultVerify),
			fmt.Sprintf("specd approve %s   (accepts verification → REFLECT)", slug)}
	case core.StatusComplete:
		return brief{"REFLECT", "Capture what was learned; promote durable patterns.",
			sp(slug, "memory.md", "decisions.md"),
			"Record learnings in memory.md and any deviations in decisions.md.",
			fmt.Sprintf("specd memory %s promote --key <pattern>", slug)}
	}
	return brief{}
}

// buildContextManifest assembles the briefing-mode context manifest for a spec
// through the shared engine. In executing it scopes the manifest to the next
// runnable task (its role, declared files, covered requirements) so the brief
// targets the actual frontier work; other phases brief at the phase level. The
// injected reader lets the engine measure and slice the real artifacts.
func buildContextManifest(root, slug string, state *core.State, doc core.ParsedTasks) contextpkg.MissionContextManifest {
	req := contextpkg.ContextRequest{
		Slug:         slug,
		Status:       state.Status,
		Role:         "builder",
		Mode:         contextpkg.ContextModeBriefing,
		HostBudget:   core.HostContextBudgetFromEnv(),
		ReadArtifact: core.SpecArtifactReader(root, slug),
	}
	if state.Status == core.StatusExecuting {
		if next := core.NextRunnable(core.DagTasksFromState(state)); next.Kind == core.NextTask {
			v := core.ResolveTaskView(doc, state, next.ID)
			req.TaskID = next.ID
			req.Role = v.Role
			req.Files = core.SplitCSV(v.Meta["files"])
			req.Requirements = v.Requirements
		}
	}
	return contextpkg.BuildContextManifest(req)
}

// manifestJSON projects a context manifest into the additive `contextManifest`
// JSON block, surfacing the measured accounting (estimatedTokens, budget) and
// the ordered load items so any host can self-report its context cost.
func manifestJSON(m contextpkg.MissionContextManifest) map[string]interface{} {
	items := make([]map[string]interface{}, 0, len(m.Items))
	for _, it := range m.Items {
		item := map[string]interface{}{
			"order": it.Order, "kind": it.Kind, "mode": it.Mode,
			"required": it.Required, "tokenHint": it.TokenHint, "rationale": it.Rationale,
		}
		if it.Path != "" {
			item["path"] = it.Path
		}
		if it.Command != "" {
			item["command"] = it.Command
		}
		items = append(items, item)
	}
	return map[string]interface{}{
		"version": m.Version, "softTokenCeiling": m.SoftTokenCeiling, "strategy": m.Strategy,
		"estimatedTokens": m.EstimatedTokens, "budget": m.Budget, "items": items,
	}
}

// printContextManifest renders the manifest's load items as a compact table
// (required marker, item, mode, ~tokens, why) followed by the budget line so a
// human reader sees exactly what to load and how close it runs to the ceiling.
func printContextManifest(m contextpkg.MissionContextManifest) {
	for _, it := range m.Items {
		ref := it.Path
		if ref == "" {
			ref = it.Command
		}
		mark := " "
		if it.Required {
			mark = "*"
		}
		fmt.Printf("  %s %-44s %-18s ~%-5d %s\n", mark, ref, it.Mode, it.TokenHint, it.Rationale)
	}
	fmt.Printf("  (* = required)  est %d / budget %d tokens\n", m.EstimatedTokens, m.Budget)
}

func RunContext(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd context <slug> [--json]")
	if !ok {
		return code
	}
	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		return specdExit(err)
	}
	state := loaded.State
	cfg := core.LoadConfig(root)
	jsonOut := args.Bool("json")

	b := buildBrief(state, slug, cfg.DefaultVerify)
	skill := phaseSkill(state.Status)
	c := core.CountTasks(state)
	load := append([]string{}, baseSteering...)
	load = append(load, b.load...)
	manifest := buildContextManifest(root, slug, state, loaded.Doc)
	gated := state.Gate == core.GateAwaitingApproval

	reqMd := core.ReadArtifact(root, slug, "requirements.md")
	blockers := core.BlockerLines(state)
	var midreq *core.MidreqSummary
	if gated {
		midreq = core.LatestMidreq(root, slug)
	}
	uncovered := []int{}
	if state.Status == core.StatusVerifying {
		if u := core.UncoveredRequirements(state, reqMd); u != nil {
			uncovered = u
		}
	}

	if jsonOut {
		focus := b.focus
		next := b.next
		if gated {
			focus = "GATE awaiting-approval — present the revised plan, do not hand out work."
			next = fmt.Sprintf("specd approve %s", slug)
		}
		out := map[string]interface{}{
			"spec": slug, "title": state.Title, "status": state.Status, "phase": state.Phase,
			"gate": state.Gate, "turn": state.Turn, "counts": c,
			"mode": state.EffectiveMode(), "modeOrigin": modeOriginOrDefault(state),
			"phaseLabel": b.phaseLabel, "purpose": b.purpose, "load": load,
			"skill": skill, "focus": focus, "next": next,
			"contextManifest": manifestJSON(manifest),
			"signals": map[string]interface{}{
				"blockers":              blockers,
				"latestMidreq":          midreq,
				"uncoveredRequirements": uncovered,
			},
		}
		if err := core.PrintJSON(out); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}

	fmt.Printf("=== CONTEXT: %s ===\n", slug)
	fmt.Printf("%s · status %s · phase %s · turn %d\n", state.Title, state.Status, state.Phase, state.Turn)
	fmt.Printf("mode: %s — %s\n", state.EffectiveMode(), modeBriefing(state.EffectiveMode()))
	fmt.Printf("tasks: %d/%d done · next: %s\n", c.Complete, c.Total, core.NextSummary(state))
	fmt.Println()
	fmt.Printf("PHASE %s — %s\n", b.phaseLabel, b.purpose)
	fmt.Println()
	fmt.Println("LOAD NOW (minimal — measured budget, don't dump the rest):")
	printContextManifest(manifest)
	fmt.Printf("SKILL: %s\n", skill)
	fmt.Println()

	var signals []string
	for _, bl := range blockers {
		signals = append(signals, "! blocker "+bl)
	}
	if len(uncovered) > 0 {
		uncovStr := fmt.Sprint(uncovered)
		signals = append(signals, "! uncovered requirements (no covering task): "+uncovStr)
	}
	if len(signals) > 0 {
		fmt.Println("SIGNALS:")
		for _, s := range signals {
			fmt.Printf("  %s\n", s)
		}
		fmt.Println()
	}

	if gated {
		fmt.Println("⛔ GATE awaiting-approval — present the revised plan; work is frozen.")
		if midreq != nil {
			fmt.Printf("   ↳ midreq Turn %d (%s): \"%s\"\n", midreq.Turn, midreq.Impact, midreq.Input)
		}
		fmt.Printf("NEXT: specd approve %s\n", slug)
		return core.ExitOK
	}
	fmt.Printf("FOCUS: %s\n", b.focus)
	fmt.Printf("NEXT:  %s\n", b.next)
	return core.ExitOK
}
