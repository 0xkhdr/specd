package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CodexAdapter uses a manual project config by default because the current
// official `codex mcp add` command writes user configuration and has no project
// scope flag. NativeProjectCLI is an explicit capability switch for a future
// Codex release that provides project-scoped registration.
type CodexAdapter struct {
	deps             AdapterDeps
	NativeProjectCLI bool
}

func NewCodexAdapter() *CodexAdapter {
	return &CodexAdapter{deps: defaultAdapterDeps()}
}

func NewCodexAdapterWithDeps(deps AdapterDeps, nativeProjectCLI bool) *CodexAdapter {
	return &CodexAdapter{deps: normalizeAdapterDeps(deps), NativeProjectCLI: nativeProjectCLI}
}

func (a *CodexAdapter) Name() string { return "codex" }

func (a *CodexAdapter) Scopes() []Scope { return []Scope{ScopeProject} }

func (a *CodexAdapter) Detect(root string) Detection {
	return a.deps.Detector.Detect(root, a.Name(), DetectionProbe{
		Executable:    "codex",
		ProjectConfig: ".codex/config.toml",
		Scopes:        a.Scopes(),
		Method:        "native-cli-or-manual",
	})
}

func (a *CodexAdapter) Plan(root string, scope Scope) (HostPlan, error) {
	if scope != ScopeProject {
		return HostPlan{}, fmt.Errorf("codex adapter supports project scope only")
	}
	target := filepath.Join(root, ".codex", "config.toml")
	if a.NativeProjectCLI {
		return HostPlan{
			Host: a.Name(), Root: root, Scope: scope,
			Actions: []HostAction{{
				Kind: "native-cli", Target: target, Command: "codex",
				Args:        []string{"mcp", "add", "--scope", "project", specdServerName, "--", "specd", "mcp", "--root", root},
				Description: "register specd through Codex project-scoped MCP management",
			}},
			Warnings: []string{},
		}, nil
	}
	return HostPlan{
		Host: a.Name(), Root: root, Scope: scope,
		Actions: []HostAction{{
			Kind:        "manual",
			Target:      target,
			Description: "merge the specd MCP table into the trusted project .codex/config.toml",
			Args:        []string{},
		}},
		Warnings: []string{"current codex mcp add has no project-scope flag; no user configuration will be changed"},
	}, nil
}

func (a *CodexAdapter) Install(ctx context.Context, plan HostPlan) (HostResult, error) {
	target := filepath.Join(plan.Root, ".codex", "config.toml")
	if len(plan.Actions) == 1 && plan.Actions[0].Kind == "native-cli" {
		return a.installNative(ctx, plan, target)
	}
	state, _, err := inspectCodexProject(plan.Root, target)
	if err != nil {
		return HostResult{}, err
	}
	result := HostResult{
		Host: a.Name(), Status: "manual", Changed: false,
		Targets: []string{target}, Backups: []string{},
		Warnings:   []string{"Codex project registration requires a manual TOML merge"},
		NextAction: "merge the generated [mcp_servers.specd] snippet into .codex/config.toml and trust the project",
	}
	if state.Registered {
		result.Status = "configured"
		result.Warnings = []string{}
		result.NextAction = "trust or reload the project in Codex and confirm the specd tools are available"
	}
	return result, nil
}

func (a *CodexAdapter) installNative(ctx context.Context, plan HostPlan, target string) (HostResult, error) {
	if err := validateConfigTarget(plan.Root, target); err != nil {
		return HostResult{}, err
	}
	before, _, err := inspectCodexProject(plan.Root, target)
	if err != nil {
		return HostResult{}, err
	}
	result := HostResult{
		Host: a.Name(), Status: "configured", Targets: []string{target},
		Backups: []string{}, Warnings: []string{},
		NextAction: "trust or reload the project in Codex and confirm the specd tools are available",
	}
	if before.Registered {
		if before.Owned {
			return result, nil
		}
		if before.Reason == "specd server registration differs from the owned manifest entry" {
			return result, fmt.Errorf("codex registration ownership mismatch at %s", target)
		}
		result.Status = "existing"
		result.Warnings = []string{"matching unowned specd registration left unchanged"}
		return result, nil
	}
	if before.Fingerprint != "" {
		return result, fmt.Errorf("codex has an existing unowned specd registration at %s", target)
	}
	action := plan.Actions[0]
	output, err := a.deps.Run(ctx, plan.Root, action.Command, action.Args)
	if err != nil {
		return result, fmt.Errorf("codex registration failed: %w: %s", err, strings.TrimSpace(string(output)))
	}
	after, canonical, err := inspectCodexProject(plan.Root, target)
	if err != nil {
		return result, err
	}
	if !after.Registered {
		return result, fmt.Errorf("codex command completed without installing the expected project registration")
	}
	if err := recordIntegration(plan.Root, a.Name(), plan.Scope, target, "native-cli", canonical, a.deps.Now()); err != nil {
		return result, err
	}
	result.Changed = true
	return result, nil
}

func (a *CodexAdapter) Inspect(root string, scope Scope) (HostState, error) {
	if scope != ScopeProject {
		return HostState{}, fmt.Errorf("codex adapter supports project scope only")
	}
	state, _, err := inspectCodexProject(root, filepath.Join(root, ".codex", "config.toml"))
	return state, err
}

func (a *CodexAdapter) Verify(root string) Verification {
	state, err := a.Inspect(root, ScopeProject)
	if err != nil {
		return Verification{Host: a.Name(), Status: "fail", Reason: err.Error(), Remedy: "repair .codex/config.toml"}
	}
	if !state.Registered {
		return Verification{Host: a.Name(), Status: "manual", Reason: state.Reason, Remedy: "merge the specd MCP snippet into .codex/config.toml"}
	}
	return Verification{Host: a.Name(), Status: "pass", Reason: state.Reason}
}

func inspectCodexProject(root, target string) (HostState, []byte, error) {
	state := HostState{Host: "codex", Scope: ScopeProject, Target: target}
	data, err := os.ReadFile(target)
	if os.IsNotExist(err) {
		state.Reason = "project configuration does not exist"
		return state, nil, nil
	}
	if err != nil {
		return state, nil, err
	}
	block := codexServerBlock(string(data))
	if block == "" {
		state.Reason = "specd server is not registered"
		return state, nil, nil
	}
	canonical := []byte(strings.TrimSpace(block))
	state.Fingerprint = Fingerprint(canonical)
	command, args, valid := parseCodexServerBlock(block)
	state.Registered = valid && command == "specd" &&
		len(args) == 3 && args[0] == "mcp" && args[1] == "--root" && args[2] == root
	if !state.Registered {
		state.Reason = "specd server entry does not match the project root"
		return state, canonical, nil
	}
	state.Reason = "specd server is registered for this project"
	manifest, err := LoadManifest(coreIntegrationsPath(root))
	if err != nil {
		return state, canonical, err
	}
	for _, entry := range manifest.Entries {
		if entry.Host == "codex" && entry.Scope == ScopeProject && entry.ServerName == specdServerName {
			state.Owned = entry.Fingerprint == state.Fingerprint
			if !state.Owned {
				state.Reason = "specd server registration differs from the owned manifest entry"
			}
			break
		}
	}
	return state, canonical, nil
}

func parseCodexServerBlock(block string) (string, []string, bool) {
	var command string
	var args []string
	for _, rawLine := range strings.Split(block, "\n")[1:] {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return "", nil, false
		}
		switch strings.TrimSpace(key) {
		case "command":
			parsed, err := strconv.Unquote(strings.TrimSpace(value))
			if err != nil {
				return "", nil, false
			}
			command = parsed
		case "args":
			if err := json.Unmarshal([]byte(strings.TrimSpace(value)), &args); err != nil {
				return "", nil, false
			}
		}
	}
	return command, args, command != "" && args != nil
}

func codexServerBlock(content string) string {
	lines := strings.Split(content, "\n")
	start := -1
	for i, line := range lines {
		if strings.TrimSpace(line) == "[mcp_servers.specd]" {
			start = i
			break
		}
	}
	if start < 0 {
		return ""
	}
	end := len(lines)
	for i := start + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			end = i
			break
		}
	}
	return strings.Join(lines[start:end], "\n")
}

func coreIntegrationsPath(root string) string {
	return filepath.Join(root, ".specd", "integrations.json")
}
