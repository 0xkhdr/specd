package integration

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestJSONMergePreservesUnrelatedValuesAndIsIdempotent(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, ".cursor", "mcp.json")
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		t.Fatal(err)
	}
	original := `{"theme":"dark","mcpServers":{"other":{"command":"other"}}}` + "\n"
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 6, time.UTC) }
	options := JSONMergeOptions{
		Root: root, Target: target, KeyPath: []string{"mcpServers"}, ServerName: "specd",
		Server: map[string]any{"command": "specd", "args": []string{"mcp", "--root", root}},
		Now:    now,
	}
	first, err := MergeJSONServer(options)
	if err != nil {
		t.Fatal(err)
	}
	if !first.Changed || first.Backup == "" {
		t.Fatalf("first merge = %#v", first)
	}
	backup, err := os.ReadFile(first.Backup)
	if err != nil {
		t.Fatal(err)
	}
	if string(backup) != original {
		t.Fatalf("backup changed content: %q", backup)
	}
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]any
	if err := json.Unmarshal(data, &document); err != nil {
		t.Fatal(err)
	}
	if document["theme"] != "dark" {
		t.Fatalf("unrelated value lost: %#v", document)
	}
	servers := document["mcpServers"].(map[string]any)
	if servers["other"] == nil || servers["specd"] == nil {
		t.Fatalf("server entries = %#v", servers)
	}
	second, err := MergeJSONServer(options)
	if err != nil {
		t.Fatal(err)
	}
	if second.Changed || second.Backup != "" {
		t.Fatalf("idempotent merge changed file: %#v", second)
	}
}

func TestJSONMergeInvalidJSONWritesNothing(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config.json")
	if err := os.WriteFile(target, []byte("{broken"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := MergeJSONServer(JSONMergeOptions{
		Root: root, Target: target, KeyPath: []string{"mcpServers"},
		ServerName: "specd", Server: map[string]any{"command": "specd"},
	})
	if err == nil {
		t.Fatal("invalid JSON accepted")
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != "{broken" {
		t.Fatalf("invalid JSON file changed: %q", data)
	}
}

func TestJSONMergeRejectsTrailingDocument(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "config.json")
	original := "{}\n{}\n"
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := MergeJSONServer(JSONMergeOptions{
		Root: root, Target: target, KeyPath: []string{"mcpServers"},
		ServerName: "specd", Server: map[string]any{"command": "specd"},
	})
	if err == nil {
		t.Fatal("trailing JSON document accepted")
	}
	data, readErr := os.ReadFile(target)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(data) != original {
		t.Fatalf("invalid JSON file changed: %q", data)
	}
}

func TestJSONMergeRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	link := filepath.Join(root, ".cursor")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	target := filepath.Join(link, "mcp.json")
	_, err := MergeJSONServer(JSONMergeOptions{
		Root: root, Target: target, KeyPath: []string{"mcpServers"},
		ServerName: "specd", Server: map[string]any{"command": "specd"},
	})
	if err == nil {
		t.Fatal("symlink escape accepted")
	}
	if _, err := os.Stat(filepath.Join(outside, "mcp.json")); !os.IsNotExist(err) {
		t.Fatalf("outside target was written: %v", err)
	}
}
