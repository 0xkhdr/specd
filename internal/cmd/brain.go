package cmd

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/obs"
)

// RunBrain implements `specd brain`, dispatching to the start/run/status/step/
// why/directive/pause/resume/cancel/checkpoint/compact/clear/ledger subcommands
// that drive Brain orchestration sessions for a single spec or, with
// --program, a whole multi-spec program.
//
//nolint:gocyclo // pre-existing complexity debt, out of scope for spec S3 — tracked for a future cleanup pass
func RunBrain(args cli.Args) int {
	if len(args.Pos) == 0 {
		return usageExit("usage: specd brain <start|run|status|step|why|directive|pause|resume|cancel|checkpoint|compact|clear|ledger> ...")
	}
	root, ok := core.FindSpecdRoot(".")
	if !ok {
		return specdExit(core.NotFoundError("not in a specd workspace"))
	}

	switch args.Pos[0] {
	case "start":
		if args.Bool("auto-step") {
			runArgs := args
			runArgs.Pos = append([]string{"run"}, args.Pos[1:]...)
			return RunBrain(runArgs)
		}
		if args.Bool("program") {
			if len(args.Pos) != 1 {
				return usageExit("usage: specd brain start --program --approval-policy <policy> --max-workers <n> --max-retries <n> --timeout-seconds <n> [--session <id>] [--cost-limit <usd>] [--json]")
			}
			sessionID, err := brainStartSessionID(args)
			if err != nil {
				return specdExit(err)
			}
			return brainProgramStep(root, sessionID, args)
		}
		if len(args.Pos) != 2 {
			return usageExit("usage: specd brain start <slug> --approval-policy <policy> --max-workers <n> --max-retries <n> --timeout-seconds <n> [--session <id>] [--cost-limit <usd>] [--json]")
		}
		return brainStart(root, args.Pos[1], args)
	case "run":
		if args.Bool("program") {
			if len(args.Pos) != 1 {
				return usageExit("usage: specd brain run --program [--approval-policy <policy>] [--worker-cmd <cmd>] [--max-steps <n>] [--session <id>] [--json]")
			}
			return brainRunProgram(root, args)
		}
		if len(args.Pos) != 2 {
			return usageExit("usage: specd brain run <slug> [--approval-policy <policy>] [--worker-cmd <cmd>] [--bootstrap] [--max-steps <n>] [--session <id>] [--json]")
		}
		return brainRun(root, args.Pos[1], args)
	case "step":
		if args.Bool("directive") {
			directiveArgs := args
			directiveArgs.Pos = []string{"directive"}
			return brainDirective(root, directiveArgs)
		}
		if args.Bool("program") {
			if len(args.Pos) != 1 || args.Str("session") == "" {
				return usageExit("usage: specd brain step --program --session <id> --approval-policy <policy> --max-workers <n> --max-retries <n> --timeout-seconds <n> [--cost-limit <usd>] [--json]")
			}
			return brainProgramStep(root, args.Str("session"), args)
		}
		if len(args.Pos) != 2 || args.Str("session") == "" {
			return usageExit("usage: specd brain step <slug> --session <id> --approval-policy <policy> --max-workers <n> --max-retries <n> --timeout-seconds <n> [--cost-limit <usd>] [--json]")
		}
		return brainStep(root, args.Pos[1], args.Str("session"), args)
	case "why":
		if len(args.Pos) != 1 {
			return usageExit("usage: specd brain why --session <id> [--json]")
		}
		return brainWhy(root, args)
	case "directive":
		return brainDirective(root, args)
	case "status":
		if args.Bool("verbose") {
			whyArgs := args
			whyArgs.Pos = []string{"why"}
			return brainWhy(root, whyArgs)
		}
		if args.Bool("ledger") {
			ledgerArgs := args
			ledgerArgs.Pos = append([]string{"ledger"}, args.Pos[1:]...)
			return brainLedger(root, ledgerArgs)
		}
		if args.Bool("program") {
			if len(args.Pos) != 1 || args.Str("session") == "" {
				return usageExit("usage: specd brain status --program --session <id> [--json]")
			}
			return brainProgramStatus(root, args)
		}
		if len(args.Pos) != 1 || args.Str("session") == "" {
			return usageExit("usage: specd brain status --session <id> [--json]")
		}
		session, err := core.LoadOrchestrationSession(root, args.Str("session"))
		if err != nil {
			return specdExit(err)
		}
		return printCommandResult(args, session)
	case "checkpoint":
		if args.Bool("compact") {
			return brainCompact(root, args, "")
		}
		return brainCheckpoint(root, args)
	case "compact":
		return brainCompact(root, args, "")
	case "clear":
		return brainCompact(root, args, "manual-clear")
	case "ledger":
		return brainLedger(root, args)
	case "pause":
		if args.Bool("program") {
			return brainProgramSessionControl(root, args, core.PauseProgramOrchestration, "pause")
		}
		return brainSessionControl(root, args, core.PauseOrchestration, "pause")
	case "resume":
		// --list is the host-facing discovery surface (R5): enumerate resumable
		// sessions without touching state. Layered on the existing verb; the
		// no-flag lifecycle-unpause behavior below is unchanged.
		if args.Bool("list") {
			return brainResumeList(root, args)
		}
		if args.Bool("program") {
			// With a worker wired, reconstruct the program frontier and restart the
			// driver (cross-spec recovery); without one, keep the lifecycle-unpause
			// control behavior.
			if args.Has("worker-cmd") {
				return brainResumeProgram(root, args)
			}
			return brainProgramSessionControl(root, args, core.ResumeProgramOrchestration, "resume")
		}
		return brainSessionControl(root, args, core.ResumeOrchestration, "resume")
	case "cancel":
		if args.Bool("program") {
			return brainProgramSessionControl(root, args, core.CancelProgramOrchestration, "cancel")
		}
		return brainSessionControl(root, args, core.CancelOrchestration, "cancel")
	default:
		return usageExit("usage: specd brain <start|run|status|step|why|directive|pause|resume|cancel|checkpoint|compact|clear|ledger> ...")
	}
}

// requireOrchestratedSpec refuses a Brain entrypoint when the spec is not in
// orchestrated execution mode. Simple is the default and the Brain/Pinky layer
// must never start a session for a Simple spec — the remediation points the host
// at the explicit opt-in. Returns ok=false with the exit code to return verbatim.
func requireOrchestratedSpec(root, slug string) (code int, ok bool) {
	state, err := core.LoadState(root, slug)
	if err != nil {
		return specdExit(err), false
	}
	if state == nil {
		return specdExit(core.NotFoundError(fmt.Sprintf("spec '%s' not found", slug))), false
	}
	if state.EffectiveMode() != core.ModeOrchestrated {
		core.Error(fmt.Sprintf("spec '%s' is in simple execution mode — Brain/Pinky will not drive it. Opt in first with `specd status %s --set-mode orchestrated`, or run it manually with `specd next %s`.", slug, slug, slug))
		return core.ExitGate, false
	}
	return core.ExitOK, true
}

func brainStart(root, slug string, args cli.Args) int {
	if code, ok := requireOrchestratedSpec(root, slug); !ok {
		return code
	}
	policy, cfg, ok := brainPolicyAndConfig(root, args)
	if !ok {
		return core.ExitUsage
	}
	sessionID, err := brainStartSessionID(args)
	if err != nil {
		return specdExit(err)
	}
	if _, err := core.StartOrchestrationSession(root, slug, sessionID, "brain-cli", policy); err != nil {
		return specdExit(err)
	}
	result, err := core.StepOrchestration(root, slug, sessionID, policy, cfg)
	if err != nil {
		return specdExit(err)
	}
	return printCommandResult(args, result)
}

func brainStep(root, slug, sessionID string, args cli.Args) int {
	if code, ok := requireOrchestratedSpec(root, slug); !ok {
		return code
	}
	policy, cfg, ok := brainPolicyAndConfig(root, args)
	if !ok {
		return core.ExitUsage
	}
	result, err := core.StepOrchestration(root, slug, sessionID, policy, cfg)
	if err != nil {
		return specdExit(err)
	}
	return printCommandResult(args, result)
}

func brainProgramStep(root, parentSessionID string, args cli.Args) int {
	policy, cfg, ok := brainPolicyAndConfig(root, args)
	if !ok {
		return core.ExitUsage
	}
	result, err := core.StepProgramOrchestration(root, parentSessionID, policy, cfg)
	if err != nil {
		return specdExit(err)
	}
	return printCommandResult(args, result)
}

// brainRun is the reference driver loop (GAP-2/GAP-6): a bare or partial repo is
// brought to a sensable spec (preflight), then driven step→spawn→step to a
// terminal outcome. Creative work is delegated to a host worker command
// (--worker-cmd); core stays deterministic. With no --worker-cmd the loop stops
// at the first dispatch and prints the mission to wire a worker manually.
func brainRun(root, slug string, args cli.Args) int {
	if code, ok := requireOrchestratedSpec(root, slug); !ok {
		return code
	}
	// Pre-spec preflight (GAP-6): ensure the spec exists before sensing.
	if items := core.OrchestrationPreflight(root, slug); len(items) > 0 {
		if !brainRunBootstrap(root, slug, args, items) {
			return core.ExitGate
		}
	}

	policy, cfg, ok := brainRunPolicy(root, args)
	if !ok {
		return core.ExitUsage
	}
	sessionID, resumed, err := brainRunSession(root, slug, args, policy)
	if err != nil {
		return specdExit(err)
	}
	if resumed && !args.Bool("json") {
		fmt.Printf("brain run: resuming active session %s\n", sessionID)
	}

	maxSteps := 100
	if args.Has("max-steps") {
		n, ok := parsePositiveIntFlag(args, "max-steps")
		if !ok {
			return core.ExitUsage
		}
		maxSteps = n
	}

	logger, closer := obs.NewSessionLogger(root, sessionID)
	if closer != nil {
		defer closer.Close()
	}
	opts := core.DriverOptions{MaxSteps: maxSteps, Worker: brainRunWorker(brainRunner, root, args.Str("worker-cmd"), logger), Observer: brainObserver(root, logger)}
	result, err := core.DriveOrchestration(root, slug, sessionID, policy, cfg, opts)
	if err != nil {
		return specdExit(err)
	}
	if args.Bool("json") {
		if err := core.PrintJSON(map[string]any{"session": sessionID, "result": result}); err != nil {
			return specdExit(err)
		}
	} else {
		fmt.Printf("brain run: %s after %d step(s) — final decision: %s (%s)\n", result.Outcome, result.Steps, result.Final.Action, result.Final.Reason)
	}
	if result.Outcome == core.DriverEscalated {
		return core.ExitGate
	}
	return core.ExitOK
}

// brainRunProgram is the program-scoped reference driver loop (GAP-7): one call
// drives a whole multi-spec program to a terminal outcome, re-resolving the
// frontier and advancing to the next spec automatically on child completion.
// Creative work is delegated to the host worker command (--worker-cmd); with no
// --worker-cmd the loop stops at the first child dispatch.
func brainRunProgram(root string, args cli.Args) int {
	policy, cfg, ok := brainRunPolicy(root, args)
	if !ok {
		return core.ExitUsage
	}
	parentID := args.Str("session")
	if parentID == "" {
		id, err := core.NewACPID()
		if err != nil {
			return specdExit(err)
		}
		parentID = id
	}

	maxSteps := 200
	if args.Has("max-steps") {
		n, ok := parsePositiveIntFlag(args, "max-steps")
		if !ok {
			return core.ExitUsage
		}
		maxSteps = n
	}

	return driveProgramAndReport(root, args, parentID, policy, cfg, maxSteps, "brain run --program")
}

// driveProgramAndReport runs the program driver from the current frontier and
// renders the outcome. It is shared by `brain run --program` and
// `brain resume --program` so both walk the cross-spec frontier identically.
func driveProgramAndReport(root string, args cli.Args, parentID string, policy core.OrchestrationPolicy, cfg core.OrchestrationCfg, maxSteps int, label string) int {
	logger, closer := obs.NewSessionLogger(root, parentID)
	if closer != nil {
		defer closer.Close()
	}
	opts := core.ProgramDriverOptions{MaxSteps: maxSteps, Worker: brainRunProgramWorker(brainRunner, root, args.Str("worker-cmd"), logger), Observer: brainObserver(root, logger)}
	result, err := core.DriveProgramOrchestration(root, parentID, policy, cfg, opts)
	if err != nil {
		return specdExit(err)
	}
	if args.Bool("json") {
		if err := core.PrintJSON(map[string]any{"session": parentID, "result": result}); err != nil {
			return specdExit(err)
		}
	} else {
		fmt.Printf("%s: %s after %d step(s) — final decision: %s (%s)\n", label, result.Outcome, result.Steps, result.Final.Action, result.Final.Reason)
	}
	if result.Outcome == core.DriverEscalated {
		return core.ExitGate
	}
	return core.ExitOK
}

// brainResumeProgram reconstructs an interrupted program's frontier from its
// persisted program-state.json and restarts the program driver from the current
// frontier (cross-spec recovery). Complete children are not re-dispatched (the
// program step releases them); children with a running worker rely on
// lease/checkpoint recovery. The drive is idempotent: a second call re-derives
// the same frontier and advances only pending/running children.
func brainResumeProgram(root string, args cli.Args) int {
	parentID := args.Str("session")
	if parentID == "" {
		return usageExit("usage: specd brain resume --program --session <id> --worker-cmd <cmd>")
	}
	policy, cfg, ok := brainRunPolicy(root, args)
	if !ok {
		return core.ExitUsage
	}

	session, err := core.LoadProgramSession(root, parentID)
	if err != nil {
		return specdExit(err)
	}
	switch session.Status {
	case core.OrchestrationSessionComplete, core.OrchestrationSessionFailed:
		fmt.Printf("brain resume --program: session already %s — nothing to resume\n", session.Status)
		return core.ExitOK
	case core.OrchestrationSessionPaused:
		// Unpause so the driver may dispatch again; a running program needs no flip.
		if _, err := core.ResumeProgramOrchestration(root, parentID); err != nil {
			return specdExit(err)
		}
	}

	// Surface the persisted frontier classification for the operator. Child
	// sessions stay authoritative — the driver re-derives the live frontier — so a
	// missing/old program-state is non-fatal here.
	if state, err := core.LoadProgramState(root, parentID); err == nil {
		fmt.Printf("brain resume --program: reconstructed frontier — %d/%d child spec(s) complete\n",
			state.CompleteChildCount(), len(state.ChildStatus))
	}

	maxSteps := 200
	if args.Has("max-steps") {
		n, ok := parsePositiveIntFlag(args, "max-steps")
		if !ok {
			return core.ExitUsage
		}
		maxSteps = n
	}
	return driveProgramAndReport(root, args, parentID, policy, cfg, maxSteps, "brain resume --program")
}

// brainRunSession resolves the session to drive: an explicit --session, an
// existing active session for the spec (resumed), or a freshly started one. This
// makes `brain run` safely re-runnable against the one-session-per-spec rule.
func brainRunSession(root, slug string, args cli.Args, policy core.OrchestrationPolicy) (string, bool, error) {
	if id := args.Str("session"); id != "" {
		// Caller named a session: start it if new, otherwise drive it as-is.
		if _, err := core.StartOrchestrationSession(root, slug, id, "brain-run", policy); err != nil {
			if existing, lookupErr := core.ActiveOrchestrationSessionForSpec(root, slug); lookupErr == nil && existing != nil && existing.SessionID == id {
				return id, true, nil
			}
			return "", false, err
		}
		return id, false, nil
	}
	if existing, err := core.ActiveOrchestrationSessionForSpec(root, slug); err == nil && existing != nil {
		return existing.SessionID, true, nil
	}
	id, err := core.NewACPID()
	if err != nil {
		return "", false, err
	}
	if _, err := core.StartOrchestrationSession(root, slug, id, "brain-run", policy); err != nil {
		return "", false, err
	}
	return id, false, nil
}
