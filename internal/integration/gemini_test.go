package integration

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestGeminiProjectAdapterPreservesSettingsAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".gemini", "settings.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(target, []byte(`{"security":{"folderTrust":{"enabled":true}},"mcpServers":{"other":{"command":"other"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	adapter := NewGeminiAdapterWithDeps(AdapterDeps{
		Detector: Detector{LookPath: func(name string) (string, error) { return "/fixture/" + name, nil }, Stat: os.Stat},
		Run:      jsonConfigRunner(t, target, root, &calls),
		Now:      func() time.Time { return time.Date(2026, 6, 18, 1, 2, 3, 0, time.UTC) },
	})
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
	want := []string{"gemini", "mcp", "add", "--scope", "project", "--transport", "stdio", "specd", "specd", "mcp", "--root", root}
	if !reflect.DeepEqual(calls, [][]string{want}) {
		t.Fatalf("calls = %#v", calls)
	}
	if !first.Changed || second.Changed {
		t.Fatalf("install not idempotent: first=%#v second=%#v", first, second)
	}
	assertJSONServer(t, target, "other")
	document := readJSONDocument(t, target)
	if document["security"] == nil {
		t.Fatalf("security settings lost: %#v", document)
	}
	state, err := adapter.Inspect(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Registered || !state.Owned {
		t.Fatalf("state = %#v", state)
	}
}

func TestDefaultRegistryIncludesWaveFiveCLIAdapters(t *testing.T) {
	got := DefaultRegistry().Names()
	want := []string{"claude-code", "codex", "gemini"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DefaultRegistry names = %v, want %v", got, want)
	}
}
