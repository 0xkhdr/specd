package core

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfigCascade(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project.yml")
	if err := os.WriteFile(project, []byte(strings.Join([]string{
		"agent: codex",
		"gates:",
		"  verify: warn",
		"context:",
		"  max_tokens: 2000",
		"orchestration:",
		"  enabled: true",
		"  api_key: should-not-apply",
		"  model: project-model",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, diagnostics := LoadConfig(ConfigPaths{Project: project}, map[string]string{
		"SPECD_GATES_VERIFY":       "error",
		"SPECD_CONTEXT_MAX_TOKENS": "3000",
	})

	if cfg.Agent != "codex" {
		t.Fatalf("agent = %q, want project codex", cfg.Agent)
	}
	if cfg.Gates.Verify != "error" {
		t.Fatalf("gates.verify = %q, want env override error", cfg.Gates.Verify)
	}
	if cfg.Context.MaxTokens != 3000 {
		t.Fatalf("context.max_tokens = %d, want env override 3000", cfg.Context.MaxTokens)
	}
	if !cfg.Orchestration.Enabled {
		t.Fatal("orchestration.enabled = false, want project true")
	}
	if cfg.Orchestration.Model != "project-model" {
		t.Fatalf("orchestration.model = %q, want project-model", cfg.Orchestration.Model)
	}
	if len(diagnostics) != 1 || !strings.Contains(diagnostics[0].Message, "secret value not allowed") {
		t.Fatalf("diagnostics = %#v, want secret scrub diagnostic", diagnostics)
	}

	bad := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(bad, []byte("agent codex\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diagnostics = LoadConfig(ConfigPaths{Project: bad}, nil)
	if len(diagnostics) != 1 || diagnostics[0].Severity != "error" {
		t.Fatalf("bad yaml diagnostics = %#v, want fail-loud error", diagnostics)
	}
}

func TestConfigNoLegacyJSON(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "config.json")
	if err := os.WriteFile(legacy, []byte(`{"agent":"legacy"}`), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, diagnostics := LoadConfig(ConfigPaths{Project: legacy}, nil)
	if len(diagnostics) != 0 {
		t.Fatalf("legacy json diagnostics = %#v, want ignored", diagnostics)
	}
	if !reflect.DeepEqual(cfg, DefaultConfig) {
		t.Fatalf("legacy config changed cfg = %#v, want default %#v", cfg, DefaultConfig)
	}
}

// TestVerifyTimeoutConfig pins the verify.timeout_seconds key (gap 4.2): a valid
// value parses onto Verify.TimeoutSecs, a negative value is a loud error, and env
// overrides project.
func TestVerifyTimeoutConfig(t *testing.T) {
	dir := t.TempDir()
	project := filepath.Join(dir, "project.yml")
	if err := os.WriteFile(project, []byte("verify:\n  timeout_seconds: 30\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, diags := LoadConfig(ConfigPaths{Project: project}, nil)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %#v, want none", diags)
	}
	if cfg.Verify.TimeoutSecs != 30 {
		t.Fatalf("verify.timeout_seconds = %d, want 30", cfg.Verify.TimeoutSecs)
	}
	cfg, _ = LoadConfig(ConfigPaths{Project: project}, map[string]string{"SPECD_VERIFY_TIMEOUT_SECONDS": "5"})
	if cfg.Verify.TimeoutSecs != 5 {
		t.Fatalf("env override verify.timeout_seconds = %d, want 5", cfg.Verify.TimeoutSecs)
	}

	bad := filepath.Join(dir, "bad.yml")
	if err := os.WriteFile(bad, []byte("verify:\n  timeout_seconds: -1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, diags = LoadConfig(ConfigPaths{Project: bad}, nil)
	if len(diags) != 1 || !strings.Contains(diags[0].Message, "verify.timeout_seconds") {
		t.Fatalf("negative timeout diagnostics = %#v, want one verify.timeout_seconds error", diags)
	}
}
