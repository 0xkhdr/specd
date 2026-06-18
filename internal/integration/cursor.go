package integration

import (
	"context"
	"fmt"
	"path/filepath"
)

type CursorAdapter struct {
	deps AdapterDeps
}

func NewCursorAdapter() *CursorAdapter {
	return &CursorAdapter{deps: defaultAdapterDeps()}
}

func NewCursorAdapterWithDeps(deps AdapterDeps) *CursorAdapter {
	return &CursorAdapter{deps: normalizeAdapterDeps(deps)}
}

func (a *CursorAdapter) Name() string { return "cursor" }

func (a *CursorAdapter) Scopes() []Scope { return []Scope{ScopeProject} }

func (a *CursorAdapter) Detect(root string) Detection {
	return a.deps.Detector.Detect(root, a.Name(), DetectionProbe{
		Executable:    "cursor",
		ProjectConfig: ".cursor/mcp.json",
		Scopes:        a.Scopes(),
		Method:        "project-json",
	})
}

func (a *CursorAdapter) Plan(root string, scope Scope) (HostPlan, error) {
	if scope != ScopeProject {
		return HostPlan{}, fmt.Errorf("cursor adapter supports project scope only")
	}
	target := filepath.Join(root, ".cursor", "mcp.json")
	return HostPlan{
		Host: a.Name(), Root: root, Scope: scope,
		Actions: []HostAction{{
			Kind:        "write-json",
			Target:      target,
			Description: "merge specd into Cursor workspace MCP configuration",
			Args:        []string{},
		}},
		Warnings: []string{},
	}, nil
}

func (a *CursorAdapter) Install(_ context.Context, plan HostPlan) (HostResult, error) {
	return installWorkspaceJSON(
		plan,
		a.deps,
		filepath.Join(plan.Root, ".cursor", "mcp.json"),
		[]string{"mcpServers"},
		specdServer(plan.Root),
		"reload Cursor, open Settings > Tools & MCP, and confirm the specd server is enabled",
	)
}

func (a *CursorAdapter) Inspect(root string, scope Scope) (HostState, error) {
	if scope != ScopeProject {
		return HostState{}, fmt.Errorf("cursor adapter supports project scope only")
	}
	state, _, err := inspectJSONServerAtPath(
		root,
		a.Name(),
		filepath.Join(root, ".cursor", "mcp.json"),
		scope,
		[]string{"mcpServers"},
	)
	return state, err
}

func (a *CursorAdapter) Verify(root string) Verification {
	state, err := a.Inspect(root, ScopeProject)
	if err != nil {
		return Verification{
			Host:   a.Name(),
			Status: "manual",
			Reason: err.Error(),
			Remedy: "repair .cursor/mcp.json or merge the generated Cursor snippet manually",
		}
	}
	if !state.Registered {
		return Verification{
			Host:   a.Name(),
			Status: "manual",
			Reason: state.Reason,
			Remedy: "run `specd init --agent cursor --repair`, then enable specd in Cursor Tools & MCP",
		}
	}
	return Verification{Host: a.Name(), Status: "pass", Reason: state.Reason}
}
