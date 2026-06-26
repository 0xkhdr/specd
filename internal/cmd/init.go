package cmd

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/integration"
	"github.com/0xkhdr/specd/internal/mcp"
	"github.com/0xkhdr/specd/internal/pack"
	"github.com/0xkhdr/specd/internal/runner"
)

// listPacks renders the embedded built-in packs as text, or JSON under
// SPECD_JSON. It performs no filesystem writes.
func listPacks() int {
	packs, err := pack.BuiltinPacks()
	if err != nil {
		return specdExit(err)
	}
	if core.IsJSONMode() {
		type packView struct {
			Name        string `json:"name"`
			Version     string `json:"version"`
			Description string `json:"description"`
			Files       int    `json:"files"`
		}
		views := make([]packView, 0, len(packs))
		for _, p := range packs {
			views = append(views, packView{p.Name, p.Version, p.Description, len(p.Files)})
		}
		if err := core.PrintJSON(views); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	fmt.Printf("specd built-in packs (%d):\n", len(packs))
	for _, p := range packs {
		fmt.Printf("  %-12s v%-7s %s (%d file%s)\n", p.Name, p.Version, p.Description, len(p.Files), plural(len(p.Files)))
	}
	fmt.Println("\nApply with: specd init --pack <name>")
	return core.ExitOK
}

func plural(n int) string {
	if n == 1 {
		return ""
	}
	return "s"
}

// applyPack resolves and transactionally applies a pack into root. A bare name
// resolves to a built-in; an http(s) URL requires --sha256 (fail-closed). It
// writes nothing on any resolve/apply error.
func applyPack(root, ref string, args cli.Args) int {
	pk, err := pack.ResolvePack(ref, args.Str("sha256"))
	if err != nil {
		return specdExit(err)
	}
	res, err := pack.ApplyPack(root, pk, args.Bool("force"))
	if err != nil {
		return specdExit(err)
	}
	if core.IsJSONMode() {
		if err := core.PrintJSON(struct {
			Pack    string   `json:"pack"`
			Version string   `json:"version"`
			Written []string `json:"written"`
		}{pk.Name, pk.Version, res.Written}); err != nil {
			return specdExit(err)
		}
		return core.ExitOK
	}
	core.Info(fmt.Sprintf("specd init --pack %s (v%s): wrote %d file(s):", pk.Name, pk.Version, len(res.Written)))
	for _, w := range res.Written {
		core.Info("  + " + w)
	}
	return core.ExitOK
}

func RunInit(args cli.Args) int {
	return runInitWithRuntime(args, core.DefaultInitExecutor(), defaultOnboardingRuntime())
}

func runInit(args cli.Args, executor core.InitExecutor) int {
	return runInitWithRuntime(args, executor, defaultOnboardingRuntime())
}

type onboardingRuntime struct {
	Registry    *integration.Registry
	Probe       func(context.Context, mcp.Dispatcher, time.Duration) (mcp.ProbeResult, error)
	Input       io.Reader
	Interactive func() bool
}

func defaultOnboardingRuntime() onboardingRuntime {
	return onboardingRuntime{
		Registry: integration.DefaultRegistry(),
		Probe:    mcp.Probe,
		Input:    os.Stdin,
		Interactive: func() bool {
			in, inErr := os.Stdin.Stat()
			out, outErr := os.Stdout.Stat()
			return inErr == nil && outErr == nil &&
				isTerminalFile(os.Stdin, in) && isTerminalFile(os.Stdout, out)
		},
	}
}

func isTerminalFile(file *os.File, info os.FileInfo) bool {
	if info.Mode()&os.ModeCharDevice == 0 ||
		strings.EqualFold(info.Name(), filepath.Base(os.DevNull)) {
		return false
	}
	if target, err := os.Readlink(fmt.Sprintf("/proc/self/fd/%d", file.Fd())); err == nil {
		return filepath.Clean(target) != filepath.Clean(os.DevNull)
	}
	return true
}

func runInitWithRuntime(args cli.Args, executor core.InitExecutor, runtime onboardingRuntime) int {
	if args.Bool("list-packs") {
		return listPacks()
	}
	root, err := os.Getwd()
	if err != nil {
		core.Error(err.Error())
		return core.ExitGate
	}
	if ref := args.Str("pack"); ref != "" {
		if args.Bool("repair") || args.Bool("refresh") || args.Bool("dry-run") {
			return usageExit("--pack cannot be combined with --repair, --refresh, or --dry-run")
		}
		return applyPack(root, ref, args)
	}
	options := core.InitOptions{
		Root:           root,
		Force:          args.Bool("force"),
		Repair:         args.Bool("repair"),
		Refresh:        args.Bool("refresh"),
		DryRun:         args.Bool("dry-run"),
		AgentSelection: []string{},
		Scope:          args.Str("scope"),
	}
	if options.Scope == "" {
		options.Scope = string(integration.ScopeProject)
	}
	if options.Scope != string(integration.ScopeProject) && options.Scope != string(integration.ScopeGlobal) {
		return usageExit("--scope must be project or global")
	}
	if options.Scope == string(integration.ScopeGlobal) && args.Bool("non-interactive") && !args.Bool("yes") {
		return usageExit("global scope requires explicit consent with --yes")
	}
	selectionName := args.Str("agent")
	if selectionName == "" {
		selectionName = "auto"
	}
	options.AgentSelection = []string{selectionName}
	if err := core.ValidateInitOptions(options); err != nil {
		return usageExit(err.Error())
	}
	plan, err := core.PlanInit(options, core.DefaultScaffoldManifest(), core.ReadTemplate)
	if err != nil {
		result := core.NewInitResult(root)
		result.Status = "failed"
		result.Warnings = append(result.Warnings, core.InitWarning{
			Code:    "preflight-failed",
			Message: err.Error(),
		})
		result.Normalize()
		return emitInitResult(result, args.Bool("json"))
	}

	// Parse and apply orchestration settings
	var orchConfig *core.OrchestrationCfg
	var rolesSubagentMode *string
	var verifySandbox *string
	var initWarnings []core.InitWarning

	if args.Has("orchestration") {
		orchPolicy := args.Str("orchestration")
		if orchPolicy == "true" || orchPolicy == "" {
			orchPolicy = "planning"
		}
		validPolicies := []string{"manual", "planning", "session"}
		valid := false
		for _, p := range validPolicies {
			if p == orchPolicy {
				valid = true
				break
			}
		}
		if !valid {
			return usageExit(fmt.Sprintf("--orchestration: invalid policy %q, expected manual|planning|session", orchPolicy))
		}

		workers := 4
		if args.Has("orchestration-workers") {
			w, err := strconv.Atoi(args.Str("orchestration-workers"))
			if err != nil {
				return usageExit("invalid --orchestration-workers: must be an integer")
			}
			workers = clamp(w, 1, 64, "maxWorkers", &initWarnings)
		}

		retries := 2
		if args.Has("orchestration-retries") {
			r, err := strconv.Atoi(args.Str("orchestration-retries"))
			if err != nil {
				return usageExit("invalid --orchestration-retries: must be an integer")
			}
			retries = clamp(r, 0, 10, "maxRetries", &initWarnings)
		}

		timeout := 120
		if args.Has("orchestration-timeout") {
			tVal, err := strconv.Atoi(args.Str("orchestration-timeout"))
			if err != nil {
				return usageExit("invalid --orchestration-timeout: must be an integer")
			}
			timeout = clamp(tVal, 1, 1440, "sessionTimeoutMinutes", &initWarnings)
		}

		costLimit := 0.0
		if args.Has("orchestration-cost-limit") {
			cl, err := parseCostLimit(args.Str("orchestration-cost-limit"))
			if err != nil {
				return usageExit(err.Error())
			}
			costLimit = cl
		}

		mode := "delegate"
		if args.Has("orchestration-mode") {
			mode = args.Str("orchestration-mode")
			if mode != "inline" && mode != "delegate" {
				return usageExit("invalid --orchestration-mode: must be inline or delegate")
			}
		}

		sandbox := "none"
		if args.Has("orchestration-sandbox") {
			sandbox = args.Str("orchestration-sandbox")
			if sandbox != "none" && sandbox != "bwrap" && sandbox != "container" {
				return usageExit("invalid --orchestration-sandbox: must be none, bwrap, or container")
			}
			if _, err := runner.SelectRunner(sandbox); err != nil {
				return specdExit(err)
			}
		}

		if mode == "delegate" && selectionName == "none" {
			msg := "Delegate mode requires a compatible host agent. Use --orchestration-mode inline or install an agent."
			core.Warn(msg)
			initWarnings = append(initWarnings, core.InitWarning{
				Code:    "orchestration-agent-mismatch",
				Message: msg,
			})
		}

		orchConfig = &core.OrchestrationCfg{
			Enabled:                  true,
			ApprovalPolicy:           orchPolicy,
			WorkerMode:               "host",
			MaxWorkers:               workers,
			MaxRetries:               retries,
			SessionTimeoutMinutes:    timeout,
			HostReportedCostLimitUSD: costLimit,
			Transport: core.TransportCfg{
				Kind:               "file",
				PollIntervalMillis: 500,
				MessageTTLSeconds:  3600,
				LeaseSeconds:       120,
				HeartbeatSeconds:   30,
			},
			Program: core.ProgramCfg{
				MaxConcurrentSpecs: 2,
			},
		}
		rolesSubagentMode = &mode
		verifySandbox = &sandbox

		// Update target config action in plan.Actions
		configTarget := filepath.Join(options.Root, ".specd", "config.json")
		for i, action := range plan.Actions {
			if action.Target == configTarget {
				var cfg core.Config
				if _, statErr := os.Stat(configTarget); statErr == nil && !options.Force {
					cfg = core.LoadConfig(options.Root)
				} else {
					if err := json.Unmarshal([]byte(action.Content), &cfg); err != nil {
						cfg = core.DefaultConfig
					}
				}

				// Within this block these are always set above, so apply directly.
				cfg.Orchestration = *orchConfig
				cfg.Roles.SubagentMode = *rolesSubagentMode
				cfg.Verify.Sandbox = *verifySandbox

				newContent, err := json.MarshalIndent(cfg, "", "  ")
				if err != nil {
					return specdExit(err)
				}
				plan.Actions[i].Content = string(newContent) + "\n"
				if action.Kind == "skip" {
					plan.Actions[i].Kind = "write"
					plan.Actions[i].Description = "update orchestration config"
				}
				break
			}
		}
	}

	if runtime.Registry == nil {
		runtime.Registry = integration.DefaultRegistry()
	}
	detections := runtime.Registry.Detect(root)
	interactive := !args.Bool("non-interactive") && !args.Bool("yes") &&
		!args.Bool("json") && !core.IsJSONMode() && runtime.Interactive != nil && runtime.Interactive()
	selected, code := resolveInitSelection(selectionName, interactive, args.Bool("yes"), detections, runtime)
	if code != core.ExitOK {
		return code
	}
	for _, host := range selected.Selected {
		adapter, _ := runtime.Registry.Get(host)
		if adapter == nil {
			return usageExit("unsupported host " + host)
		}
		detection := detectionForHost(detections, host)
		if !detection.Detected {
			return usageExit(fmt.Sprintf("requested host %q is unavailable; install it or choose --agent none", host))
		}
		if _, err := runtime.Registry.Plan(host, root, integration.Scope(options.Scope)); err != nil {
			return usageExit(err.Error())
		}
	}
	if options.Scope == string(integration.ScopeGlobal) && len(selected.Selected) > 0 && !args.Bool("yes") {
		if !interactive || !confirm(runtime.Input, "Configure global MCP integration? [y/N] ") {
			return usageExit("global scope requires explicit consent")
		}
	}

	// Claude Code loads CLAUDE.md (not AGENTS.md), so when claude-code is the
	// selected/detected host, splice a managed-marker CLAUDE.md merge into the
	// plan after selection resolves — mirroring the post-plan config mutation
	// above. Non-Claude repos stay clean (R1.4).
	if hostSelected(selected.Selected, "claude-code") {
		action, code := claudeMDInitAction(options)
		if code != core.ExitOK {
			return code
		}
		plan.Actions = append(plan.Actions, action)
	}

	result := core.ExecuteInitPlan(plan, options.Force, executor)
	for _, detection := range detections {
		if detection.Detected {
			result.Agents.Detected = append(result.Agents.Detected, detection.Host)
		}
	}
	if result.Status != "ready" && result.Status != "planned" {
		return emitInitResult(result, args.Bool("json"))
	}
	result = configureInitAgents(result, selected, options, runtime)
	if result.Status == "ready" {
		probe, err := runtime.Probe(context.Background(), nil, 2*time.Second)
		if err != nil {
			result.Status = "failed"
			result.Verification.MCP = "fail"
			result.Warnings = append(result.Warnings, core.InitWarning{Code: "mcp-probe-failed", Message: err.Error()})
		} else {
			result.Verification = core.InitVerificationResult{
				MCP:             "pass",
				ProtocolVersion: probe.ProtocolVersion,
				ToolCount:       probe.ToolCount,
			}
		}
	}

	result.Warnings = append(result.Warnings, initWarnings...)
	if orchConfig != nil {
		if len(selected.Selected) > 0 {
			host := selected.Selected[0]
			agentDisplay := host
			switch host {
			case "claude-code":
				agentDisplay = "Claude Code"
			case "cursor":
				agentDisplay = "Cursor"
			}
			result.NextAction.Text = fmt.Sprintf("Restart %s to pick up MCP registration, then run: specd brain run <spec> --bootstrap --approval-policy %s", agentDisplay, orchConfig.ApprovalPolicy)
		} else {
			result.NextAction.Text = fmt.Sprintf("Restart your agent to pick up MCP registration, then run: specd brain run <spec> --bootstrap --approval-policy %s", orchConfig.ApprovalPolicy)
		}
	}

	return emitInitResult(result, args.Bool("json"), args.Bool("verbose"))
}

func resolveInitSelection(name string, interactive, yes bool, detections []integration.Detection, runtime onboardingRuntime) (integration.Selection, int) {
	selected, err := integration.SelectHosts(name, interactive, detections)
	if err != nil {
		return selected, usageExit(err.Error())
	}
	if selected.Ambiguous && interactive {
		names := append([]string{}, selected.Suggestions...)
		fmt.Printf("Coding agents detected: %s\n", strings.Join(names, ", "))
		fmt.Printf("Configure which agent (%s, all, skip)? ", strings.Join(names, "/"))
		answer, readErr := bufio.NewReader(runtime.Input).ReadString('\n')
		if readErr != nil && strings.TrimSpace(answer) == "" {
			return selected, usageExit("could not read coding-agent selection")
		}
		answer = strings.TrimSpace(answer)
		if answer == "skip" {
			answer = "none"
		}
		selected, err = integration.SelectHosts(answer, false, detections)
		if err != nil {
			return selected, usageExit(err.Error())
		}
	}
	if name == "auto" && !interactive && !yes {
		selected.Suggestions = append(selected.Suggestions, selected.Selected...)
		selected.Selected = []string{}
		selected.Reason = "non-interactive auto-detection requires --yes; no host selected"
	}
	return selected, core.ExitOK
}

func detectionForHost(detections []integration.Detection, host string) integration.Detection {
	for _, detection := range detections {
		if detection.Host == host {
			return detection
		}
	}
	return integration.Detection{Host: host}
}

func confirm(input io.Reader, prompt string) bool {
	fmt.Print(prompt)
	answer, _ := bufio.NewReader(input).ReadString('\n')
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "y" || answer == "yes"
}

func configureInitAgents(result core.InitResult, selected integration.Selection, options core.InitOptions, runtime onboardingRuntime) core.InitResult {
	result.Agents.Manual = append(result.Agents.Manual, selected.Suggestions...)
	if selected.Ambiguous || (len(selected.Selected) == 0 && len(selected.Suggestions) > 0) {
		result.Warnings = append(result.Warnings, core.InitWarning{
			Code:    "agent-selection-required",
			Message: selected.Reason + "; rerun with --agent <name> --yes",
		})
	}
	scope := integration.Scope(options.Scope)
	for _, host := range selected.Selected {
		plan, err := runtime.Registry.Plan(host, options.Root, scope)
		if err != nil {
			result.Status = "failed"
			result.Warnings = append(result.Warnings, core.InitWarning{Code: "agent-plan-failed", Message: host + ": " + err.Error()})
			continue
		}
		if options.DryRun {
			result.Agents.Manual = append(result.Agents.Manual, host)
			for _, action := range plan.Actions {
				result.Warnings = append(result.Warnings, core.InitWarning{
					Code:    "agent-dry-run",
					Message: host + ": " + action.Description,
				})
			}
			continue
		}
		hostResult, err := runtime.Registry.Install(context.Background(), plan)
		if err != nil {
			result.Status = "failed"
			result.Warnings = append(result.Warnings, core.InitWarning{Code: "agent-install-failed", Message: host + ": " + err.Error()})
			continue
		}
		switch hostResult.Status {
		case "configured", "existing":
			result.Agents.Configured = append(result.Agents.Configured, host)
		default:
			result.Agents.Manual = append(result.Agents.Manual, host)
		}
		for _, warning := range hostResult.Warnings {
			result.Warnings = append(result.Warnings, core.InitWarning{Code: "agent-warning", Message: host + ": " + warning})
		}
	}
	return result
}

func emitInitResult(result core.InitResult, jsonOut bool, verbose ...bool) int {
	result.Normalize()
	if jsonOut || core.IsJSONMode() {
		if err := core.PrintJSON(result); err != nil {
			return specdExit(err)
		}
	} else {
		ready := len(result.Files.Written) + len(result.Files.Updated) + len(result.Files.Skipped)
		switch result.Status {
		case "planned":
			core.Info(fmt.Sprintf("specd init %s dry run in %s", result.Mode, result.Root))
			for _, path := range result.Files.Written {
				core.Info("would write: " + path)
			}
			for _, path := range result.Files.Updated {
				core.Info("would update: " + path)
			}
			for _, path := range result.Files.Skipped {
				core.Info("would preserve: " + path)
			}
		case "ready":
			core.Info(fmt.Sprintf("Initialized specd in %s", result.Root))
			core.Info(fmt.Sprintf("Project assets: %d ready, 0 failed", ready))
			if len(result.Agents.Configured) > 0 {
				core.Info("Coding agents configured: " + strings.Join(result.Agents.Configured, ", "))
			} else if len(result.Agents.Detected) > 0 {
				core.Info("Coding agents detected: " + strings.Join(result.Agents.Detected, ", "))
			}
			core.Info(fmt.Sprintf("MCP verification: %s", result.Verification.MCP))
			core.Info("Next: " + result.NextAction.Text)
			if len(result.Files.Skipped) > 0 {
				core.Info(fmt.Sprintf("skipped %d existing file(s)", len(result.Files.Skipped)))
			}
			for _, warning := range result.Warnings {
				core.Warn(warning.Message)
			}
			if len(verbose) > 0 && verbose[0] {
				printInitPaths(result)
			}
		default:
			core.Error(fmt.Sprintf("specd init failed in %s", result.Root))
			core.Error(fmt.Sprintf("Project assets: %d ready, %d failed", ready, len(result.Files.Failed)))
			for _, path := range result.Files.Failed {
				core.Error("failed: " + path)
			}
			for _, warning := range result.Warnings {
				core.Error(warning.Message)
			}
		}
	}
	if result.Status != "ready" && result.Status != "planned" {
		return core.ExitGate
	}
	return core.ExitOK
}

func printInitPaths(result core.InitResult) {
	groups := []struct {
		label string
		paths []string
	}{
		{"wrote", result.Files.Written},
		{"updated", result.Files.Updated},
		{"preserved", result.Files.Skipped},
	}
	for _, group := range groups {
		sort.Strings(group.paths)
		for _, path := range group.paths {
			core.Info(group.label + ": " + path)
		}
	}
}

func clamp(val, min, max int, name string, warnings *[]core.InitWarning) int {
	if val < min {
		msg := fmt.Sprintf("orchestration.%s: %d outside [%d,%d] — using %d", name, val, min, max, min)
		*warnings = append(*warnings, core.InitWarning{Code: "orchestration-clamp", Message: msg})
		core.Warn(msg)
		return min
	}
	if val > max {
		msg := fmt.Sprintf("orchestration.%s: %d outside [%d,%d] — using %d", name, val, min, max, max)
		*warnings = append(*warnings, core.InitWarning{Code: "orchestration-clamp", Message: msg})
		core.Warn(msg)
		return max
	}
	return val
}

func parseCostLimit(s string) (float64, error) {
	if s == "" {
		return 0, nil
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid cost limit: must be a non-negative number")
	}
	if math.IsNaN(f) || math.IsInf(f, 0) || f < 0 {
		return 0, fmt.Errorf("invalid cost limit: must be a non-negative number")
	}
	return f, nil
}

// hostSelected reports whether host is among the resolved selected/detected hosts.
func hostSelected(hosts []string, host string) bool {
	for _, h := range hosts {
		if h == host {
			return true
		}
	}
	return false
}

// claudeMDInitAction builds the project-root CLAUDE.md merge action, mirroring
// PlanInit's ScaffoldMarkerMerge handling: --repair preserves an existing file,
// otherwise an idempotent managed-marker merge wraps @AGENTS.md so re-runs and
// user edits outside the markers are preserved (R1.1, R1.2, R1.3).
func claudeMDInitAction(options core.InitOptions) (core.InitAction, int) {
	content, err := core.ReadTemplate("CLAUDE.md")
	if err != nil {
		return core.InitAction{}, specdExit(fmt.Errorf("read CLAUDE.md template: %w", err))
	}
	target := filepath.Join(options.Root, "CLAUDE.md")
	action := core.InitAction{
		Target:   target,
		Required: false,
		Template: "CLAUDE.md",
		Content:  content,
	}
	_, statErr := os.Stat(target)
	if statErr != nil && !os.IsNotExist(statErr) {
		return core.InitAction{}, specdExit(fmt.Errorf("inspect %s: %w", target, statErr))
	}
	switch {
	case options.Repair && statErr == nil:
		action.Kind = "skip"
		action.Description = "preserve existing file during repair"
	default:
		if statErr == nil && !options.Force {
			if _, err := core.ValidateAgentsMD(target); err != nil {
				return core.InitAction{}, specdExit(fmt.Errorf("inspect %s: %w", target, err))
			}
		}
		action.Kind = "merge"
		action.Description = "merge managed marker section"
		action.Destructive = options.Force
	}
	return action, core.ExitOK
}
