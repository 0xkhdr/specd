package integration

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestCursorWorkspaceAdapterCurrentSchemaAndIdempotency(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{"mcpServers":{"other":{"command":"other"}},"projectSetting":true}`
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	adapter := NewCursorAdapterWithDeps(AdapterDeps{
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
	if len(first.Backups) != 1 || !strings.Contains(first.NextAction, "Tools & MCP") {
		t.Fatalf("first result = %#v", first)
	}
	assertJSONField(t, target, "projectSetting", true)
	assertJSONServer(t, target, "other")
	state, err := adapter.Inspect(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Registered || !state.Owned {
		t.Fatalf("state = %#v", state)
	}
}

func TestCursorWorkspaceAdapterFallsBackWithoutDestructiveWrite(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "invalid JSON", content: `{"mcpServers":`},
		{name: "unknown schema", content: `{"mcpServers":[],"other":"preserve"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, ".cursor", "mcp.json")
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(target, []byte(tc.content), 0o644); err != nil {
				t.Fatal(err)
			}
			adapter := NewCursorAdapterWithDeps(AdapterDeps{
				Detector: Detector{
					LookPath: func(string) (string, error) { return "", errors.New("not found") },
					Stat:     os.Stat,
				},
			})
			plan, err := adapter.Plan(root, ScopeProject)
			if err != nil {
				t.Fatal(err)
			}
			result, err := adapter.Install(context.Background(), plan)
			if err != nil {
				t.Fatal(err)
			}
			if result.Status != "manual" || result.Changed || len(result.Warnings) == 0 {
				t.Fatalf("result = %#v", result)
			}
			after, err := os.ReadFile(target)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(after, []byte(tc.content)) {
				t.Fatalf("config changed:\n%s", after)
			}
		})
	}
}
