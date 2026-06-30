package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// RunApprove implements `specd approve`. Under the spec lock it handles three
// cases in order: clearing an awaiting-approval gate, accepting a verifying
// spec as complete (enforcing the acceptance-criteria gate when configured),
// or advancing a planning-phase spec to the next status — printing the result
// as JSON or human-readable text.
//
//nolint:gocyclo // pre-existing complexity debt, out of scope for spec S3 — tracked for a future cleanup pass
func RunApprove(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd approve <slug> [--json]")
	if !ok {
		return code
	}
	jsonOut := args.Bool("json")

	result, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		loaded, err := core.LoadSpec(root, slug)
		if err != nil {
			return specdExit(err), err
		}
		state := loaded.State
		doc := loaded.Doc

		// Case 1: clear awaiting-approval gate.
		if state.Gate == core.GateAwaitingApproval {
			state.Gate = core.GateNone
			if err := core.SaveState(root, slug, state); err != nil {
				return specdExit(err), err
			}
			if jsonOut {
				if err := core.PrintJSON(map[string]interface{}{"ok": true, "action": "gate-cleared", "status": state.Status, "phase": state.Phase}); err != nil {
					return specdExit(err), err
				}
			} else {
				fmt.Printf("approve: gate cleared — resume at status '%s' (phase %s).\n", state.Status, state.Phase)
			}
			return core.ExitOK, nil
		}

		// Case 2: verifying → complete.
		if state.Status == core.StatusVerifying {
			cfg := core.LoadConfig(root)
			if cfg.Gates.Acceptance == "required" {
				gaps := core.GetAcceptanceGaps(state, core.ReadArtifact(root, slug, "requirements.md"))
				if len(gaps.Unmet) > 0 || len(gaps.Failed) > 0 {
					var problems []string
					for _, n := range gaps.Unmet {
						problems = append(problems, fmt.Sprintf("requirement %d: no passing acceptance criterion", n))
					}
					for _, k := range gaps.Failed {
						problems = append(problems, fmt.Sprintf("criterion %s: recorded as fail", k))
					}
					if jsonOut {
						if err := core.PrintJSON(map[string]interface{}{"ok": false, "action": "blocked", "status": state.Status, "problems": problems}); err != nil {
							return specdExit(err), err
						}
					} else {
						for _, p := range problems {
							errLine("fail  %s", p)
						}
						errLine("\n✗ cannot approve verification — %d unmet acceptance criterion/criteria. Record with `specd verify %s --criterion <r>.<n> --status pass --evidence \"...\"`.", len(problems), slug)
					}
					return core.ExitGate, nil
				}
			}
			from := state.Status
			state.Status = core.StatusComplete
			state.Phase = core.PhaseForStatus(core.StatusComplete)
			if err := core.SaveState(root, slug, state); err != nil {
				return specdExit(err), err
			}
			if jsonOut {
				if err := core.PrintJSON(map[string]interface{}{"ok": true, "action": "verified", "from": from, "status": state.Status, "phase": state.Phase}); err != nil {
					return specdExit(err), err
				}
			} else {
				fmt.Printf("approve: verification accepted → status 'complete' (phase %s).\n", state.Phase)
			}
			return core.ExitOK, nil
		}

		// Case 3: planning ratchet.
		advance, ok := core.PlanningAdvance[state.Status]
		if !ok {
			return specdExit(core.GateError(fmt.Sprintf("approve: nothing to approve — spec '%s' is '%s'.", slug, state.Status))), core.GateError("")
		}
		problems := core.PhaseReadiness(state.Status, core.ReadArtifact(root, slug, "requirements.md"), core.ReadArtifact(root, slug, "design.md"), doc)
		if len(problems) > 0 {
			if jsonOut {
				if err := core.PrintJSON(map[string]interface{}{"ok": false, "action": "blocked", "status": state.Status, "problems": problems}); err != nil {
					return specdExit(err), err
				}
			} else {
				for _, p := range problems {
					errLine("fail  %s", p)
				}
				errLine("\n✗ cannot approve '%s' — %d gate violation(s). Fix and retry.", state.Status, len(problems))
			}
			return core.ExitGate, nil
		}
		from := state.Status
		state.Status = advance.Status
		state.Phase = advance.Phase
		if err := core.SaveState(root, slug, state); err != nil {
			return specdExit(err), err
		}
		if jsonOut {
			if err := core.PrintJSON(map[string]interface{}{"ok": true, "action": "advanced", "from": from, "status": state.Status, "phase": state.Phase}); err != nil {
				return specdExit(err), err
			}
		} else {
			fmt.Printf("approve: '%s' approved → status '%s' (phase %s).\n", from, state.Status, state.Phase)
		}
		return core.ExitOK, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return result
}
