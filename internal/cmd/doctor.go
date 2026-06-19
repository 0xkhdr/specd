package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
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
	SchemaVersion int                 `json:"schemaVersion"`
	Status        string              `json:"status"`
	Root          string              `json:"root"`
	Checks        []doctorCheck       `json:"checks"`
	Hosts         []doctorHost        `json:"hosts"`
	Orchestration doctorOrchestration `json:"orchestration"`
	Remediations  []string            `json:"remediations"`
	NextAction    string              `json:"nextAction"`
}

type doctorRuntime struct {
	Registry *integration.Registry
	Probe    func(context.Context, mcp.Dispatcher, time.Duration) (mcp.ProbeResult, error)
}

func RunDoctor(args cli.Args) int {
	return runDoctor(args, doctorRuntime{Registry: integration.DefaultRegistry(), Probe: mcp.Probe})
}

func runDoctor(args cli.Args, runtime doctorRuntime) int {
	root, err := os.Getwd()
	if err != nil {
		return specdExit(err)
	}
	if runtime.Registry == nil {
		runtime.Registry = integration.DefaultRegistry()
	}
	if runtime.Probe == nil {
		runtime.Probe = mcp.Probe
	}

	result := doctorResult{
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

	executable, execErr := os.Executable()
	if execErr != nil {
		addDoctorFailure(&result, "binary", execErr.Error(), "reinstall specd and ensure it is on PATH")
	} else {
		result.Checks = append(result.Checks, doctorCheck{Name: "binary", Status: "pass", Detail: executable})
	}

	missing, scaffoldErr := inspectScaffold(root)
	if scaffoldErr != nil {
		addDoctorFailure(&result, "scaffold", scaffoldErr.Error(), "run `specd init --repair`")
	} else if len(missing) > 0 {
		if args.Bool("fix") {
			options := core.InitOptions{Root: root, Repair: true, Scope: string(integration.ScopeProject)}
			plan, planErr := core.PlanInit(options, core.DefaultScaffoldManifest(), core.ReadTemplate)
			if planErr == nil {
				fixed := core.ExecuteInitPlan(plan, false, core.DefaultInitExecutor())
				if fixed.Status == "ready" {
					missing, scaffoldErr = inspectScaffold(root)
				} else {
					planErr = fmt.Errorf("repair failed: %v", fixed.Warnings)
				}
			}
			if planErr != nil {
				addDoctorFailure(&result, "scaffold", planErr.Error(), "run `specd init --repair`")
			}
		}
		if scaffoldErr == nil && len(missing) > 0 {
			addDoctorFailure(&result, "scaffold", "missing: "+strings.Join(missing, ", "), "run `specd doctor --fix` or `specd init --repair`")
		} else if scaffoldErr == nil {
			result.Checks = append(result.Checks, doctorCheck{Name: "scaffold", Status: "pass", Detail: "all required project assets are present"})
		}
	} else {
		result.Checks = append(result.Checks, doctorCheck{Name: "scaffold", Status: "pass", Detail: "all required project assets are present"})
	}

	if probe, probeErr := runtime.Probe(context.Background(), nil, 2*time.Second); probeErr != nil {
		result.Orchestration.ServerCapability = "unavailable"
		addDoctorFailure(&result, "mcp", probeErr.Error(), "run `specd doctor` after rebuilding or reinstalling specd")
	} else {
		result.Orchestration.ServerCapability = "available"
		result.Orchestration.Tools = append([]string(nil), probe.OrchestrationTools...)
		result.Checks = append(result.Checks, doctorCheck{
			Name:   "mcp",
			Status: "pass",
			Detail: fmt.Sprintf("protocol %s; %d tools; orchestration tools: %s", probe.ProtocolVersion, probe.ToolCount, strings.Join(probe.OrchestrationTools, ", ")),
		})
	}

	detections := runtime.Registry.Detect(root)
	hostNames, selectionErr := doctorHostNames(args.Str("agent"), detections, runtime.Registry)
	if selectionErr != nil {
		return usageExit(selectionErr.Error())
	}
	for _, name := range hostNames {
		adapter, _ := runtime.Registry.Get(name)
		detection := detectionForHost(detections, name)
		host := doctorHost{Name: name, Detected: detection.Detected}
		host.LifecycleSupport, host.ReloadRequired, host.TrustRequired = doctorHostLifecycle(name)
		state, inspectErr := adapter.Inspect(root, integration.ScopeProject)
		if inspectErr != nil {
			host.Status = "fail"
			host.Reason = inspectErr.Error()
			host.Remediation = "repair the host project configuration, then run `specd doctor`"
		} else {
			if args.Bool("fix") && detection.Detected && !state.Registered && state.Fingerprint == "" {
				plan, planErr := runtime.Registry.Plan(name, root, integration.ScopeProject)
				if planErr == nil {
					_, planErr = runtime.Registry.Install(context.Background(), plan)
				}
				if planErr != nil {
					host.Status = "fail"
					host.Reason = planErr.Error()
				} else {
					state, inspectErr = adapter.Inspect(root, integration.ScopeProject)
				}
			}
			if inspectErr != nil {
				host.Status = "fail"
				host.Reason = inspectErr.Error()
			} else {
				host.Registered = state.Registered
				host.Owned = state.Owned
				verification := adapter.Verify(root)
				host.Status = verification.Status
				host.Reason = verification.Reason
				host.Remediation = verification.Remedy
			}
		}
		if host.Status != "pass" {
			result.Status = "unhealthy"
			if host.Remediation != "" {
				result.Remediations = append(result.Remediations, host.Remediation)
			}
		}
		result.Hosts = append(result.Hosts, host)
	}

	normalizeDoctorResult(&result)
	if result.Status != "healthy" {
		result.NextAction = firstRemediation(result.Remediations)
	}
	return emitDoctorResult(result, args.Bool("json"))
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
	case "claude-code", "cursor", "gemini":
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
