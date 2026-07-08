package core

import (
	"path/filepath"
	"testing"
)

// TestConfigYAMLLoadsFullConfig loads a full, every-section YAML config and
// spot-checks representative fields across each block. Config is YAML-only as of
// v0.2.0; a leftover config.json is rejected rather than parsed.
func TestConfigYAMLLoadsFullConfig(t *testing.T) {
	yamlRoot := t.TempDir()
	isolateGlobalConfig(t)
	writeConfigFile(t, filepath.Join(yamlRoot, ".specd", "config.yml"), `version: 2
defaults:
  verify_command: "make test"
report:
  format: "html"
  auto_refresh_seconds: 5
roles:
  subagent_mode: "delegate"
gates:
  traceability: "error"
  acceptance: "warn"
  scope: "error"
  context_budget: "warn"
  max_context_tokens: 12345
verify:
  sandbox: "bwrap"
orchestration:
  enabled: true
  approval_policy: "planning"
  worker_mode: "host"
  max_workers: 5
  max_retries: 1
  session_timeout_minutes: 60
  host_reported_cost_limit_usd: 1.5
  transport:
    kind: "file"
    poll_interval_millis: 250
    message_ttl_seconds: 600
    lease_seconds: 60
    heartbeat_seconds: 10
  program:
    max_concurrent_specs: 3
  resilience:
    checkpoint_enabled: true
    max_suspend_seconds: 300
    context_snapshot_enabled: true
    progress_timeout_seconds: 120
    auto_resume:
      enabled: true
      on_host_start: true
      max_age_minutes: 30
mcp:
  expose: "essential"
  include_meta: true
  include_orchestration: true
  essential_tools: [status, next]
`)
	cfg := LoadConfig(yamlRoot)
	if cfg.DefaultVerify != "make test" {
		t.Errorf("DefaultVerify = %q, want make test", cfg.DefaultVerify)
	}
	if cfg.Report != (ReportCfg{Format: "html", AutoRefreshSeconds: 5}) {
		t.Errorf("Report = %#v", cfg.Report)
	}
	if cfg.Roles != (RolesCfg{SubagentMode: "delegate"}) {
		t.Errorf("Roles = %#v", cfg.Roles)
	}
	if cfg.Gates.Traceability != "error" || cfg.Gates.MaxContextTokens != 12345 {
		t.Errorf("Gates = %#v", cfg.Gates)
	}
	if cfg.Verify != (VerifyCfg{Sandbox: "bwrap"}) {
		t.Errorf("Verify = %#v", cfg.Verify)
	}
	if !cfg.Orchestration.Enabled || cfg.Orchestration.MaxWorkers != 5 ||
		cfg.Orchestration.Transport.LeaseSeconds != 60 ||
		!cfg.Orchestration.Resilience.AutoResume.OnHostStart {
		t.Errorf("Orchestration = %#v", cfg.Orchestration)
	}
	if cfg.MCP.Expose != "essential" || !cfg.MCP.IncludeMeta {
		t.Errorf("MCP = %#v", cfg.MCP)
	}

	// Legacy JSON config is no longer parsed — it must be rejected, not applied.
	jsonRoot := t.TempDir()
	writeConfigFile(t, filepath.Join(jsonRoot, ".specd", "config.json"), `{"defaultVerify":"make test"}`)
	if _, res := LoadConfigWithDiagnostics(jsonRoot); !hasDiagError(res.Diagnostics) {
		t.Fatalf("legacy JSON config should be rejected; diagnostics=%+v", res.Diagnostics)
	}
}

func TestEnsureGlobalConfigScaffold(t *testing.T) {
	xdg := filepath.Join(t.TempDir(), "xdg")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
	result, err := EnsureGlobalConfigScaffold(func(string) (string, error) { return "version: 2\n", nil })
	if err != nil || !result.Created || filepath.Ext(result.Path) != ".yml" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
	again, err := EnsureGlobalConfigScaffold(func(string) (string, error) { return "bad", nil })
	if err != nil || again.Created || again.Path != result.Path {
		t.Fatalf("again=%#v err=%v", again, err)
	}
}
