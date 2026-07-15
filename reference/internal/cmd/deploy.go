package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/runner"
)

const deployUsage = "usage: specd deploy <slug> --env <env> [--dry-run] [--json]  |  specd deploy rollback <slug> --env <env> [--json]"

// productionEnvName is the env whose deploys hard-require a human approval
// record regardless of the plan's approvalRequired flag (V9 §3, invariant:
// human at boundaries).
const productionEnvName = "production"

// RunDeploy implements `specd deploy` (V9/P5.1): the evidence-gated deploy
// driver runner and its rollback. The bare form refuses unless the spec is
// complete, required gates are recorded green, and (for production or an
// approval-required plan) a human deploy approval exists; then it runs the
// plan's steps sequenced through the shared sandboxed exec path, appending every
// result to deploy.jsonl. `deploy rollback` replays the recorded inverse chain
// in reverse. No CD logic is embedded — steps are operator-authored commands.
func RunDeploy(args cli.Args) int {
	if len(args.Pos) >= 1 && args.Pos[0] == "rollback" {
		return runDeployRollback(args)
	}
	root, slug, code, ok := requireRootAndSlug(args, deployUsage)
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	env := strings.TrimSpace(args.Str("env"))
	if env == "" {
		return usageExit(deployUsage)
	}
	plan, err := core.LoadDeployPlan(root, env)
	if err != nil {
		return specdExit(err)
	}
	state, err := core.LoadState(root, slug)
	if err != nil {
		return specdExit(err)
	}
	problems := core.DeployPreconditions(state, plan, env == productionEnvName)
	if len(problems) > 0 {
		return renderDeployBlocked(slug, env, problems, args.Bool("json"))
	}

	if args.Bool("dry-run") {
		return renderDeployDryRun(slug, plan, args.Bool("json"))
	}

	return execDeployPlan(root, slug, plan, args.Bool("json"))
}

// execDeployPlan runs each step in order, recording results, and stops at the
// first failure. A mid-chain failure returns exit gate — the operator runs
// `specd deploy rollback` to unwind the recorded successful steps.
func execDeployPlan(root, slug string, plan core.DeployPlan, jsonOut bool) int {
	cfg := core.LoadConfig(root)
	sandbox := cfg.Deploy.Sandbox
	approved := plan.ApprovalRequired || plan.Env == productionEnvName
	ran := 0
	for _, step := range plan.Steps {
		res := execDeployCommand(root, step.Command, sandbox, time.Duration(step.TimeoutSeconds)*time.Second)
		entry := core.DeployLedgerEntry{
			Env:             plan.Env,
			Kind:            "step",
			Step:            step.Name,
			RollbackCommand: step.RollbackCommand,
			ExitCode:        res.ExitCode,
			Success:         res.ExitCode == 0,
		}
		if err := core.AppendDeployEntry(root, slug, entry); err != nil {
			return specdExit(err)
		}
		ran++
		if res.ExitCode != 0 {
			recordDeploy(root, slug, plan.Env, "failed", ran, approved)
			return renderDeployStepFailure(slug, step.Name, res, jsonOut)
		}
	}
	recordDeploy(root, slug, plan.Env, "succeeded", ran, approved)
	if jsonOut {
		return printJSONExit(map[string]interface{}{"ok": true, "env": plan.Env, "steps": ran, "outcome": "succeeded"})
	}
	fmt.Printf("✓ deploy: %d step(s) succeeded for '%s' → env %s\n", ran, slug, plan.Env)
	return core.ExitOK
}

// runDeployRollback replays the recorded inverse chain (successful steps of the
// most recent run, reverse order). A failing rollback step halts and returns
// exit 3 (blocked) — no automatic retry; a human resolves (V9 §7).
func runDeployRollback(args cli.Args) int {
	// Shift positionals so the shared slug resolver sees the slug (rollback <slug>).
	shifted := args
	shifted.Pos = args.Pos[1:]
	root, slug, code, ok := requireRootAndSlug(shifted, deployUsage)
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}
	env := strings.TrimSpace(args.Str("env"))
	if env == "" {
		return usageExit(deployUsage)
	}
	if err := core.ValidateEnv(env); err != nil {
		return specdExit(err)
	}
	entries, err := core.ReadDeployLedger(root, slug)
	if err != nil {
		return specdExit(err)
	}
	chain := core.RollbackChain(entries)
	if len(chain) == 0 {
		if args.Bool("json") {
			return printJSONExit(map[string]interface{}{"ok": true, "env": env, "rolledBack": 0, "note": "nothing to roll back"})
		}
		fmt.Printf("deploy rollback: nothing to roll back for '%s'\n", slug)
		return core.ExitOK
	}
	cfg := core.LoadConfig(root)
	sandbox := cfg.Deploy.Sandbox
	undone := 0
	for _, step := range chain {
		res := execDeployCommand(root, step.RollbackCommand, sandbox, deployRollbackTimeout)
		entry := core.DeployLedgerEntry{Env: env, Kind: "rollback", Step: step.Step, ExitCode: res.ExitCode, Success: res.ExitCode == 0}
		if err := core.AppendDeployEntry(root, slug, entry); err != nil {
			return specdExit(err)
		}
		if res.ExitCode != 0 {
			recordDeploy(root, slug, env, "rolled-back", undone, true)
			errLine("✗ deploy rollback: step %q failed (exit %d)%s — halted, resolve manually\n%s", step.Step, res.ExitCode, timedOutNote(res.TimedOut), strings.TrimSpace(res.Stderr))
			return core.ExitNotFound // blocked (3): human resolves
		}
		undone++
	}
	recordDeploy(root, slug, env, "rolled-back", undone, true)
	if args.Bool("json") {
		return printJSONExit(map[string]interface{}{"ok": true, "env": env, "rolledBack": undone})
	}
	fmt.Printf("✓ deploy rollback: unwound %d step(s) for '%s'\n", undone, slug)
	return core.ExitOK
}

// deployRollbackTimeout bounds each rollback command; rollback plans do not carry
// their own timeout, so a fixed bound applies.
const deployRollbackTimeout = 300 * time.Second

// execDeployCommand runs one deploy/rollback command through the shared
// sandboxed runner with a scrubbed env. An unavailable backend fails closed.
func execDeployCommand(root, command, sandbox string, timeout time.Duration) runner.RunResult {
	r, err := runner.SelectRunner(sandbox)
	if err != nil {
		return runner.RunResult{ExitCode: 1, Stderr: err.Error()}
	}
	shell := strings.TrimSpace(os.Getenv("SPECD_VERIFY_SHELL"))
	if shell == "" {
		shell = "sh"
	}
	return r.Run(context.Background(), runner.RunSpec{
		Root:    root,
		Shell:   shell,
		Command: command,
		Env:     core.ScrubbedEnv(),
		Timeout: timeout,
	})
}

// recordDeploy writes the deterministic deploy summary to state (dual-write with
// the ledger). A save failure is non-fatal to the deploy outcome already
// reported, but surfaced on stderr.
func recordDeploy(root, slug, env, outcome string, steps int, approved bool) {
	state, err := core.LoadState(root, slug)
	if err != nil || state == nil {
		return
	}
	state.Deploy = &core.DeployRecord{Env: env, Outcome: outcome, Steps: steps, Approved: approved, Time: core.NowISO()}
	if err := core.SaveState(root, slug, state); err != nil {
		errLine("warn: could not record deploy summary: %v", err)
	}
}

func renderDeployBlocked(slug, env string, problems []string, jsonOut bool) int {
	if jsonOut {
		return printJSONExit(map[string]interface{}{"ok": false, "env": env, "action": "blocked", "problems": problems})
	}
	errLine("✗ deploy blocked for '%s' (env %s) — %d precondition(s):", slug, env, len(problems))
	for _, p := range problems {
		errLine("  %s", p)
	}
	return core.ExitGate
}

func renderDeployDryRun(slug string, plan core.DeployPlan, jsonOut bool) int {
	if jsonOut {
		return printJSONExit(map[string]interface{}{"ok": true, "env": plan.Env, "steps": len(plan.Steps), "dryRun": true})
	}
	fmt.Printf("deploy dry-run: '%s' → env %s (%d step(s), preconditions clear)\n", slug, plan.Env, len(plan.Steps))
	for _, s := range plan.Steps {
		fmt.Printf("  %s: %s\n", s.Name, s.Command)
	}
	return core.ExitOK
}

func renderDeployStepFailure(slug, step string, res runner.RunResult, jsonOut bool) int {
	if jsonOut {
		return printJSONExit(map[string]interface{}{"ok": false, "action": "step-failed", "step": step, "exitCode": res.ExitCode, "timedOut": res.TimedOut})
	}
	errLine("✗ deploy: step %q failed (exit %d)%s for '%s' — run `specd deploy rollback %s --env <env>`\n%s",
		step, res.ExitCode, timedOutNote(res.TimedOut), slug, slug, strings.TrimSpace(res.Stderr))
	return core.ExitGate
}

// printJSONExit prints v as JSON and maps its "ok" field to an exit code.
func printJSONExit(v map[string]interface{}) int {
	if err := core.PrintJSON(v); err != nil {
		return specdExit(err)
	}
	if ok, _ := v["ok"].(bool); ok {
		return core.ExitOK
	}
	return core.ExitGate
}
