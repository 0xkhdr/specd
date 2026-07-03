package integration

import (
	"context"
	"fmt"
	"path/filepath"
)

type ClaudeCodeAdapter struct {
	deps AdapterDeps
}

func NewClaudeCodeAdapter() *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{deps: defaultAdapterDeps()}
}

func NewClaudeCodeAdapterWithDeps(deps AdapterDeps) *ClaudeCodeAdapter {
	return &ClaudeCodeAdapter{deps: normalizeAdapterDeps(deps)}
}

func (a *ClaudeCodeAdapter) Name() string { return "claude-code" }

func (a *ClaudeCodeAdapter) Scopes() []Scope { return []Scope{ScopeProject} }

func (a *ClaudeCodeAdapter) Detect(root string) Detection {
	return a.deps.Detector.Detect(root, a.Name(), DetectionProbe{
		Executable:    "claude",
		ProjectConfig: ".mcp.json",
		Scopes:        a.Scopes(),
		Method:        "native-cli",
	})
}

func (a *ClaudeCodeAdapter) Plan(root string, scope Scope) (HostPlan, error) {
	if scope != ScopeProject {
		return HostPlan{}, fmt.Errorf("claude-code adapter supports project scope only")
	}
	target := filepath.Join(root, ".mcp.json")
	return HostPlan{
		Host: a.Name(), Root: root, Scope: scope,
		Actions: []HostAction{{
			Kind: "native-cli", Target: target, Command: "claude",
			Args:        []string{"mcp", "add", "--transport", "stdio", "--scope", "project", specdServerName, "--", "specd", "mcp", "--root", root},
			Description: "register specd in Claude Code project configuration",
		}},
		Warnings: []string{},
	}, nil
}

func (a *ClaudeCodeAdapter) Install(ctx context.Context, plan HostPlan) (HostResult, error) {
	return installNativeJSON(ctx, a.deps, plan, filepath.Join(plan.Root, ".mcp.json"))
}

func (a *ClaudeCodeAdapter) Inspect(root string, scope Scope) (HostState, error) {
	if scope != ScopeProject {
		return HostState{}, fmt.Errorf("claude-code adapter supports project scope only")
	}
	state, _, err := inspectJSONServer(root, a.Name(), filepath.Join(root, ".mcp.json"), scope)
	return state, err
}

func (a *ClaudeCodeAdapter) Verify(root string) Verification {
	state, err := a.Inspect(root, ScopeProject)
	if err != nil {
		return Verification{Host: a.Name(), Status: "fail", Reason: err.Error(), Remedy: "repair .mcp.json"}
	}
	if !state.Registered {
		return Verification{Host: a.Name(), Status: "fail", Reason: state.Reason, Remedy: "run specd init --agent claude-code --repair"}
	}
	return Verification{Host: a.Name(), Status: "pass", Reason: state.Reason}
}
