package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigPathResolvers(t *testing.T) {
	root := t.TempDir()
	want := []string{
		filepath.Join(root, ".specd", "config.yml"),
		filepath.Join(root, ".specd", "config.yaml"),
		filepath.Join(root, ".specd", "config.json"),
	}
	got := ConfigPaths(root)
	if len(got) != len(want) {
		t.Fatalf("ConfigPaths len = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ConfigPaths[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if ConfigPath(root) != LegacyConfigPath(root) || filepath.Base(ConfigPath(root)) != "config.json" {
		t.Fatalf("ConfigPath must remain legacy JSON path, got %q", ConfigPath(root))
	}
	if got := GlobalConfigPaths(); len(got) == 0 {
		t.Fatalf("GlobalConfigPaths() is empty")
	}
}

func TestConfigLoaderYAMLV2DefaultsNamespace(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".specd", "config.yml"), []byte(`version: 2
defaults:
  verify_command: "go test ./..."
  report_format: html
  subagent_mode: delegate
  promotion_threshold: 7
orchestration:
  enabled: true
  max_workers: 2
  transport:
    poll_interval_millis: 250
mcp:
  expose: essential
  essential_tools: [status, next]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, result := LoadConfigWithDiagnostics(root)
	if HasErrorDiagnostics(result.Diagnostics) {
		t.Fatalf("unexpected diagnostics: %#v", result.Diagnostics)
	}
	if cfg.DefaultVerify != "go test ./..." || cfg.Report.Format != "html" || cfg.Roles.SubagentMode != "delegate" || cfg.PromotionThreshold != 7 {
		t.Fatalf("v2 defaults not mapped: %#v", cfg)
	}
	if !cfg.Orchestration.Enabled || cfg.Orchestration.MaxWorkers != 2 || cfg.Orchestration.Transport.PollIntervalMillis != 250 {
		t.Fatalf("snake_case orchestration not mapped: %#v", cfg.Orchestration)
	}
	if cfg.MCP.Expose != "essential" || len(cfg.MCP.EssentialTools) != 2 {
		t.Fatalf("snake_case mcp not mapped: %#v", cfg.MCP)
	}
}

func TestConfigCandidatePriorityAndCascade(t *testing.T) {
	root := t.TempDir()
	projectDir := filepath.Join(root, ".specd")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	globalDir := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("XDG_CONFIG_HOME", globalDir)
	if err := os.MkdirAll(filepath.Join(globalDir, "specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(globalDir, "specd", "config.yml"), []byte("defaults:\n  verify_command: global\norchestration:\n  max_workers: 3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "config.json"), []byte(`{"defaultVerify":"legacy"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(projectDir, "config.yml"), []byte("defaults:\n  report_format: html\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, result := LoadConfigWithDiagnostics(root)
	if cfg.DefaultVerify != "global" || cfg.Report.Format != "html" || cfg.Orchestration.MaxWorkers != 3 {
		t.Fatalf("cascade mismatch: %#v", cfg)
	}
	if filepath.Base(result.ProjectPath) != "config.yml" || filepath.Base(result.GlobalPath) != "config.yml" {
		t.Fatalf("selected paths = project %q global %q", result.ProjectPath, result.GlobalPath)
	}
	foundIgnored := false
	for _, d := range result.Diagnostics {
		if d.Severity == "warning" && filepath.Base(d.Path) == "config.json" {
			foundIgnored = true
		}
	}
	if !foundIgnored {
		t.Fatalf("expected ignored legacy JSON diagnostic, got %#v", result.Diagnostics)
	}
}

func TestConfigLoaderLegacyJSONStillWorks(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ConfigPath(root), []byte(`{"defaultVerify":"go test ./...","roles":{"subagentMode":"delegate"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := LoadConfig(root)
	if cfg.DefaultVerify != "go test ./..." || cfg.Roles.SubagentMode != "delegate" {
		t.Fatalf("legacy JSON not preserved: %#v", cfg)
	}
}
