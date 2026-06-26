package integration

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"sort"
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

func TestCompatibilityMatrixMatchesProjectAdapters(t *testing.T) {
	rows := compatibilityRows(t)
	var documented []string
	for host, adapter := range rows {
		if adapter == "project" {
			documented = append(documented, host)
		}
	}
	sort.Strings(documented)
	if got := DefaultRegistry().Names(); !reflect.DeepEqual(got, documented) {
		t.Fatalf("project adapter registry %v != compatibility matrix %v", got, documented)
	}
}

func TestDefaultRegistryIncludesAntigravityNotGemini(t *testing.T) {
	got := DefaultRegistry().Names()
	if i := sort.SearchStrings(got, "antigravity"); i == len(got) || got[i] != "antigravity" {
		t.Fatalf("DefaultRegistry().Names() = %v, want antigravity", got)
	}
	if i := sort.SearchStrings(got, "gemini"); i < len(got) && got[i] == "gemini" {
		t.Fatalf("DefaultRegistry().Names() = %v, want no gemini", got)
	}
}

func TestDefaultAdapterConformance(t *testing.T) {
	root := t.TempDir()
	registry := DefaultRegistry()
	for _, adapter := range registry.Adapters() {
		t.Run(adapter.Name(), func(t *testing.T) {
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

			first, err := registry.Plan(adapter.Name(), root, ScopeProject)
			if err != nil {
				t.Fatal(err)
			}
			second, err := registry.Plan(adapter.Name(), root, ScopeProject)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first, second) {
				t.Fatalf("plan is not deterministic:\n%#v\n%#v", first, second)
			}
			for _, action := range first.Actions {
				switch action.Command {
				case "sh", "bash", "zsh", "cmd.exe":
					t.Fatalf("plan uses a shell: %#v", action)
				}
				if action.Target == "" {
					t.Fatal("plan action has no target")
				}
				rel, err := filepath.Rel(root, action.Target)
				if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					t.Fatalf("project action escapes root: %s", action.Target)
				}
			}
		})
	}
}

func compatibilityRows(t *testing.T) map[string]string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate conformance test")
	}
	path := filepath.Join(filepath.Dir(file), "..", "..", "docs", "agent-harness-compat.md")
	handle, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer handle.Close()

	rows := map[string]string{}
	scanner := bufio.NewScanner(handle)
	inHosts := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "## Hosts" {
			inHosts = true
			continue
		}
		if inHosts && strings.HasPrefix(line, "## ") {
			break
		}
		if !inHosts || !strings.HasPrefix(line, "|") {
			continue
		}
		cells := strings.Split(strings.Trim(line, "|"), "|")
		if len(cells) < 2 {
			continue
		}
		host := strings.TrimSpace(cells[0])
		adapter := strings.TrimSpace(cells[1])
		if host == "Host" || strings.HasPrefix(host, "---") {
			continue
		}
		rows[host] = adapter
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if len(rows) == 0 {
		t.Fatal("compatibility matrix has no host rows")
	}
	return rows
}
