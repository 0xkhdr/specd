package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunBrain(args cli.Args) int {
	if len(args.Pos) == 0 {
		return usageExit("usage: specd brain <start|run|status|step|why|directive|pause|resume|cancel> ...")
	}
	root, ok := core.FindSpecdRoot(".")
	if !ok {
		return specdExit(core.NotFoundError("not in a specd workspace"))
	}

	switch args.Pos[0] {
	case "start":
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
	case "pause":
		if args.Bool("program") {
			return brainProgramSessionControl(root, args, core.PauseProgramOrchestration, "pause")
		}
		return brainSessionControl(root, args, core.PauseOrchestration, "pause")
	case "resume":
		if args.Bool("program") {
			return brainProgramSessionControl(root, args, core.ResumeProgramOrchestration, "resume")
		}
		return brainSessionControl(root, args, core.ResumeOrchestration, "resume")
	case "cancel":
		if args.Bool("program") {
			return brainProgramSessionControl(root, args, core.CancelProgramOrchestration, "cancel")
		}
		return brainSessionControl(root, args, core.CancelOrchestration, "cancel")
	default:
		return usageExit("usage: specd brain <start|run|status|step|why|directive|pause|resume|cancel> ...")
	}
}

func brainStart(root, slug string, args cli.Args) int {
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

func brainDirective(root string, args cli.Args) int {
	if len(args.Pos) != 1 || args.Str("session") == "" || args.Str("worker") == "" || args.Str("spec") == "" || args.Str("task") == "" || args.Str("action") == "" || args.Str("reason") == "" {
		return usageExit("usage: specd brain directive --session <id> --worker <id> --spec <slug> --task <id> --attempt <n> --action <continue|retry|cancel|reassign|escalate> --reason <text> [--in-reply-to <message-id>] [--json]")
	}
	attempt, ok := parsePositiveIntFlag(args, "attempt")
	if !ok {
		return usageExit("specd brain directive: --attempt must be a positive integer")
	}
	cfg := core.LoadConfig(root).Orchestration
	event, err := core.RecordBrainDirective(root, core.BrainDirective{
		SessionID: args.Str("session"),
		WorkerID:  args.Str("worker"),
		Spec:      args.Str("spec"),
		TaskID:    args.Str("task"),
		Attempt:   attempt,
		Action:    args.Str("action"),
		Reason:    args.Str("reason"),
		InReplyTo: args.Str("in-reply-to"),
	}, cfg)
	if err != nil {
		return specdExit(err)
	}
	return printCommandResult(args, event)
}

func brainProgramStatus(root string, args cli.Args) int {
	cfg := core.LoadConfig(root).Orchestration
	report, err := core.SenseProgramOrchestration(root, args.Str("session"), cfg)
	if err != nil {
		return specdExit(err)
	}
	return printCommandResult(args, report)
}

// brainRun is the reference driver loop (GAP-2/GAP-6): a bare or partial repo is
// brought to a sensable spec (preflight), then driven step→spawn→step to a
// terminal outcome. Creative work is delegated to a host worker command
// (--worker-cmd); core stays deterministic. With no --worker-cmd the loop stops
// at the first dispatch and prints the mission to wire a worker manually.
func brainRun(root, slug string, args cli.Args) int {
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

	opts := core.DriverOptions{MaxSteps: maxSteps, Worker: brainRunWorker(root, args.Str("worker-cmd"))}
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
	if result.Outcome == core.DriverComplete {
		return core.ExitOK
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

	opts := core.ProgramDriverOptions{MaxSteps: maxSteps, Worker: brainRunProgramWorker(root, args.Str("worker-cmd"))}
	result, err := core.DriveProgramOrchestration(root, parentID, policy, cfg, opts)
	if err != nil {
		return specdExit(err)
	}
	if args.Bool("json") {
		if err := core.PrintJSON(map[string]any{"session": parentID, "result": result}); err != nil {
			return specdExit(err)
		}
	} else {
		fmt.Printf("brain run --program: %s after %d step(s) — final decision: %s (%s)\n", result.Outcome, result.Steps, result.Final.Action, result.Final.Reason)
	}
	return core.ExitOK
}

// brainRunProgramWorker adapts the single-spec worker callback to the program
// driver: each child dispatch reuses the same shell-out contract (mission via
// temp file + SPECD_* env). An empty workerCmd returns nil — the loop then stops
// at the first child dispatch.
func brainRunProgramWorker(root, workerCmd string) func(core.ProgramDriverDispatch) error {
	inner := brainRunWorker(root, workerCmd)
	if inner == nil {
		return nil
	}
	return func(d core.ProgramDriverDispatch) error {
		return inner(d.Dispatch)
	}
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

// brainRunBootstrap applies the deterministic preflight remedies it can
// (creating the spec); it never fabricates a workspace or steering. Returns true
// if the spec is now ready to drive.
func brainRunBootstrap(root, slug string, args cli.Args, items []core.PreflightItem) bool {
	for _, item := range items {
		if item.Kind == "spec" && args.Bool("bootstrap") {
			newFlags := map[string]string{}
			if title := args.Str("title"); title != "" {
				newFlags["title"] = title
			}
			if code := RunNew(cli.Args{Pos: []string{slug}, Flags: newFlags}); code != core.ExitOK {
				errLine("brain run: bootstrap `%s` failed", item.Remedy)
				return false
			}
			continue
		}
		errLine("brain run: blocked — %s. Fix with `%s`%s.", item.Message, item.Remedy, bootstrapHint(item))
		return false
	}
	return true
}

func bootstrapHint(item core.PreflightItem) string {
	if item.Kind == "spec" {
		return " (or pass --bootstrap)"
	}
	return ""
}

// brainRunWorker returns a DriveOrchestration worker callback that shells out to
// workerCmd per dispatch, passing the mission via a temp file and SPECD_* env.
// An empty workerCmd returns nil — the loop then stops at the first dispatch.
func brainRunWorker(root, workerCmd string) func(core.DriverDispatch) error {
	if workerCmd == "" {
		return nil
	}
	return func(d core.DriverDispatch) error {
		raw, err := json.MarshalIndent(d.Mission, "", "  ")
		if err != nil {
			return err
		}
		f, err := os.CreateTemp("", "specd-mission-*.json")
		if err != nil {
			return err
		}
		defer os.Remove(f.Name())
		if _, err := f.Write(raw); err != nil {
			f.Close()
			return err
		}
		f.Close()

		cmd := exec.Command("sh", "-c", workerCmd)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		cmd.Env = append(os.Environ(),
			"SPECD_MISSION="+f.Name(),
			"SPECD_SESSION="+d.Mission.SessionID,
			"SPECD_WORKER="+d.Mission.WorkerID,
			"SPECD_SPEC="+d.Mission.Spec,
			"SPECD_TASK="+d.Mission.TaskID,
			"SPECD_ROLE="+d.Mission.Role,
			"SPECD_ARTIFACT="+filepath.Base(strings.Join(d.Mission.Files, " ")),
		)
		return cmd.Run()
	}
}

// brainRunPolicy builds a policy with sane defaults (planning autonomy) so a
// drive needs no flag plumbing; any provided brain flag overrides a default.
func brainRunPolicy(root string, args cli.Args) (core.OrchestrationPolicy, core.OrchestrationCfg, bool) {
	policy, err := core.NewOrchestrationPolicy(core.LoadConfig(root).Orchestration)
	if err != nil {
		errLine("brain run: %v", err)
		return core.OrchestrationPolicy{}, core.OrchestrationCfg{}, false
	}
	policy.ApprovalPolicy = "planning"
	if v := args.Str("approval-policy"); v != "" {
		policy.ApprovalPolicy = v
	}
	if args.Has("max-workers") {
		if n, ok := parsePositiveIntFlag(args, "max-workers"); ok {
			policy.MaxWorkers = n
		} else {
			return core.OrchestrationPolicy{}, core.OrchestrationCfg{}, false
		}
	}
	if args.Has("max-retries") {
		if n, ok := parseNonNegativeIntFlag(args, "max-retries"); ok {
			policy.MaxRetries = n
		} else {
			return core.OrchestrationPolicy{}, core.OrchestrationCfg{}, false
		}
	}
	if err := core.ValidateOrchestrationPolicy(policy); err != nil {
		errLine("brain run: %v", err)
		return core.OrchestrationPolicy{}, core.OrchestrationCfg{}, false
	}
	cfg := core.LoadConfig(root).Orchestration
	cfg.SessionTimeoutMinutes = policy.SessionTimeoutSeconds / 60
	cfg.ApprovalPolicy = policy.ApprovalPolicy
	cfg.MaxWorkers = policy.MaxWorkers
	cfg.MaxRetries = policy.MaxRetries
	cfg.HostReportedCostLimitUSD = policy.HostReportedCostLimitUSD
	return policy, cfg, true
}

func brainPolicyAndConfig(root string, args cli.Args) (core.OrchestrationPolicy, core.OrchestrationCfg, bool) {
	policy, ok := brainPolicy(args)
	if !ok {
		return core.OrchestrationPolicy{}, core.OrchestrationCfg{}, false
	}
	cfg := core.LoadConfig(root).Orchestration
	cfg.SessionTimeoutMinutes = policy.SessionTimeoutSeconds / 60
	cfg.ApprovalPolicy = policy.ApprovalPolicy
	cfg.MaxWorkers = policy.MaxWorkers
	cfg.MaxRetries = policy.MaxRetries
	cfg.HostReportedCostLimitUSD = policy.HostReportedCostLimitUSD
	return policy, cfg, true
}

func brainStartSessionID(args cli.Args) (string, error) {
	if args.Str("session") != "" {
		return args.Str("session"), nil
	}
	id, err := core.NewACPID()
	if err != nil {
		return "", err
	}
	return id, nil
}

func brainPolicy(args cli.Args) (core.OrchestrationPolicy, bool) {
	required := []string{"approval-policy", "max-workers", "max-retries", "timeout-seconds"}
	for _, key := range required {
		if args.Str(key) == "" {
			fmt.Printf("missing --%s\n", key)
			return core.OrchestrationPolicy{}, false
		}
	}
	maxWorkers, ok := parsePositiveIntFlag(args, "max-workers")
	if !ok {
		return core.OrchestrationPolicy{}, false
	}
	maxRetries, ok := parseNonNegativeIntFlag(args, "max-retries")
	if !ok {
		return core.OrchestrationPolicy{}, false
	}
	timeout, ok := parsePositiveIntFlag(args, "timeout-seconds")
	if !ok {
		return core.OrchestrationPolicy{}, false
	}
	cost := 0.0
	if args.Str("cost-limit") != "" {
		parsed, err := strconv.ParseFloat(args.Str("cost-limit"), 64)
		if err != nil || parsed < 0 {
			fmt.Println("--cost-limit must be a non-negative number")
			return core.OrchestrationPolicy{}, false
		}
		cost = parsed
	}
	return core.OrchestrationPolicy{
		ApprovalPolicy:           args.Str("approval-policy"),
		MaxWorkers:               maxWorkers,
		MaxRetries:               maxRetries,
		SessionTimeoutSeconds:    timeout,
		HostReportedCostLimitUSD: cost,
	}, true
}

func brainSessionControl(root string, args cli.Args, fn func(string, string) (core.OrchestrationSession, error), verb string) int {
	if len(args.Pos) != 1 || args.Str("session") == "" {
		return usageExit(fmt.Sprintf("usage: specd brain %s --session <id> [--json]", verb))
	}
	session, err := fn(root, args.Str("session"))
	if err != nil {
		return specdExit(err)
	}
	return printCommandResult(args, session)
}

func brainWhy(root string, args cli.Args) int {
	sessionID := args.Str("session")
	if sessionID == "" {
		return usageExit("usage: specd brain why --session <id> [--json]")
	}
	store, err := core.NewACPStore(root)
	if err != nil {
		return specdExit(err)
	}
	events, err := store.ReplaySessionEvents(sessionID)
	if err != nil {
		return specdExit(err)
	}
	event, ok := core.ExplainCurrentSessionDecision(events)
	if !ok {
		return specdExit(core.NotFoundError(fmt.Sprintf("no events for session %q", sessionID)))
	}
	if args.Bool("json") || core.IsJSONMode() {
		return printCommandResult(args, event)
	}
	fmt.Printf("brain why — %s\n", sessionID)
	fmt.Printf(" %s\n", core.FormatSessionTimelineEvent(event))
	return core.ExitOK
}

func brainProgramSessionControl(root string, args cli.Args, fn func(string, string) (core.ProgramSession, error), verb string) int {
	if len(args.Pos) != 1 || args.Str("session") == "" {
		return usageExit(fmt.Sprintf("usage: specd brain %s --program --session <id> [--json]", verb))
	}
	session, err := fn(root, args.Str("session"))
	if err != nil {
		return specdExit(err)
	}
	return printCommandResult(args, session)
}

func parsePositiveIntFlag(args cli.Args, key string) (int, bool) {
	n, err := strconv.Atoi(args.Str(key))
	if err != nil || n <= 0 {
		fmt.Printf("--%s must be a positive integer\n", key)
		return 0, false
	}
	return n, true
}

func parseNonNegativeIntFlag(args cli.Args, key string) (int, bool) {
	n, err := strconv.Atoi(args.Str(key))
	if err != nil || n < 0 {
		fmt.Printf("--%s must be a non-negative integer\n", key)
		return 0, false
	}
	return n, true
}

func printCommandResult(args cli.Args, v any) int {
	if args.Bool("json") {
		core.PrintJSON(v)
		return core.ExitOK
	}
	fmt.Printf("%+v\n", v)
	return core.ExitOK
}
