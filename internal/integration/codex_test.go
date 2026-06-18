package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestCodexNativeProjectAdapterExactArgvAndIdempotency(t *testing.T) {
	root := t.TempDir()
	var calls [][]string
	deps := AdapterDeps{
		Detector: Detector{
			LookPath: func(name string) (string, error) { return "/fixture/bin/" + name, nil },
			Stat:     os.Stat,
		},
		Run: func(_ context.Context, dir, command string, args []string) ([]byte, error) {
			calls = append(calls, append([]string{command}, args...))
			content := "[mcp_servers.specd]\ncommand = \"specd\"\nargs = [\"mcp\", \"--root\", " + quoteTOML(dir) + "]\n"
			if err := os.MkdirAll(filepath.Join(dir, ".codex"), 0o755); err != nil {
				return nil, err
			}
			return nil, os.WriteFile(filepath.Join(dir, ".codex", "config.toml"), []byte(content), 0o644)
		},
		Now: func() time.Time { return time.Date(2026, 6, 18, 1, 2, 3, 0, time.UTC) },
	}
	adapter := NewCodexAdapterWithDeps(deps, true)
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
	want := []string{"codex", "mcp", "add", "--scope", "project", "specd", "--", "specd", "mcp", "--root", root}
	if !reflect.DeepEqual(calls, [][]string{want}) {
		t.Fatalf("calls = %#v, want %#v", calls, [][]string{want})
	}
	if !first.Changed || second.Changed {
		t.Fatalf("install not idempotent: first=%#v second=%#v", first, second)
	}
	state, err := adapter.Inspect(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Registered || !state.Owned {
		t.Fatalf("state = %#v", state)
	}
}

func TestCodexDefaultsToActionableManualProjectConfig(t *testing.T) {
	root := t.TempDir()
	adapter := NewCodexAdapterWithDeps(AdapterDeps{
		Detector: Detector{
			LookPath: func(string) (string, error) { return "", errors.New("not found") },
			Stat:     os.Stat,
		},
	}, false)
	detection := adapter.Detect(root)
	if detection.Detected {
		t.Fatalf("detection = %#v", detection)
	}
	plan, err := adapter.Plan(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Actions) != 1 || plan.Actions[0].Kind != "manual" || len(plan.Warnings) == 0 {
		t.Fatalf("plan = %#v", plan)
	}
	result, err := adapter.Install(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != "manual" || result.NextAction == "" {
		t.Fatalf("result = %#v", result)
	}
}

func quoteTOML(value string) string {
	return `"` + value + `"`
}
