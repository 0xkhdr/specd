package integration

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"
)

func TestClaudeCodeProjectAdapterPreservesServersAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".mcp.json")
	if err := os.WriteFile(target, []byte(`{"theme":"dark","mcpServers":{"other":{"command":"other"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var calls [][]string
	adapter := NewClaudeCodeAdapterWithDeps(AdapterDeps{
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
	want := []string{"claude", "mcp", "add", "--transport", "stdio", "--scope", "project", "specd", "--", "specd", "mcp", "--root", root}
	if !reflect.DeepEqual(calls, [][]string{want}) {
		t.Fatalf("calls = %#v", calls)
	}
	if !first.Changed || second.Changed {
		t.Fatalf("install not idempotent: first=%#v second=%#v", first, second)
	}
	assertJSONField(t, target, "theme", "dark")
	assertJSONServer(t, target, "other")
	state, err := adapter.Inspect(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Registered || !state.Owned {
		t.Fatalf("state = %#v", state)
	}
}

func TestClaudeCodeRefusesUnownedSpecdEntry(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".mcp.json")
	if err := os.WriteFile(target, []byte(`{"mcpServers":{"specd":{"command":"other"}}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	called := false
	adapter := NewClaudeCodeAdapterWithDeps(AdapterDeps{
		Run: func(context.Context, string, string, []string) ([]byte, error) {
			called = true
			return nil, nil
		},
	})
	plan, err := adapter.Plan(root, ScopeProject)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := adapter.Install(context.Background(), plan); err == nil {
		t.Fatal("unowned registration accepted")
	}
	if called {
		t.Fatal("host command ran before ownership conflict was resolved")
	}
}

func jsonConfigRunner(t *testing.T, target, root string, calls *[][]string) CommandRunner {
	t.Helper()
	return func(_ context.Context, _ string, command string, args []string) ([]byte, error) {
		*calls = append(*calls, append([]string{command}, args...))
		data, err := os.ReadFile(target)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		document := map[string]any{}
		if len(data) > 0 {
			if err := json.Unmarshal(data, &document); err != nil {
				return nil, err
			}
		}
		servers, _ := document["mcpServers"].(map[string]any)
		if servers == nil {
			servers = map[string]any{}
			document["mcpServers"] = servers
		}
		servers["specd"] = specdServer(root)
		updated, err := json.MarshalIndent(document, "", "  ")
		if err != nil {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return nil, err
		}
		return nil, os.WriteFile(target, append(updated, '\n'), 0o644)
	}
}

func assertJSONField(t *testing.T, target, key string, want any) {
	t.Helper()
	document := readJSONDocument(t, target)
	if !reflect.DeepEqual(document[key], want) {
		t.Fatalf("%s = %#v, want %#v", key, document[key], want)
	}
}

func assertJSONServer(t *testing.T, target, name string) {
	t.Helper()
	document := readJSONDocument(t, target)
	servers, _ := document["mcpServers"].(map[string]any)
	if servers[name] == nil {
		t.Fatalf("server %q missing from %#v", name, servers)
	}
}

func readJSONDocument(t *testing.T, target string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	return document
}
