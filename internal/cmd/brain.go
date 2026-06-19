package cmd

import (
	"fmt"
	"strconv"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func RunBrain(args cli.Args) int {
	if len(args.Pos) == 0 {
		return usageExit("usage: specd brain <start|status|step|pause|resume|cancel> ...")
	}
	root, ok := core.FindSpecdRoot(".")
	if !ok {
		return specdExit(core.NotFoundError("not in a specd workspace"))
	}

	switch args.Pos[0] {
	case "start":
		if len(args.Pos) != 2 {
			return usageExit("usage: specd brain start <slug> --approval-policy <policy> --max-workers <n> --max-retries <n> --timeout-seconds <n> [--cost-limit <usd>] [--json]")
		}
		return brainStep(root, args.Pos[1], "", args)
	case "step":
		if len(args.Pos) != 2 || args.Str("session") == "" {
			return usageExit("usage: specd brain step <slug> --session <id> --approval-policy <policy> --max-workers <n> --max-retries <n> --timeout-seconds <n> [--cost-limit <usd>] [--json]")
		}
		return brainStep(root, args.Pos[1], args.Str("session"), args)
	case "status":
		if len(args.Pos) != 1 || args.Str("session") == "" {
			return usageExit("usage: specd brain status --session <id> [--json]")
		}
		session, err := core.LoadOrchestrationSession(root, args.Str("session"))
		if err != nil {
			return specdExit(err)
		}
		return printCommandResult(args, session)
	case "pause":
		return brainSessionControl(root, args, core.PauseOrchestration, "pause")
	case "resume":
		return brainSessionControl(root, args, core.ResumeOrchestration, "resume")
	case "cancel":
		return brainSessionControl(root, args, core.CancelOrchestration, "cancel")
	default:
		return usageExit("usage: specd brain <start|status|step|pause|resume|cancel> ...")
	}
}

func brainStep(root, slug, sessionID string, args cli.Args) int {
	policy, ok := brainPolicy(args)
	if !ok {
		return core.ExitUsage
	}
	cfg := core.LoadConfig(root).Orchestration
	cfg.SessionTimeoutMinutes = policy.SessionTimeoutSeconds / 60
	cfg.ApprovalPolicy = policy.ApprovalPolicy
	cfg.MaxWorkers = policy.MaxWorkers
	cfg.MaxRetries = policy.MaxRetries
	cfg.HostReportedCostLimitUSD = policy.HostReportedCostLimitUSD

	result, err := core.StepOrchestration(root, slug, sessionID, policy, cfg)
	if err != nil {
		return specdExit(err)
	}
	return printCommandResult(args, result)
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
