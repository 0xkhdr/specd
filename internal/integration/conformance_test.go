package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fakeAdapter struct {
	name      string
	detection Detection
	installed bool
}

func (a *fakeAdapter) Name() string    { return a.name }
func (a *fakeAdapter) Scopes() []Scope { return []Scope{ScopeProject} }
func (a *fakeAdapter) Detect(string) Detection {
	return a.detection
}
func (a *fakeAdapter) Plan(root string, scope Scope) (HostPlan, error) {
	return HostPlan{
		Host:  a.name,
		Root:  root,
		Scope: scope,
		Actions: []HostAction{{
			Kind:        "write-json",
			Target:      filepath.Join(root, ".host", "mcp.json"),
			Description: "install project MCP entry",
			Args:        []string{},
		}},
		Warnings: []string{},
	}, nil
}
func (a *fakeAdapter) Install(_ context.Context, plan HostPlan) (HostResult, error) {
	changed := !a.installed
	a.installed = true
	return HostResult{
		Host:       a.name,
		Status:     "configured",
		Changed:    changed,
		Targets:    []string{plan.Actions[0].Target},
		Backups:    []string{},
		Warnings:   []string{},
		NextAction: "reload host",
	}, nil
}
func (a *fakeAdapter) Inspect(string, Scope) (HostState, error) {
	return HostState{Host: a.name, Scope: ScopeProject, Registered: a.installed}, nil
}
func (a *fakeAdapter) Verify(string) Verification {
	return Verification{Host: a.name, Status: "pass"}
}

func TestAdapterConformance(t *testing.T) {
	root := t.TempDir()
	adapter := &fakeAdapter{
		name: "fake",
		detection: Detection{
			Host:       "fake",
			Detected:   true,
			Scopes:     []Scope{ScopeProject},
			Method:     "project-file",
			Confidence: ConfidenceHigh,
			Reason:     "fixture marker found",
		},
	}
	registry, err := NewRegistry(adapter)
	if err != nil {
		t.Fatal(err)
	}

	first, err := registry.Plan("fake", root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	second, err := registry.Plan("fake", root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if err := validatePlan(adapter, root, ScopeProject, first); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(normalizePlan(first), normalizePlan(second)) {
		t.Fatalf("plan is not deterministic:\n%#v\n%#v", first, second)
	}
	encoded, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(strings.ToLower(string(encoded)), "secret") ||
		strings.Contains(strings.ToLower(string(encoded)), "token") {
		t.Fatalf("plan contains secret-like fields: %s", encoded)
	}
	for _, action := range first.Actions {
		if action.Command == "sh" || action.Command == "bash" ||
			action.Command == "zsh" || action.Command == "cmd.exe" {
			t.Fatalf("adapter plan uses a shell: %#v", action)
		}
		rel, err := filepath.Rel(root, action.Target)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			t.Fatalf("project action escapes root: %s", action.Target)
		}
	}
	if got := registry.Names(); !reflect.DeepEqual(got, []string{"fake"}) {
		t.Fatalf("registry names = %v", got)
	}

	one, err := registry.Install(context.Background(), first)
	if err != nil {
		t.Fatal(err)
	}
	two, err := registry.Install(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if !one.Changed || two.Changed {
		t.Fatalf("install is not idempotent: first=%#v second=%#v", one, two)
	}
}

func TestRegistryDeterministicAndValidated(t *testing.T) {
	a := &fakeAdapter{name: "zeta"}
	b := &fakeAdapter{name: "alpha"}
	registry, err := NewRegistry(a, b)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := registry.Names(), []string{"alpha", "zeta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Names() = %v, want %v", got, want)
	}
	if _, err := NewRegistry(a, a); err == nil {
		t.Fatal("duplicate adapter accepted")
	}
}

func TestAdapterConformanceDoesNotWriteDuringDetection(t *testing.T) {
	root := t.TempDir()
	adapter := &fakeAdapter{name: "fake", detection: Detection{Host: "fake"}}
	before, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	adapter.Detect(root)
	after, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatalf("detection modified project: before=%v after=%v", before, after)
	}
}
