package cmd

import (
	"bufio"
	"context"
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
// resolves to a built-in, or — when --registry <git-url> is supplied and the
// name is not built-in — via the git-repo pack registry with a checksum lockfile
// (V11/P6.3). An http(s) URL requires --sha256 (fail-closed). It writes nothing
// on any resolve/apply/lock error.
func applyPack(root, ref string, args cli.Args) int {
	pk, code, ok := resolvePackForInit(root, ref, args)
	if !ok {
		return code
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

// resolvePackForInit resolves a --pack reference, routing a named pack through
// the git-repo registry when --registry is supplied (pinning the resolved digest
// into .specd/pack.lock), and otherwise through the built-in/remote resolver. It
// returns (pack, exitCode, ok); on any error ok is false and exitCode is the
// mapped specd exit.
func resolvePackForInit(root, ref string, args cli.Args) (*pack.Pack, int, bool) {
	registry := strings.TrimSpace(args.Str("registry"))
	isHTTP := strings.HasPrefix(ref, "http://") || strings.HasPrefix(ref, "https://")
	if registry != "" && !isHTTP {
		pk, entry, err := pack.ResolveFromRegistry(ref, registry)
		if err != nil {
			return nil, specdExit(err), false
		}
		lock, err := pack.LoadPackLock(root)
		if err != nil {
			return nil, specdExit(err), false
		}
		if err := lock.CheckAndPin(entry.Name, entry.SHA256); err != nil {
			return nil, specdExit(err), false
		}
		if err := lock.Save(root); err != nil {
			return nil, specdExit(err), false
		}
		return pk, core.ExitOK, true
	}
	pk, err := pack.ResolvePack(ref, args.Str("sha256"))
	if err != nil {
		return nil, specdExit(err), false
	}
	return pk, core.ExitOK, true
}

// RunInit implements `specd init`. It runs the workspace onboarding flow
// (scaffolding .specd, applying packs, probing/registering MCP integrations)
// via the default init executor and onboarding runtime.
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
	options, selectionName, code := initOptionsFromArgs(root, args)
	if code != core.ExitOK {
		return code
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

	orchConfig, initWarnings, code := applyInitOrchestrationArgs(args, selectionName, options, &plan)
	if code != core.ExitOK {
		return code
	}

	selected, detections, code := selectInitAgents(root, selectionName, options, args, &runtime)
	if code != core.ExitOK {
		return code
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

	result, terminal := executeInitPlanWithAgents(plan, options, executor, selected, detections, runtime, args.Bool("json"))
	if terminal {
		return emitInitResult(result, args.Bool("json"))
	}
	result.Warnings = append(result.Warnings, initWarnings...)
	if args.Bool("guardrails") {
		created, err := core.EnsureGuardrailsScaffold(plan.Root)
		if err != nil {
			result.Status = "failed"
			result.Warnings = append(result.Warnings, core.InitWarning{Code: "guardrails-failed", Message: err.Error()})
			result.Normalize()
			return emitInitResult(result, args.Bool("json"))
		}
		if created {
			result.Files.Written = append(result.Files.Written, ".specd/guardrails.json")
		} else {
			result.Files.Skipped = append(result.Files.Skipped, ".specd/guardrails.json")
		}
	}
	applyInitOrchestrationNextAction(&result, orchConfig, selected)

	return emitInitResult(result, args.Bool("json"), args.Bool("verbose"))
}

func initOptionsFromArgs(root string, args cli.Args) (core.InitOptions, string, int) {
	selectionName := args.Str("agent")
	if selectionName == "" {
		selectionName = "auto"
	}
	options := core.InitOptions{
		Root:           root,
		Force:          args.Bool("force"),
		Repair:         args.Bool("repair"),
		Refresh:        args.Bool("refresh"),
		DryRun:         args.Bool("dry-run"),
		AgentSelection: []string{selectionName},
		Scope:          args.Str("scope"),
	}
	if options.Scope == "" {
		options.Scope = string(integration.ScopeProject)
	}
	if options.Scope != string(integration.ScopeProject) && options.Scope != string(integration.ScopeGlobal) {
		return core.InitOptions{}, "", usageExit("--scope must be project or global")
	}
	if options.Scope == string(integration.ScopeGlobal) && args.Bool("non-interactive") && !args.Bool("yes") {
		return core.InitOptions{}, "", usageExit("global scope requires explicit consent with --yes")
	}
	if err := core.ValidateInitOptions(options); err != nil {
		return core.InitOptions{}, "", usageExit(err.Error())
	}
	return options, selectionName, core.ExitOK
}

func selectInitAgents(root, selectionName string, options core.InitOptions, args cli.Args, runtime *onboardingRuntime) (integration.Selection, []integration.Detection, int) {
	if runtime.Registry == nil {
		runtime.Registry = integration.DefaultRegistry()
	}
	detections := runtime.Registry.Detect(root)
	interactive := initInteractive(args, *runtime)
	selected, code := resolveInitSelection(selectionName, interactive, args.Bool("yes"), detections, *runtime)
	if code != core.ExitOK {
		return selected, detections, code
	}
	if code := validateSelectedInitAgents(root, options, selected, detections, *runtime); code != core.ExitOK {
		return selected, detections, code
	}
	if code := confirmGlobalInit(options, selected, args, interactive, runtime.Input); code != core.ExitOK {
		return selected, detections, code
	}
	return selected, detections, core.ExitOK
}

func initInteractive(args cli.Args, runtime onboardingRuntime) bool {
	return !args.Bool("non-interactive") && !args.Bool("yes") && !args.Bool("json") && !core.IsJSONMode() && runtime.Interactive != nil && runtime.Interactive()
}

func validateSelectedInitAgents(root string, options core.InitOptions, selected integration.Selection, detections []integration.Detection, runtime onboardingRuntime) int {
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
	return core.ExitOK
}

func confirmGlobalInit(options core.InitOptions, selected integration.Selection, args cli.Args, interactive bool, input io.Reader) int {
	if options.Scope != string(integration.ScopeGlobal) || len(selected.Selected) == 0 || args.Bool("yes") {
		return core.ExitOK
	}
	if !interactive || !confirm(input, "Configure global MCP integration? [y/N] ") {
		return usageExit("global scope requires explicit consent")
	}
	return core.ExitOK
}

func executeInitPlanWithAgents(plan core.InitPlan, options core.InitOptions, executor core.InitExecutor, selected integration.Selection, detections []integration.Detection, runtime onboardingRuntime, jsonOut bool) (core.InitResult, bool) {
	result := core.ExecuteInitPlan(plan, options.Force, executor)
	result.Warnings = append(result.Warnings, projectConfigDeprecationWarnings(options.Root)...)
	addGlobalConfigWarning(&result, options)
	addDetectedInitAgents(&result, detections)
	if result.Status != "ready" && result.Status != "planned" {
		return result, true
	}
	result = configureInitAgents(result, selected, options, runtime)
	probeInitMCP(&result, runtime)
	return result, false
}

func addGlobalConfigWarning(result *core.InitResult, options core.InitOptions) {
	if result.Status != "ready" || options.DryRun {
		return
	}
	global, err := core.EnsureGlobalConfigScaffold(core.ReadTemplate)
	if err != nil {
		result.Warnings = append(result.Warnings, core.InitWarning{Code: "global-config-warning", Message: "global config was not created: " + err.Error()})
		return
	}
	if global.Created {
		result.Warnings = append(result.Warnings, core.InitWarning{Code: "global-config-created", Message: "created global config: " + global.Path})
	}
}

func addDetectedInitAgents(result *core.InitResult, detections []integration.Detection) {
	for _, detection := range detections {
		if detection.Detected {
			result.Agents.Detected = append(result.Agents.Detected, detection.Host)
		}
	}
}

func probeInitMCP(result *core.InitResult, runtime onboardingRuntime) {
	if result.Status != "ready" {
		return
	}
	probe, err := runtime.Probe(context.Background(), nil, 2*time.Second)
	if err != nil {
		result.Status = "failed"
		result.Verification.MCP = "fail"
		result.Warnings = append(result.Warnings, core.InitWarning{Code: "mcp-probe-failed", Message: err.Error()})
		return
	}
	result.Verification = core.InitVerificationResult{MCP: "pass", ProtocolVersion: probe.ProtocolVersion, ToolCount: probe.ToolCount}
}

func applyInitOrchestrationNextAction(result *core.InitResult, orchConfig *core.OrchestrationCfg, selected integration.Selection) {
	if orchConfig == nil {
		return
	}
	if len(selected.Selected) == 0 {
		result.NextAction.Text = fmt.Sprintf("Restart your agent to pick up MCP registration, then run: specd brain run <spec> --bootstrap --approval-policy %s", orchConfig.ApprovalPolicy)
		return
	}
	agentDisplay := initAgentDisplayName(selected.Selected[0])
	result.NextAction.Text = fmt.Sprintf("Restart %s to pick up MCP registration, then run: specd brain run <spec> --bootstrap --approval-policy %s", agentDisplay, orchConfig.ApprovalPolicy)
}

func initAgentDisplayName(host string) string {
	switch host {
	case "claude-code":
		return "Claude Code"
	case "cursor":
		return "Cursor"
	default:
		return host
	}
}

func applyInitOrchestrationArgs(args cli.Args, selectionName string, options core.InitOptions, plan *core.InitPlan) (*core.OrchestrationCfg, []core.InitWarning, int) {
	if !args.Has("orchestration") {
		return nil, nil, core.ExitOK
	}
	parsed, warnings, code := parseInitOrchestrationArgs(args)
	if code != core.ExitOK {
		return nil, warnings, code
	}
	if parsed.mode == "delegate" && selectionName == "none" {
		msg := "Delegate mode requires a compatible host agent. Use --orchestration-mode inline or install an agent."
		core.Warn(msg)
		warnings = append(warnings, core.InitWarning{Code: "orchestration-agent-mismatch", Message: msg})
	}
	applyInitOrchestrationConfig(options, plan, parsed)
	return parsed.config, warnings, core.ExitOK
}

type parsedInitOrchestration struct {
	config  *core.OrchestrationCfg
	mode    string
	sandbox string
}

func parseInitOrchestrationArgs(args cli.Args) (parsedInitOrchestration, []core.InitWarning, int) {
	warnings := []core.InitWarning{}
	policy, code := parseOrchestrationPolicy(args.Str("orchestration"))
	if code != core.ExitOK {
		return parsedInitOrchestration{}, warnings, code
	}
	workers, code := parseClampedInitInt(args, "orchestration-workers", 4, 1, 64, "maxWorkers", &warnings)
	if code != core.ExitOK {
		return parsedInitOrchestration{}, warnings, code
	}
	retries, code := parseClampedInitInt(args, "orchestration-retries", 2, 0, 10, "maxRetries", &warnings)
	if code != core.ExitOK {
		return parsedInitOrchestration{}, warnings, code
	}
	timeout, code := parseClampedInitInt(args, "orchestration-timeout", 120, 1, 1440, "sessionTimeoutMinutes", &warnings)
	if code != core.ExitOK {
		return parsedInitOrchestration{}, warnings, code
	}
	costLimit, code := parseInitCostLimit(args)
	if code != core.ExitOK {
		return parsedInitOrchestration{}, warnings, code
	}
	mode, code := parseInitMode(args)
	if code != core.ExitOK {
		return parsedInitOrchestration{}, warnings, code
	}
	sandbox, code := parseInitSandbox(args)
	if code != core.ExitOK {
		return parsedInitOrchestration{}, warnings, code
	}
	return parsedInitOrchestration{config: buildInitOrchestrationConfig(policy, workers, retries, timeout, costLimit), mode: mode, sandbox: sandbox}, warnings, core.ExitOK
}

func parseOrchestrationPolicy(policy string) (string, int) {
	if policy == "true" || policy == "" {
		policy = "planning"
	}
	switch policy {
	case "manual", "planning", "session":
		return policy, core.ExitOK
	default:
		return "", usageExit(fmt.Sprintf("--orchestration: invalid policy %q, expected manual|planning|session", policy))
	}
}

func parseClampedInitInt(args cli.Args, flag string, defaultValue, min, max int, name string, warnings *[]core.InitWarning) (int, int) {
	if !args.Has(flag) {
		return defaultValue, core.ExitOK
	}
	value, err := strconv.Atoi(args.Str(flag))
	if err != nil {
		return 0, usageExit("invalid --" + flag + ": must be an integer")
	}
	return clamp(value, min, max, name, warnings), core.ExitOK
}

func parseInitCostLimit(args cli.Args) (float64, int) {
	if !args.Has("orchestration-cost-limit") {
		return 0, core.ExitOK
	}
	costLimit, err := parseCostLimit(args.Str("orchestration-cost-limit"))
	if err != nil {
		return 0, usageExit(err.Error())
	}
	return costLimit, core.ExitOK
}

func parseInitMode(args cli.Args) (string, int) {
	mode := "delegate"
	if args.Has("orchestration-mode") {
		mode = args.Str("orchestration-mode")
	}
	if mode != "inline" && mode != "delegate" {
		return "", usageExit("invalid --orchestration-mode: must be inline or delegate")
	}
	return mode, core.ExitOK
}

func parseInitSandbox(args cli.Args) (string, int) {
	sandbox := "none"
	if args.Has("orchestration-sandbox") {
		sandbox = args.Str("orchestration-sandbox")
	}
	if sandbox != "none" && sandbox != "bwrap" && sandbox != "container" {
		return "", usageExit("invalid --orchestration-sandbox: must be none, bwrap, or container")
	}
	if _, err := runner.SelectRunner(sandbox); err != nil {
		return "", specdExit(err)
	}
	return sandbox, core.ExitOK
}

func buildInitOrchestrationConfig(policy string, workers, retries, timeout int, costLimit float64) *core.OrchestrationCfg {
	return &core.OrchestrationCfg{
		Enabled:                  true,
		ApprovalPolicy:           policy,
		WorkerMode:               "host",
		MaxWorkers:               workers,
		MaxRetries:               retries,
		SessionTimeoutMinutes:    timeout,
		HostReportedCostLimitUSD: costLimit,
		Transport:                core.TransportCfg{Kind: "file", PollIntervalMillis: 500, MessageTTLSeconds: 3600, LeaseSeconds: 120, HeartbeatSeconds: 30},
		Program:                  core.ProgramCfg{MaxConcurrentSpecs: 2},
	}
}

func applyInitOrchestrationConfig(options core.InitOptions, plan *core.InitPlan, parsed parsedInitOrchestration) {
	configTarget := filepath.Join(options.Root, ".specd", "config.yml")
	for i, action := range plan.Actions {
		if action.Target != configTarget {
			continue
		}
		cfg := core.LoadConfig(options.Root)
		if _, statErr := os.Stat(configTarget); os.IsNotExist(statErr) || options.Force {
			cfg = core.DefaultConfig
			cfg.Version = 2
		}
		cfg.Orchestration = *parsed.config
		cfg.Roles.SubagentMode = parsed.mode
		cfg.Verify.Sandbox = parsed.sandbox
		plan.Actions[i].Content = core.RenderConfigYAML(cfg)
		if action.Kind == "skip" {
			plan.Actions[i].Kind = "write"
			plan.Actions[i].Description = "update orchestration config"
		}
		return
	}
}

func projectConfigDeprecationWarnings(root string) []core.InitWarning {
	yml := filepath.Join(root, ".specd", "config.yml")
	jsonPath := filepath.Join(root, ".specd", "config.json")
	_, ymlErr := os.Stat(yml)
	_, jsonErr := os.Stat(jsonPath)
	jsonExists := jsonErr == nil
	ymlExists := ymlErr == nil
	if jsonExists && !ymlExists {
		return []core.InitWarning{{Code: "legacy-config-deprecated", Message: "config.json is deprecated; convert it to config.yml manually (see docs/user-guide.md) or continue using JSON."}}
	}
	if jsonExists && ymlExists {
		return []core.InitWarning{{Code: "legacy-config-ignored", Message: "config.yml is active; config.json is ignored and deprecated."}}
	}
	return nil
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
