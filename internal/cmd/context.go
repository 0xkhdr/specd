package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
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

func RunContext(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if slug == "" {
		return usageExit("usage: specd context <slug> [--json]")
	}
	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		return specdExit(err)
	}
	state := loaded.State
	cfg := core.LoadConfig(root)
	jsonOut := args.Bool("json")

	b := buildBrief(state, slug, cfg.DefaultVerify)
	c := core.CountTasks(state)
	load := append(baseSteering, b.load...)
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
			"phaseLabel": b.phaseLabel, "purpose": b.purpose, "load": load,
			"focus": focus, "next": next,
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
	fmt.Printf("tasks: %d/%d done · next: %s\n", c.Complete, c.Total, core.NextSummary(state))
	fmt.Println()
	fmt.Printf("PHASE %s — %s\n", b.phaseLabel, b.purpose)
	fmt.Println()
	fmt.Println("LOAD NOW (minimal — don't dump the rest):")
	for _, f := range load {
		fmt.Printf("  - %s\n", f)
	}
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
