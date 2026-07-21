package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigMigrationCLI(t *testing.T) {
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	legacy := filepath.Join(root, "project.yml")
	if err := os.WriteFile(legacy, []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	preview, err := captureStdout(t, func() error {
		return Run(root, "config", []string{"migrate"}, map[string]string{"dry-run": ""})
	})
	if err != nil || !strings.Contains(preview, `"action": "write-canonical"`) {
		t.Fatalf("preview=%s err=%v", preview, err)
	}
	if _, err := os.Stat(filepath.Join(root, ".specd", "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("CLI dry-run wrote a file")
	}
	result, err := captureStdout(t, func() error {
		return Run(root, "config", []string{"migrate"}, nil)
	})
	if err != nil || !strings.Contains(result, `"completed": true`) {
		t.Fatalf("migration=%s err=%v", result, err)
	}
	show, err := captureStdout(t, func() error { return Run(root, "config", []string{"show"}, nil) })
	if err != nil || !strings.Contains(show, `"selected_kind": "canonical"`) {
		t.Fatalf("show=%s err=%v", show, err)
	}
}
