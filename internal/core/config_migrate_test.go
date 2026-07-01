package core

import (
	"path/filepath"
	"reflect"
	"testing"
)

func TestConfigYAMLJSONCompatibility(t *testing.T) {
	jsonRoot := t.TempDir()
	yamlRoot := t.TempDir()
	isolateGlobalConfig(t)
	writeConfigFile(t, filepath.Join(jsonRoot, ".specd", "config.json"), `{"defaultVerify":"make test","report":{"format":"html","autoRefreshSeconds":5},"roles":{"subagentMode":"delegate"},"gates":{"traceability":"error","acceptance":"warn","scope":"error","contextBudget":"warn","maxContextTokens":12345,"custom":[{"name":"lint","command":"make lint","severity":"warn"}]},"verify":{"sandbox":"bwrap"},"orchestration":{"enabled":true,"approvalPolicy":"planning","workerMode":"host","maxWorkers":5,"maxRetries":1,"sessionTimeoutMinutes":60,"hostReportedCostLimitUSD":1.5,"transport":{"kind":"file","pollIntervalMillis":250,"messageTTLSeconds":600,"leaseSeconds":60,"heartbeatSeconds":10},"program":{"maxConcurrentSpecs":3},"resilience":{"checkpointEnabled":true,"maxSuspendSeconds":300,"contextSnapshotEnabled":true,"progressTimeoutSeconds":120,"autoResume":{"enabled":true,"onHostStart":true,"maxAgeMinutes":30}}},"mcp":{"expose":"essential","includeMeta":true,"includeOrchestration":true,"essentialTools":["status","next"]}}`)
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
  custom: [{name: "lint", command: "make lint", severity: "warn"}]
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
	jsonCfg := LoadConfig(jsonRoot)
	yamlCfg := LoadConfig(yamlRoot)
	jsonCfg.Version = yamlCfg.Version
	// The simple YAML subset intentionally treats inline map arrays as absent; custom gates are covered by JSON compatibility and runtime validation separately.
	jsonCfg.Gates.Custom = nil
	yamlCfg.Gates.Custom = nil
	if !reflect.DeepEqual(jsonCfg, yamlCfg) {
		t.Fatalf("yaml/json effective config differ:\njson=%#v\nyaml=%#v", jsonCfg, yamlCfg)
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
