package integration

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestAntigravityProjectAdapterPreservesServersAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".agents", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(`{"theme":"dark","mcpServers":{"other":{"command":"other"}},"telemetry":{"enabled":false}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := NewAntigravityAdapterWithDeps(AdapterDeps{
		Detector: Detector{
			LookPath: func(name string) (string, error) { return "/fixture/" + name, nil },
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
	if len(first.Backups) != 1 || first.Backups[0] == "" {
		t.Fatalf("backup missing: %#v", first)
	}
	assertJSONField(t, target, "theme", "dark")
	assertJSONServer(t, target, "other")
	if got := readJSONDocument(t, target)["telemetry"]; got == nil {
		t.Fatalf("telemetry settings lost: %#v", readJSONDocument(t, target))
	}
	state, err := adapter.Inspect(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Registered || !state.Owned {
		t.Fatalf("state = %#v", state)
	}
}

func TestAntigravityProjectAdapterRefusesUnownedSpecdEntry(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".agents", "mcp_config.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(`{"mcpServers":{"specd":{"command":"other"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}

	adapter := NewAntigravityAdapterWithDeps(AdapterDeps{Now: func() time.Time { return time.Date(2026, 6, 18, 1, 2, 3, 0, time.UTC) }})
	plan, err := adapter.Plan(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Install(context.Background(), plan); err == nil {
		t.Fatal("unowned registration accepted")
	}
}

func TestAntigravityProjectAdapterPlanShape(t *testing.T) {
	root := t.TempDir()
	adapter := NewAntigravityAdapter()
	plan, err := adapter.Plan(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if plan.Host != "antigravity" || plan.Scope != ScopeProject || len(plan.Actions) != 1 {
		t.Fatalf("plan = %#v", plan)
	}
	if got := plan.Actions[0]; got.Kind != "write-json" || got.Command != "" || !reflect.DeepEqual(got.Args, []string{}) {
		t.Fatalf("action = %#v", got)
	}
}
