package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/integration"
	"github.com/0xkhdr/specd/internal/mcp"
)

const doctorResultSchemaVersion = 1

type doctorCheck struct {
	Name        string `json:"name"`
	Status      string `json:"status"`
	Detail      string `json:"detail"`
	Remediation string `json:"remediation"`
}

type doctorHost struct {
	Name             string `json:"name"`
	Detected         bool   `json:"detected"`
	Registered       bool   `json:"registered"`
	Owned            bool   `json:"owned"`
	Status           string `json:"status"`
	Reason           string `json:"reason"`
	Remediation      string `json:"remediation"`
	LifecycleSupport string `json:"lifecycleSupport"`
	ReloadRequired   bool   `json:"reloadRequired"`
	TrustRequired    bool   `json:"trustRequired"`
}

type doctorOrchestration struct {
	ServerCapability string   `json:"serverCapability"`
	Tools            []string `json:"tools"`
	HostLifecycle    string   `json:"hostLifecycle"`
}

type doctorResult struct {
	SchemaVersion     int                     `json:"schemaVersion"`
	Status            string                  `json:"status"`
	Root              string                  `json:"root"`
	Checks            []doctorCheck           `json:"checks"`
	Hosts             []doctorHost            `json:"hosts"`
	Orchestration     doctorOrchestration     `json:"orchestration"`
	ConfigDiagnostics []core.ConfigDiagnostic `json:"configDiagnostics"`
	Remediations      []string                `json:"remediations"`
	NextAction        string                  `json:"nextAction"`
}

type doctorRuntime struct {
	Registry *integration.Registry
	Probe    func(context.Context, mcp.Dispatcher, time.Duration) (mcp.ProbeResult, error)
}

func runDoctorCmd(args cli.Args) int {
	return runDoctor(args, doctorRuntime{Registry: integration.DefaultRegistry(), Probe: mcp.Probe})
}

func runDoctor(args cli.Args, runtime doctorRuntime) int {
	root, err := os.Getwd()
	if err != nil {
		return specdExit(err)
	}
	runtime = defaultDoctorRuntime(runtime)
	result := newDoctorResult(root)
	inspectDoctorBinary(&result)
	inspectDoctorScaffold(&result, root, args.Bool("fix"))
	cfg := inspectDoctorConfig(&result, root)
	inspectSandboxAvailability(&result, cfg)
	inspectDoctorMCP(&result, runtime)
	if code := inspectDoctorHosts(&result, root, args, runtime); code != core.ExitOK {
		return code
	}
	normalizeDoctorResult(&result)
	if result.Status != "healthy" {
		result.NextAction = firstRemediation(result.Remediations)
	}
	return emitDoctorResult(result, args.Bool("json"))
}

func defaultDoctorRuntime(runtime doctorRuntime) doctorRuntime {
	if runtime.Registry == nil {
		runtime.Registry = integration.DefaultRegistry()
	}
	if runtime.Probe == nil {
		runtime.Probe = mcp.Probe
	}
	return runtime
}

func newDoctorResult(root string) doctorResult {
	return doctorResult{
		SchemaVersion: doctorResultSchemaVersion,
		Status:        "healthy",
		Root:          root,
		Checks:        []doctorCheck{},
		Hosts:         []doctorHost{},
		Orchestration: doctorOrchestration{
			ServerCapability: "unknown",
			Tools:            []string{},
			HostLifecycle:    "host-managed; specd exposes MCP tools but does not spawn, reload, or trust coding-agent hosts",
		},
		Remediations: []string{},
		NextAction:   "No action required.",
	}
}

func inspectDoctorBinary(result *doctorResult) {
	executable, execErr := os.Executable()
	if execErr != nil {
		addDoctorFailure(result, "binary", execErr.Error(), "reinstall specd and ensure it is on PATH")
		return
	}
	result.Checks = append(result.Checks, doctorCheck{Name: "binary", Status: "pass", Detail: executable})
}

func inspectDoctorScaffold(result *doctorResult, root string, fix bool) {
	missing, scaffoldErr := inspectScaffold(root)
	if scaffoldErr != nil {
		addDoctorFailure(result, "scaffold", scaffoldErr.Error(), "run `specd init --repair`")
		return
	}
	if len(missing) > 0 && fix {
		missing, scaffoldErr = repairDoctorScaffold(root)
		if scaffoldErr != nil {
			addDoctorFailure(result, "scaffold", scaffoldErr.Error(), "run `specd init --repair`")
			return
		}
	}
	if len(missing) > 0 {
		addDoctorFailure(result, "scaffold", "missing: "+strings.Join(missing, ", "), "run `specd doctor --fix` or `specd init --repair`")
		return
	}
	result.Checks = append(result.Checks, doctorCheck{Name: "scaffold", Status: "pass", Detail: "all required project assets are present"})
}

func repairDoctorScaffold(root string) ([]string, error) {
	options := core.InitOptions{Root: root, Repair: true, Scope: string(integration.ScopeProject)}
	plan, planErr := core.PlanInit(options, core.DefaultScaffoldManifest(), core.ReadTemplate)
	if planErr != nil {
		return nil, planErr
	}
	fixed := core.ExecuteInitPlan(plan, false, core.DefaultInitExecutor())
	if fixed.Status != "ready" {
		return nil, fmt.Errorf("repair failed: %v", fixed.Warnings)
	}
	return inspectScaffold(root)
}

func inspectDoctorConfig(result *doctorResult, root string) core.Config {
	cfg, configDiagnostics := core.LoadConfigStrict(root)
	result.ConfigDiagnostics = configDiagnostics
	if !core.HasErrorDiagnostics(configDiagnostics) {
		result.Checks = append(result.Checks, doctorCheck{Name: "config-policy", Status: "pass", Detail: "strict config validation passed"})
		return cfg
	}
	parts := []string{}
	for _, d := range configDiagnostics {
		if d.Severity == "error" {
			parts = append(parts, d.Path+": "+d.Message)
		}
	}
	addDoctorFailure(result, "config-policy", strings.Join(parts, "; "), "fix .specd/config.json, then rerun `specd fusion policy --json`")
	return cfg
}

func inspectDoctorMCP(result *doctorResult, runtime doctorRuntime) {
	probe, probeErr := runtime.Probe(context.Background(), nil, 2*time.Second)
	if probeErr != nil {
		result.Orchestration.ServerCapability = "unavailable"
		addDoctorFailure(result, "mcp", probeErr.Error(), "run `specd doctor` after rebuilding or reinstalling specd")
		return
	}
	result.Orchestration.ServerCapability = "available"
	result.Orchestration.Tools = append([]string(nil), probe.OrchestrationTools...)
	result.Checks = append(result.Checks, doctorCheck{
		Name:   "mcp",
		Status: "pass",
		Detail: fmt.Sprintf("protocol %s; %d tools; orchestration tools: %s", probe.ProtocolVersion, probe.ToolCount, strings.Join(probe.OrchestrationTools, ", ")),
	})
}

func inspectDoctorHosts(result *doctorResult, root string, args cli.Args, runtime doctorRuntime) int {
	detections := runtime.Registry.Detect(root)
	hostNames, selectionErr := doctorHostNames(args.Str("agent"), detections, runtime.Registry)
	if selectionErr != nil {
		return usageExit(selectionErr.Error())
	}
	for _, name := range hostNames {
		host := inspectDoctorHost(root, name, args.Bool("fix"), detections, runtime)
		if host.Status != "pass" {
			result.Status = "unhealthy"
			if host.Remediation != "" {
				result.Remediations = append(result.Remediations, host.Remediation)
			}
		}
		result.Hosts = append(result.Hosts, host)
	}
	return core.ExitOK
}

func inspectDoctorHost(root, name string, fix bool, detections []integration.Detection, runtime doctorRuntime) doctorHost {
	adapter, _ := runtime.Registry.Get(name)
	detection := detectionForHost(detections, name)
	host := doctorHost{Name: name, Detected: detection.Detected}
	host.LifecycleSupport, host.ReloadRequired, host.TrustRequired = doctorHostLifecycle(name)
	state, inspectErr := adapter.Inspect(root, integration.ScopeProject)
	if inspectErr != nil {
		host.Status = "fail"
		host.Reason = inspectErr.Error()
		host.Remediation = "repair the host project configuration, then run `specd doctor`"
		return host
	}
	if fix && detection.Detected && !state.Registered && state.Fingerprint == "" {
		state, inspectErr = repairDoctorHost(root, name, runtime, host)
	}
	if inspectErr != nil {
		host.Status = "fail"
		host.Reason = inspectErr.Error()
		return host
	}
	host.Registered = state.Registered
	host.Owned = state.Owned
	verification := adapter.Verify(root)
	host.Status = verification.Status
	host.Reason = verification.Reason
	host.Remediation = verification.Remedy
	return host
}

func repairDoctorHost(root, name string, runtime doctorRuntime, host doctorHost) (integration.HostState, error) {
	plan, planErr := runtime.Registry.Plan(name, root, integration.ScopeProject)
	if planErr == nil {
		_, planErr = runtime.Registry.Install(context.Background(), plan)
	}
	if planErr != nil {
		return integration.HostState{}, planErr
	}
	adapter, _ := runtime.Registry.Get(host.Name)
	return adapter.Inspect(root, integration.ScopeProject)
}

func inspectScaffold(root string) ([]string, error) {
	missing := []string{}
	for _, asset := range core.DefaultScaffoldManifest() {
		if !asset.Required {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(asset.Target))
		if _, err := os.Stat(target); os.IsNotExist(err) {
			missing = append(missing, asset.Target)
		} else if err != nil {
			return nil, err
		} else if asset.Policy == core.ScaffoldMarkerMerge {
			if _, err := core.ValidateAgentsMD(target); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(missing)
	return missing, nil
}

func doctorHostNames(requested string, detections []integration.Detection, registry *integration.Registry) ([]string, error) {
	switch requested {
	case "", "all":
		names := []string{}
		for _, detection := range detections {
			if detection.Detected {
				names = append(names, detection.Host)
			}
		}
		return names, nil
	default:
		if _, ok := registry.Get(requested); !ok {
			return nil, fmt.Errorf("unsupported host %q", requested)
		}
		return []string{requested}, nil
	}
}

func addDoctorFailure(result *doctorResult, name, detail, remediation string) {
	result.Status = "unhealthy"
	result.Checks = append(result.Checks, doctorCheck{Name: name, Status: "fail", Detail: detail, Remediation: remediation})
	result.Remediations = append(result.Remediations, remediation)
}

// addDoctorAdvisory records a non-fatal finding: unlike addDoctorFailure, it
// never flips result.Status to "unhealthy" or affects doctor's exit code —
// SelectRunner already fails closed at verify time, so doctor's job here is
// only to explain why ahead of time, not to gate on it (Requirement 3.2).
func addDoctorAdvisory(result *doctorResult, name, detail, remediation string) {
	result.Checks = append(result.Checks, doctorCheck{Name: name, Status: "advisory", Detail: detail, Remediation: remediation})
}

// inspectSandboxAvailability reports, as an advisory finding, why a
// configured verify.sandbox backend would refuse to run — without itself
// calling or otherwise touching runner.SelectRunner. The verify command
// remains the sole place that decides and fails closed (Requirement 3).
func inspectSandboxAvailability(result *doctorResult, cfg core.Config) {
	switch strings.TrimSpace(cfg.Verify.Sandbox) {
	case "bwrap":
		if _, err := exec.LookPath("bwrap"); err != nil {
			addDoctorAdvisory(result, "verify-sandbox",
				"verify.sandbox is \"bwrap\" but bubblewrap is not on PATH — `specd verify` will fail closed and refuse to run",
				sandboxInstallHint("bwrap"))
			return
		}
		result.Checks = append(result.Checks, doctorCheck{Name: "verify-sandbox", Status: "pass", Detail: "bubblewrap found on PATH"})
	case "container":
		_, dockerErr := exec.LookPath("docker")
		_, podmanErr := exec.LookPath("podman")
		if dockerErr != nil && podmanErr != nil {
			addDoctorAdvisory(result, "verify-sandbox",
				"verify.sandbox is \"container\" but neither docker nor podman is on PATH — `specd verify` will fail closed and refuse to run",
				sandboxInstallHint("container"))
			return
		}
		if strings.TrimSpace(os.Getenv("SPECD_SANDBOX_IMAGE")) == "" {
			addDoctorAdvisory(result, "verify-sandbox",
				"verify.sandbox is \"container\" but SPECD_SANDBOX_IMAGE is unset — `specd verify` will fail closed and refuse to run",
				"set SPECD_SANDBOX_IMAGE to a pinned image, e.g. \"golang:1.26\"")
			return
		}
		result.Checks = append(result.Checks, doctorCheck{Name: "verify-sandbox", Status: "pass", Detail: "container engine and SPECD_SANDBOX_IMAGE configured"})
	}
}

// sandboxInstallHint names an OS-appropriate install command for a missing
// sandbox dependency, dep being "bwrap" or "container".
func sandboxInstallHint(dep string) string {
	switch dep {
	case "bwrap":
		if runtime.GOOS == "darwin" {
			return "bubblewrap is Linux-only; set verify.sandbox to \"container\" or \"none\" on macOS"
		}
		return "install bubblewrap, e.g. `apt-get install bubblewrap` or `dnf install bubblewrap`"
	case "container":
		if runtime.GOOS == "darwin" {
			return "install a container engine, e.g. `brew install podman` or Docker Desktop"
		}
		return "install a container engine, e.g. `apt-get install podman` or `dnf install podman`"
	default:
		return ""
	}
}

func normalizeDoctorResult(result *doctorResult) {
	if result.Checks == nil {
		result.Checks = []doctorCheck{}
	}
	if result.Hosts == nil {
		result.Hosts = []doctorHost{}
	}
	if result.Remediations == nil {
		result.Remediations = []string{}
	}
	if result.Orchestration.Tools == nil {
		result.Orchestration.Tools = []string{}
	}
	sort.Slice(result.Checks, func(i, j int) bool { return result.Checks[i].Name < result.Checks[j].Name })
	sort.Slice(result.Hosts, func(i, j int) bool { return result.Hosts[i].Name < result.Hosts[j].Name })
	sort.Strings(result.Remediations)
	result.Remediations = compactStrings(result.Remediations)
}

func compactStrings(values []string) []string {
	out := values[:0]
	for _, value := range values {
		if value == "" || len(out) > 0 && out[len(out)-1] == value {
			continue
		}
		out = append(out, value)
	}
	return out
}

func firstRemediation(remediations []string) string {
	if len(remediations) == 0 {
		return "Review the failed doctor checks."
	}
	return remediations[0]
}

func doctorHostLifecycle(name string) (support string, reloadRequired, trustRequired bool) {
	support = "host-managed; specd registers project MCP config only and does not spawn Pinky agents"
	switch name {
	case "claude-code", "cursor", "antigravity":
		return support + "; reload or enable the MCP server in the host if tools are not visible", true, false
	case "vscode":
		return support + "; reload the workspace, approve workspace trust, and start the MCP server in VS Code", true, true
	case "codex":
		return support + "; merge or reload project MCP config according to Codex host support", true, false
	default:
		return support, false, false
	}
}

func emitDoctorResult(result doctorResult, jsonOut bool) int {
	normalizeDoctorResult(&result)
	if jsonOut || core.IsJSONMode() {
		if err := core.PrintJSON(result); err != nil {
			return specdExit(err)
		}
	} else {
		core.Info(fmt.Sprintf("specd doctor: %s", result.Status))
		for _, check := range result.Checks {
			core.Info(fmt.Sprintf("%s: %s — %s", check.Name, check.Status, check.Detail))
		}
		for _, host := range result.Hosts {
			core.Info(fmt.Sprintf("agent %s: %s — %s", host.Name, host.Status, host.Reason))
		}
		core.Info("Next: " + result.NextAction)
	}
	if result.Status != "healthy" {
		return core.ExitGate
	}
	return core.ExitOK
}
