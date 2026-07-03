package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

const orchestrateUsage = "usage: specd orchestrate <slug> <status|resume> [--override] [--json]"

// RunOrchestrate surfaces and resolves auto-escalations (V7/P3.2). `status`
// prints the current escalation record and the advisory conductor-handoff
// recommendation; `resume --override` is the human override path that clears the
// escalation so orchestration may proceed. The binary never auto-clears an
// escalation and never auto-switches mode — resolution is always an explicit
// human action (invariant: never auto-switch).
func RunOrchestrate(args cli.Args) int {
	if len(args.Pos) < 2 {
		return usageExit(orchestrateUsage)
	}
	root, slug, code, ok := requireRootAndSlug(args, orchestrateUsage)
	if !ok {
		return code
	}
	action := args.Pos[1]
	switch action {
	case "status":
		return orchestrateStatus(root, slug, args.Bool("json"))
	case "resume":
		return orchestrateResume(root, slug, args)
	default:
		return usageExit(orchestrateUsage)
	}
}

func orchestrateStatus(root, slug string, jsonOut bool) int {
	state, err := core.LoadState(root, slug)
	if err != nil {
		return specdExit(err)
	}
	if state == nil {
		return specdExit(core.NotFoundError(fmt.Sprintf("spec '%s' not found", slug)))
	}
	rec := RecommendConductorHandoff(state)
	if jsonOut {
		out := map[string]interface{}{"spec": slug, "escalation": state.Escalation, "recommendation": rec}
		if err := core.PrintJSON(out); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	if state.Escalation == nil {
		fmt.Printf("orchestrate %s: no active escalation\n", slug)
		return core.ExitOK
	}
	e := state.Escalation
	fmt.Printf("orchestrate %s: escalated\n", slug)
	fmt.Printf("  task:  %s\n", e.Task)
	fmt.Printf("  rule:  %s\n", e.Rule)
	fmt.Printf("  facts: %s\n", e.Facts)
	fmt.Printf("  time:  %s\n", e.Time)
	fmt.Printf("  → recommend: %s\n", rec.Rationale)
	return core.ExitOK
}

// RecommendConductorHandoff wraps the core recommendation, substituting the
// concrete spec slug into the rationale placeholder.
func RecommendConductorHandoff(state *core.State) core.ConductorHandoffRecommendation {
	return core.RecommendConductorForEscalation(state)
}

func orchestrateResume(root, slug string, args cli.Args) int {
	// The override flag is mandatory: clearing an escalation is a deliberate human
	// decision, never an implicit side effect of resuming.
	if !args.Bool("override") {
		return specdExit(core.GateError("orchestrate resume requires --override to clear an active escalation (the escalation is evidence; overriding it is a human decision)"))
	}
	rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		state, err := core.LoadState(root, slug)
		if err != nil {
			return core.ExitGate, err
		}
		if state == nil {
			return core.ExitNotFound, core.NotFoundError(fmt.Sprintf("spec '%s' not found", slug))
		}
		if state.Escalation == nil {
			return core.ExitOK, core.GateError(fmt.Sprintf("orchestrate resume: spec '%s' has no active escalation to override", slug))
		}
		cleared := state.Escalation
		state.Escalation = nil
		if err := core.SaveState(root, slug, state); err != nil {
			return core.ExitGate, err
		}
		if args.Bool("json") {
			return core.ExitOK, core.PrintJSON(map[string]interface{}{"ok": true, "action": "override", "cleared": cleared})
		}
		fmt.Printf("orchestrate %s: escalation on task %s (rule %s) overridden — resuming\n", slug, cleared.Task, cleared.Rule)
		return core.ExitOK, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}
