package integration

import (
	"context"
	"fmt"
	"path/filepath"
)

type GeminiAdapter struct {
	deps AdapterDeps
}

func NewGeminiAdapter() *GeminiAdapter {
	return &GeminiAdapter{deps: defaultAdapterDeps()}
}

func NewGeminiAdapterWithDeps(deps AdapterDeps) *GeminiAdapter {
	return &GeminiAdapter{deps: normalizeAdapterDeps(deps)}
}

func (a *GeminiAdapter) Name() string { return "gemini" }

func (a *GeminiAdapter) Scopes() []Scope { return []Scope{ScopeProject} }

func (a *GeminiAdapter) Detect(root string) Detection {
	return a.deps.Detector.Detect(root, a.Name(), DetectionProbe{
		Executable:    "gemini",
		ProjectConfig: ".gemini/settings.json",
		Scopes:        a.Scopes(),
		Method:        "native-cli",
	})
}

func (a *GeminiAdapter) Plan(root string, scope Scope) (HostPlan, error) {
	if scope != ScopeProject {
		return HostPlan{}, fmt.Errorf("gemini adapter supports project scope only")
	}
	target := filepath.Join(root, ".gemini", "settings.json")
	return HostPlan{
		Host: a.Name(), Root: root, Scope: scope,
		Actions: []HostAction{{
			Kind: "native-cli", Target: target, Command: "gemini",
			Args:        []string{"mcp", "add", "--scope", "project", "--transport", "stdio", specdServerName, "specd", "mcp", "--root", root},
			Description: "register specd in Gemini CLI project configuration",
		}},
		Warnings: []string{},
	}, nil
}

func (a *GeminiAdapter) Install(ctx context.Context, plan HostPlan) (HostResult, error) {
	return installNativeJSON(ctx, a.deps, plan, filepath.Join(plan.Root, ".gemini", "settings.json"))
}

func (a *GeminiAdapter) Inspect(root string, scope Scope) (HostState, error) {
	if scope != ScopeProject {
		return HostState{}, fmt.Errorf("gemini adapter supports project scope only")
	}
	state, _, err := inspectJSONServer(root, a.Name(), filepath.Join(root, ".gemini", "settings.json"), scope)
	return state, err
}

func (a *GeminiAdapter) Verify(root string) Verification {
	state, err := a.Inspect(root, ScopeProject)
	if err != nil {
		return Verification{Host: a.Name(), Status: "fail", Reason: err.Error(), Remedy: "repair .gemini/settings.json"}
	}
	if !state.Registered {
		return Verification{Host: a.Name(), Status: "fail", Reason: state.Reason, Remedy: "run specd init --agent gemini --repair"}
	}
	return Verification{Host: a.Name(), Status: "pass", Reason: state.Reason}
}
