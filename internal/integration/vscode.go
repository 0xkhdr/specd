package integration

import (
	"context"
	"fmt"
	"path/filepath"
)

type VSCodeAdapter struct {
	deps AdapterDeps
}

func NewVSCodeAdapter() *VSCodeAdapter {
	return &VSCodeAdapter{deps: defaultAdapterDeps()}
}

func NewVSCodeAdapterWithDeps(deps AdapterDeps) *VSCodeAdapter {
	return &VSCodeAdapter{deps: normalizeAdapterDeps(deps)}
}

func (a *VSCodeAdapter) Name() string { return "vscode" }

func (a *VSCodeAdapter) Scopes() []Scope { return []Scope{ScopeProject} }

func (a *VSCodeAdapter) Detect(root string) Detection {
	res := a.deps.Detector.Detect(root, a.Name(), DetectionProbe{
		Executable:    "code",
		ProjectConfig: ".vscode/mcp.json",
		Scopes:        a.Scopes(),
		Method:        "project-json",
	})
	if res.Executable != "" {
		return res
	}
	resInsiders := a.deps.Detector.Detect(root, a.Name(), DetectionProbe{
		Executable:    "code-insiders",
		ProjectConfig: ".vscode/mcp.json",
		Scopes:        a.Scopes(),
		Method:        "project-json",
	})
	if resInsiders.Executable != "" {
		return resInsiders
	}
	return res
}

func (a *VSCodeAdapter) Plan(root string, scope Scope) (HostPlan, error) {
	if scope != ScopeProject {
		return HostPlan{}, fmt.Errorf("vscode adapter supports project scope only")
	}
	target := filepath.Join(root, ".vscode", "mcp.json")
	return HostPlan{
		Host: a.Name(), Root: root, Scope: scope,
		Actions: []HostAction{{
			Kind:        "write-json",
			Target:      target,
			Description: "merge specd into VS Code workspace MCP configuration",
			Args:        []string{},
		}},
		Warnings: []string{},
	}, nil
}

func (a *VSCodeAdapter) Install(_ context.Context, plan HostPlan) (HostResult, error) {
	server := specdServer(plan.Root)
	server["type"] = "stdio"
	return installProjectJSON(
		plan,
		a.deps,
		filepath.Join(plan.Root, ".vscode", "mcp.json"),
		[]string{"servers"},
		server,
		"reload VS Code, review the workspace trust prompt, and start specd from MCP: List Servers",
	)
}

func (a *VSCodeAdapter) Inspect(root string, scope Scope) (HostState, error) {
	if scope != ScopeProject {
		return HostState{}, fmt.Errorf("vscode adapter supports project scope only")
	}
	state, _, err := inspectJSONServerAtPath(
		root,
		a.Name(),
		filepath.Join(root, ".vscode", "mcp.json"),
		scope,
		[]string{"servers"},
	)
	return state, err
}

func (a *VSCodeAdapter) Verify(root string) Verification {
	state, err := a.Inspect(root, ScopeProject)
	if err != nil {
		return Verification{
			Host:   a.Name(),
			Status: "manual",
			Reason: err.Error(),
			Remedy: "repair .vscode/mcp.json or use MCP: Add Server with Workspace scope",
		}
	}
	if !state.Registered {
		return Verification{
			Host:   a.Name(),
			Status: "manual",
			Reason: state.Reason,
			Remedy: "run `specd init --agent vscode --repair`, then review workspace trust",
		}
	}
	return Verification{Host: a.Name(), Status: "pass", Reason: state.Reason}
}
