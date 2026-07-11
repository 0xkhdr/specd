package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestInitWritesBoundedProjectConfig pins R3: a fresh `specd init` scaffolds a
// project.yml whose active verify.timeout_seconds bound is visible and parseable,
// and a second init never clobbers an operator-edited file.
func TestInitWritesBoundedProjectConfig(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	path := filepath.Join(root, "project.yml")
	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("project.yml not written: %v", err)
	}
	if !strings.Contains(string(body), "timeout_seconds: 600") {
		t.Fatalf("project.yml missing active bound:\n%s", body)
	}

	// The scaffolded template must parse and yield the bound — guards the template
	// against drifting out of parseSimpleYAML's supported subset.
	cfg, diags := core.LoadConfig(core.ConfigPaths{Project: path}, nil)
	if len(diags) != 0 {
		t.Fatalf("scaffolded project.yml did not parse cleanly: %#v", diags)
	}
	if cfg.Verify.TimeoutSecs != 600 {
		t.Fatalf("verify.timeout_seconds = %d, want 600", cfg.Verify.TimeoutSecs)
	}

	// Idempotent: a second init preserves an operator-edited file byte-for-byte.
	edited := "verify:\n  timeout_seconds: 42\n"
	if err := os.WriteFile(path, []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("second init: %v", err)
	}
	if got, _ := os.ReadFile(path); string(got) != edited {
		t.Fatalf("second init clobbered operator config:\n%s", got)
	}
}

// TestInitScaffoldGuidanceParity pins spec 01 R6.2: the scaffolded AGENTS.md
// points agents at the machine guidance surface and keeps approval human-only —
// it never tells an agent to self-approve.
func TestInitScaffoldGuidanceParity(t *testing.T) {
	root := t.TempDir()
	if err := runInit(root, nil, map[string]string{}); err != nil {
		t.Fatalf("init: %v", err)
	}
	body, err := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if err != nil {
		t.Fatalf("AGENTS.md not written: %v", err)
	}
	agents := string(body)
	if !strings.Contains(agents, "status <slug> --guide") {
		t.Fatalf("scaffold does not point agents at machine guidance:\n%s", agents)
	}
	if !strings.Contains(agents, "human-only") || !strings.Contains(agents, "self-approve") {
		t.Fatalf("scaffold must mark approval human-only and forbid self-approval:\n%s", agents)
	}
}
