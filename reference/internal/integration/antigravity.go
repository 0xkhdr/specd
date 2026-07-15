package integration

import (
	"context"
	"fmt"
	"path/filepath"
)

type AntigravityAdapter struct {
	deps AdapterDeps
}

func NewAntigravityAdapter() *AntigravityAdapter {
	return &AntigravityAdapter{deps: defaultAdapterDeps()}
}

func NewAntigravityAdapterWithDeps(deps AdapterDeps) *AntigravityAdapter {
	return &AntigravityAdapter{deps: normalizeAdapterDeps(deps)}
}

func (a *AntigravityAdapter) Name() string { return "antigravity" }

func (a *AntigravityAdapter) Scopes() []Scope { return []Scope{ScopeProject} }

func (a *AntigravityAdapter) Detect(root string) Detection {
	return a.deps.Detector.Detect(root, a.Name(), DetectionProbe{
		Executable:    "agy",
		ProjectConfig: ".agents/mcp_config.json",
		Scopes:        a.Scopes(),
		Method:        "project-json",
	})
}

func (a *AntigravityAdapter) Plan(root string, scope Scope) (HostPlan, error) {
	if scope != ScopeProject {
		return HostPlan{}, fmt.Errorf("antigravity adapter supports project scope only")
	}
	target := filepath.Join(root, ".agents", "mcp_config.json")
	return HostPlan{
		Host:  a.Name(),
		Root:  root,
		Scope: scope,
		Actions: []HostAction{{
			Kind:        "write-json",
			Target:      target,
			Description: "merge specd into Antigravity project configuration",
			Args:        []string{},
		}},
		Warnings: []string{},
	}, nil
}

func (a *AntigravityAdapter) Install(_ context.Context, plan HostPlan) (HostResult, error) {
	return installProjectJSON(
		plan,
		a.deps,
		filepath.Join(plan.Root, ".agents", "mcp_config.json"),
		[]string{"mcpServers"},
		specdServer(plan.Root),
		"reload Antigravity and confirm the specd server is available",
	)
}

func (a *AntigravityAdapter) Inspect(root string, scope Scope) (HostState, error) {
	if scope != ScopeProject {
		return HostState{}, fmt.Errorf("antigravity adapter supports project scope only")
	}
	state, _, err := inspectJSONServerAtPath(
		root,
		a.Name(),
		filepath.Join(root, ".agents", "mcp_config.json"),
		scope,
		[]string{"mcpServers"},
	)
	return state, err
}

func (a *AntigravityAdapter) Verify(root string) Verification {
	state, err := a.Inspect(root, ScopeProject)
	if err != nil {
		return Verification{Host: a.Name(), Status: "fail", Reason: err.Error(), Remedy: "repair .agents/mcp_config.json"}
	}
	if !state.Registered {
		return Verification{Host: a.Name(), Status: "fail", Reason: state.Reason, Remedy: "run specd init --agent antigravity --repair"}
	}
	return Verification{Host: a.Name(), Status: "pass", Reason: state.Reason}
}
