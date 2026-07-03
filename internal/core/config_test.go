package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigCascade(t *testing.T) {
	dir := t.TempDir()
	global := filepath.Join(dir, "global.yml")
	project := filepath.Join(dir, "project.yml")
	if err := os.WriteFile(global, []byte(strings.Join([]string{
		"version: 1",
		"agent: claude",
		"gates:",
		"  verify: warn",
		"context:",
		"  max_tokens: 1000",
		"orchestration:",
		"  enabled: true",
		"  model: global-model",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(project, []byte(strings.Join([]string{
		"agent: codex",
		"context:",
		"  max_tokens: 2000",
		"orchestration:",
		"  api_key: should-not-apply",
		"  model: project-model",
		"",
	}, "\n")), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, diagnostics := LoadConfig(ConfigPaths{Global: global, Project: project}, map[string]string{
		"SPECD_GATES_VERIFY":       "error",
		"SPECD_CONTEXT_MAX_TOKENS": "3000",
	})

	if cfg.Agent != "codex" {
		t.Fatalf("agent = %q, want project override codex", cfg.Agent)
	}
	if cfg.Gates.Verify != "error" {
		t.Fatalf("gates.verify = %q, want env override error", cfg.Gates.Verify)
	}
	if cfg.Context.MaxTokens != 3000 {
		t.Fatalf("context.max_tokens = %d, want env override 3000", cfg.Context.MaxTokens)
	}
	if !cfg.Orchestration.Enabled {
		t.Fatal("orchestration.enabled = false, want global true")
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
	if cfg != DefaultConfig {
		t.Fatalf("legacy config changed cfg = %#v, want default %#v", cfg, DefaultConfig)
	}
}
