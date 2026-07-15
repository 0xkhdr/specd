package integration

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestVSCodeWorkspaceAdapterCurrentSchemaAndIdempotency(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".vscode", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{"servers":{"other":{"type":"stdio","command":"other"}},"inputs":[]}`
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := NewVSCodeAdapterWithDeps(AdapterDeps{
		Detector: Detector{
			LookPath: func(name string) (string, error) { return "/fixture/bin/" + name, nil },
			Stat:     os.Stat,
		},
		Now: func() time.Time { return time.Date(2026, 6, 18, 1, 2, 3, 0, time.UTC) },
	})

	detection := adapter.Detect(root)
	if !detection.Detected || detection.Executable == "" || detection.ProjectConfig != target {
		t.Fatalf("detection = %#v", detection)
	}
	plan, err := adapter.Plan(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	first, err := adapter.Install(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	second, err := adapter.Install(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Changed || second.Changed {
		t.Fatalf("install not idempotent: first=%#v second=%#v", first, second)
	}
	if len(first.Backups) != 1 || !strings.Contains(first.NextAction, "workspace trust") {
		t.Fatalf("first result = %#v", first)
	}
	assertJSONServerAtPath(t, target, []string{"servers"}, "other")
	assertJSONServerAtPath(t, target, []string{"servers"}, "specd")
	document := readJSONDocument(t, target)
	servers := document["servers"].(map[string]any)
	specd := servers["specd"].(map[string]any)
	if specd["type"] != "stdio" {
		t.Fatalf("specd server = %#v", specd)
	}
	if _, ok := document["inputs"]; !ok {
		t.Fatalf("inputs lost: %#v", document)
	}
	state, err := adapter.Inspect(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Registered || !state.Owned {
		t.Fatalf("state = %#v", state)
	}
}

func TestVSCodeWorkspaceAdapterFallsBackFromLegacySettingsSchema(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".vscode", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := `{"servers":[],"mcp":{"servers":{"other":{"command":"other"}}}}`
	if err := os.WriteFile(target, []byte(legacy), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := NewVSCodeAdapter()
	plan, err := adapter.Plan(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	result, err := adapter.Install(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "manual" || result.Changed {
		t.Fatalf("result = %#v", result)
	}
	after, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(after, []byte(legacy)) {
		t.Fatalf("legacy config changed:\n%s", after)
	}
}

func assertJSONServerAtPath(t *testing.T, target string, path []string, name string) {
	t.Helper()
	current := any(readJSONDocument(t, target))
	for _, key := range path {
		object, ok := current.(map[string]any)
		if !ok {
			t.Fatalf("%s is not an object in %#v", key, current)
		}
		current = object[key]
	}
	servers, ok := current.(map[string]any)
	if !ok || servers[name] == nil {
		t.Fatalf("server %q missing from %#v", name, current)
	}
}

func TestVSCodeAdapterDetectInsiders(t *testing.T) {
	root := t.TempDir()
	adapter := NewVSCodeAdapterWithDeps(AdapterDeps{
		Detector: Detector{
			LookPath: func(name string) (string, error) {
				if name == "code-insiders" {
					return "/fixture/bin/code-insiders", nil
				}
				return "", os.ErrNotExist
			},
			Stat: os.Stat,
		},
	})
	detection := adapter.Detect(root)
	if !detection.Detected {
		t.Fatalf("expected detected for code-insiders")
	}
	if detection.Executable != "/fixture/bin/code-insiders" {
		t.Errorf("expected executable /fixture/bin/code-insiders, got %s", detection.Executable)
	}
}
