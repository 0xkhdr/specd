package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// modePayload is the typed schema for `specd mode --json` (show / set). Field
// order matches the human output: the effective mode, how it was chosen, and
// whether the project can run orchestration at all (capability ≠ selection).
type modePayload struct {
	Spec       string `json:"spec"`
	Mode       string `json:"mode"`
	Origin     string `json:"origin"`
	Capability bool   `json:"orchestrationCapable"`
}

func runMode(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd mode <slug> [--set simple|orchestrated] [--recommend] [--json]")
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	jsonOut := args.Bool("json")

	if args.Bool("recommend") {
		return runModeRecommend(root, slug, jsonOut)
	}

	if args.Has("set") {
		return runModeSet(root, slug, args.Str("set"), jsonOut)
	}

	// Default: show effective mode + origin + capability.
	loaded, err := core.LoadSpec(root, slug)
	if err != nil {
		return specdExit(err)
	}
	return printMode(slug, loaded.State, root, jsonOut)
}

// runModeSet records a new per-spec execution mode, failing closed when
// orchestration is requested without project capability.
func runModeSet(root, slug, target string, jsonOut bool) int {
	if target != core.ModeSimple && target != core.ModeOrchestrated {
		core.Error(fmt.Sprintf("--set: invalid mode %q, expected simple|orchestrated", target))
		return core.ExitUsage
	}
	if target == core.ModeOrchestrated && !core.ProjectOrchestrationEnabled(root) {
		core.Error("--set orchestrated: project has no orchestration capability. Enable it with `specd init --orchestration session` (or manual|planning), then retry.")
		return core.ExitGate
	}

	result, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		loaded, err := core.LoadSpec(root, slug)
		if err != nil {
			return specdExit(err), err
		}
		state := loaded.State

		// Refuse switching to Simple while a Brain session is live — cancel first
		// so the running session is never orphaned.
		if target == core.ModeSimple && state.EffectiveMode() == core.ModeOrchestrated {
			if session, err := core.ActiveOrchestrationSessionForSpec(root, slug); err == nil && session != nil {
				core.Error(fmt.Sprintf("cannot switch '%s' to simple: Brain session %s is active. Cancel it first with `specd brain cancel %s`.", slug, session.SessionID, slug))
				return core.ExitGate, core.GateError("brain session active")
			}
		}

		if state.EffectiveMode() == target {
			// No-op switch: report current, do not bump revision.
			return printMode(slug, state, root, jsonOut), nil
		}

		if target == core.ModeSimple {
			// Opting out: clear the fields so Simple state stays byte-stable.
			state.ExecutionMode = ""
			state.ModeOrigin = ""
		} else {
			state.ExecutionMode = target
			state.ModeOrigin = core.OriginUser
		}
		if err := core.SaveState(root, slug, state); err != nil {
			return specdExit(err), err
		}
		return printMode(slug, state, root, jsonOut), nil
	})
	if err != nil {
		return result
	}
	return result
}

// runModeRecommend emits the deterministic advisory recommendation. The verdict
// never changes the recorded mode — it is input to the user's decision.
func runModeRecommend(root, slug string, jsonOut bool) int {
	rec, err := core.RecommendMode(root, slug)
	if err != nil {
		return specdExit(err)
	}
	if jsonOut {
		if err := core.PrintJSON(rec); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	fmt.Printf("recommend: %s (%s)\n", rec.Recommended, rec.Confidence)
	fmt.Printf("  signals: tasks=%d maxWaveWidth=%d roles=%d crossSpecEdges=%d estTokens=%d\n",
		rec.Signals.TaskCount, rec.Signals.MaxWaveWidth, rec.Signals.DistinctRoles,
		rec.Signals.CrossSpecEdges, rec.Signals.EstimatedTokens)
	fmt.Printf("  %s\n", rec.Rationale)
	fmt.Println("  (advisory — you decide; `specd mode " + slug + " --set orchestrated` to opt in)")
	return core.ExitOK
}

func printMode(slug string, state *core.State, root string, jsonOut bool) int {
	mode := state.EffectiveMode()
	origin := state.ModeOrigin
	if origin == "" {
		origin = core.OriginDefault
	}
	capable := core.ProjectOrchestrationEnabled(root)
	if jsonOut {
		if err := core.PrintJSON(modePayload{Spec: slug, Mode: mode, Origin: origin, Capability: capable}); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	fmt.Printf("mode: %s (origin %s)\n", mode, origin)
	if capable {
		fmt.Println("  project orchestration capability: available")
	} else {
		fmt.Println("  project orchestration capability: not enabled (`specd init --orchestration session` to enable)")
	}
	return core.ExitOK
}
