package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestCheckSchemaOnlySkipsContentGates(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "init", nil, map[string]string{"agent": "codex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() error {
		return Run(root, "new", []string{"demo"}, nil)
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() error {
		return Run(root, "check", []string{"demo"}, map[string]string{"schema-only": "true"})
	}); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSchemaOnlyRejectsUnknownStateField(t *testing.T) {
	root := t.TempDir()
	if err := Run(root, "init", nil, map[string]string{"agent": "codex"}); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() error {
		return Run(root, "new", []string{"demo"}, nil)
	}); err != nil {
		t.Fatal(err)
	}
	path := core.StatePath(root, "demo")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	raw = []byte(strings.Replace(string(raw), "\n}", ",\n  \"unexpected\": true\n}", 1))
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() error {
		return Run(root, "check", []string{"demo"}, map[string]string{"schema-only": "true"})
	}); err == nil {
		t.Fatal("check --schema-only accepted unknown state field")
	}
}
