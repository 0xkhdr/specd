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

func TestConfigSecurityProfile(t *testing.T) {
	if DefaultConfig.Security.Profile != "prototype" {
		t.Fatalf("default profile = %q", DefaultConfig.Security.Profile)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yml")
	os.WriteFile(path, []byte("security:\n  profile: production\n"), 0o644)
	cfg, diags := LoadConfig(ConfigPaths{Project: path}, nil)
	if len(diags) != 0 || cfg.Security.Profile != "production" {
		t.Fatalf("cfg=%+v diags=%v", cfg.Security, diags)
	}
	os.WriteFile(path, []byte("security:\n  profile: unsafe\n"), 0o644)
	_, diags = LoadConfig(ConfigPaths{Project: path}, nil)
	if len(diags) != 1 {
		t.Fatalf("invalid profile diagnostics=%v", diags)
	}
}

func TestConfigRoutingPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yml")
	raw := strings.Join([]string{
		"routing:",
		"  version: 1",
		"  classes: basic,reviewed,sandboxed",
		"  fallback: sandboxed,reviewed,basic",
		"  class_capabilities: basic=context;reviewed=context+review;sandboxed=context+review+sandbox+eval",
		"  max_tokens: 50000",
		"  max_cost_micros: 250000",
		"  deadline_seconds: 900",
		"  max_retries: 2",
		"  allow_unknown_telemetry: false",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, diags := LoadConfig(ConfigPaths{Project: path}, nil)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %#v", diags)
	}
	if cfg.Routing.Version != "1" || cfg.Routing.MaxTokens != 50000 || cfg.Routing.MaxCostMicros != 250000 || cfg.Routing.DeadlineSeconds != 900 || cfg.Routing.MaxRetries != 2 || cfg.Routing.AllowUnknownTelemetry {
		t.Fatalf("routing = %#v", cfg.Routing)
	}
	if got := cfg.Routing.ClassCapabilities["sandboxed"]; !reflect.DeepEqual(got, []string{"context", "eval", "review", "sandbox"}) {
		t.Fatalf("sandboxed capabilities = %#v", got)
	}
}

// TestEnvPolicy pins R7.1: closed environment policy loads per-environment
// strategy/approver/authority/criteria/window/freshness/rollback, and an unknown
// environment name or a missing/invalid required field fails closed.
func TestEnvPolicy(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yml")
	good := strings.Join([]string{
		"environments:",
		"  staging: strategy=rolling;criteria=health;window=5m;freshness=2m;rollback=previous",
		"  production: strategy=canary;approver=release-manager;authority=oncall;criteria=health+latency;window=10m;freshness=5m;rollback=previous",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(good), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, diags := LoadConfig(ConfigPaths{Project: path}, nil)
	if len(diags) != 0 {
		t.Fatalf("diagnostics = %#v", diags)
	}
	prod, ok := cfg.Environments[EnvironmentProduction]
	if !ok {
		t.Fatal("production policy missing")
	}
	if prod.Schema != EnvironmentSchemaV1 || prod.Name != EnvironmentProduction {
		t.Fatalf("schema/name = %q/%q", prod.Schema, prod.Name)
	}
	if prod.Strategy != "canary" || prod.RequiredApprover != "release-manager" || prod.RequiredAuthority != "oncall" ||
		prod.ObservationWindow != "10m" || prod.Freshness != "5m" || prod.RollbackTarget != "previous" {
		t.Fatalf("production policy = %#v", prod)
	}
	if !reflect.DeepEqual(prod.HealthCriteria, []string{"health", "latency"}) {
		t.Fatalf("criteria = %#v", prod.HealthCriteria)
	}

	for _, raw := range []string{
		"environments:\n  qa: strategy=rolling;criteria=health;window=5m;freshness=2m;rollback=previous\n",         // unknown env
		"environments:\n  production: strategy=canary;criteria=health;window=10m;freshness=5m\n",                   // missing rollback
		"environments:\n  production: strategy=canary;criteria=health;window=nope;freshness=5m;rollback=prev\n",    // bad duration
		"environments:\n  production: strategy=canary;window=10m;freshness=5m;rollback=prev\n",                     // missing criteria
		"environments:\n  production: strategy=canary;criteria=health;window=10m;freshness=5m;rollback=prev;x=y\n", // unknown field
	} {
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, diags := LoadConfig(ConfigPaths{Project: path}, nil); len(diags) == 0 {
			t.Fatalf("policy %q accepted, want fail-closed diagnostic", raw)
		}
	}
}

func TestConfigRoutingSafeDefaultsAndValidation(t *testing.T) {
	if DefaultConfig.Routing.Version != "1" || !DefaultConfig.Routing.AllowUnknownTelemetry || DefaultConfig.Routing.DefaultClass == "" {
		t.Fatalf("unsafe routing defaults = %#v", DefaultConfig.Routing)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "project.yml")
	for _, raw := range []string{
		"routing:\n  version: 2\n",
		"routing:\n  classes: basic,basic\n",
		"routing:\n  fallback: missing\n",
		"routing:\n  max_cost_micros: -1\n",
		"routing:\n  class_capabilities: malformed\n",
	} {
		if err := os.WriteFile(path, []byte(raw), 0o644); err != nil {
			t.Fatal(err)
		}
		_, diags := LoadConfig(ConfigPaths{Project: path}, nil)
		if len(diags) == 0 {
			t.Fatalf("config %q accepted, want diagnostic", raw)
		}
	}
}
