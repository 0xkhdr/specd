package cmd

import (
	"fmt"
	"strconv"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

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
