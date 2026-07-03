package core

import (
	"path/filepath"
	"testing"
)

func TestConfigPrecedenceLadder(t *testing.T) {
	root := t.TempDir()
	global := isolateGlobalConfig(t)
	writeConfigFile(t, global, `defaults:
  verify_command: global
orchestration:
  enabled: true
  max_workers: 2
  max_retries: 1
mcp:
  essential_tools: [status, next]
`)
	writeConfigFile(t, filepath.Join(root, ".specd", "config.yml"), `defaults:
  verify_command: project
orchestration:
  enabled: false
  max_workers: 0
mcp:
  essential_tools: [doctor]
`)
	t.Setenv("SPECD_DEFAULT_VERIFY", "env")
	t.Setenv("SPECD_ORCHESTRATION_MAX_WORKERS", "6")

	cfg, result := LoadConfigWithDiagnostics(root)
	if HasErrorDiagnostics(result.Diagnostics) {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
	if cfg.Version != DefaultConfig.Version {
		t.Fatalf("embedded default version lost: %d", cfg.Version)
	}
	if cfg.DefaultVerify != "env" {
		t.Fatalf("env should beat project/global, got %q", cfg.DefaultVerify)
	}
	if cfg.Orchestration.Enabled {
		t.Fatalf("explicit project false should beat global true")
	}
	if cfg.Orchestration.MaxWorkers != 6 {
		t.Fatalf("env maxWorkers should beat explicit project zero after clamping, got %d", cfg.Orchestration.MaxWorkers)
	}
	if cfg.Orchestration.MaxRetries != 1 {
		t.Fatalf("global field should survive absent project field, got %d", cfg.Orchestration.MaxRetries)
	}
	if len(cfg.MCP.EssentialTools) != 1 || cfg.MCP.EssentialTools[0] != "doctor" {
		t.Fatalf("list fields should be replaced by higher layer, got %#v", cfg.MCP.EssentialTools)
	}
}
