package core

import (
	"os"
	"path/filepath"
	"testing"
)

func writeConfigFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func isolateGlobalConfig(t *testing.T) string {
	t.Helper()
	xdg := filepath.Join(t.TempDir(), "xdg")
	home := filepath.Join(t.TempDir(), "home")
	t.Setenv("XDG_CONFIG_HOME", xdg)
	t.Setenv("HOME", home)
	return filepath.Join(xdg, "specd", "config.yml")
}

func TestConfigEnvOverridesAfterCascade(t *testing.T) {
	root := t.TempDir()
	global := isolateGlobalConfig(t)
	writeConfigFile(t, global, "defaults:\n  verify_command: global\norchestration:\n  max_workers: 2\ngates:\n  context_budget: warn\n")
	writeConfigFile(t, filepath.Join(root, ".specd", "config.yml"), "defaults:\n  verify_command: project\norchestration:\n  max_workers: 3\ngates:\n  context_budget: error\n")
	t.Setenv("SPECD_DEFAULT_VERIFY", "env")
	t.Setenv("SPECD_ORCHESTRATION_MAX_WORKERS", "9")
	t.Setenv("SPECD_GATES_CONTEXT_BUDGET", "off")

	cfg, result := LoadConfigWithDiagnostics(root)
	if cfg.DefaultVerify != "env" || cfg.Orchestration.MaxWorkers != 9 || cfg.Gates.ContextBudget != "off" {
		t.Fatalf("env did not win after cascade: %#v", cfg)
	}
	assertEnvDiag(t, result.Diagnostics, "SPECD_DEFAULT_VERIFY", "defaultVerify")
	assertEnvDiag(t, result.Diagnostics, "SPECD_ORCHESTRATION_MAX_WORKERS", "orchestration.maxWorkers")
}

func TestConfigEnvInvalidIsStrictDiagnostic(t *testing.T) {
	root := t.TempDir()
	isolateGlobalConfig(t)
	writeConfigFile(t, filepath.Join(root, ".specd", "config.yml"), "defaults:\n  verify_command: project\n")
	t.Setenv("SPECD_VERIFY_SANDBOX", "shell")
	t.Setenv("SPECD_ORCHESTRATION_MAX_WORKERS", "bogus")

	_, diags := LoadConfigStrict(root)
	if !HasErrorDiagnostics(diags) {
		t.Fatalf("expected strict env diagnostics, got %#v", diags)
	}
	assertEnvDiag(t, diags, "SPECD_ORCHESTRATION_MAX_WORKERS", "orchestration.maxWorkers")
	foundSandbox := false
	for _, d := range diags {
		if d.Field == "verify.sandbox" && d.Severity == "error" {
			foundSandbox = true
		}
	}
	if !foundSandbox {
		t.Fatalf("missing effective verify.sandbox diagnostic: %#v", diags)
	}
}

func TestConfigEnvOverlayCoversBooleansFloatsAndResilience(t *testing.T) {
	root := t.TempDir()
	isolateGlobalConfig(t)
	writeConfigFile(t, filepath.Join(root, ".specd", "config.yml"), `orchestration:
  resilience:
    checkpoint_enabled: false
    auto_resume:
      enabled: false
      max_age_minutes: 1
`)
	t.Setenv("SPECD_ORCHESTRATION_ENABLED", "true")
	t.Setenv("SPECD_ORCHESTRATION_COMPACTION_BUDGET_THRESHOLD", "0.5")
	t.Setenv("SPECD_ORCHESTRATION_RESILIENCE_CHECKPOINT_ENABLED", "true")
	t.Setenv("SPECD_ORCHESTRATION_RESILIENCE_AUTO_RESUME_ENABLED", "true")
	t.Setenv("SPECD_ORCHESTRATION_RESILIENCE_AUTO_RESUME_MAX_AGE_MINUTES", "10")
	cfg, result := LoadConfigWithDiagnostics(root)
	if !cfg.Orchestration.Enabled || cfg.Orchestration.CompactionBudgetThreshold != 0.5 || cfg.Orchestration.Resilience == nil || !cfg.Orchestration.Resilience.CheckpointEnabled || !cfg.Orchestration.Resilience.AutoResume.Enabled || cfg.Orchestration.Resilience.AutoResume.MaxAgeMinutes != 10 {
		t.Fatalf("env overlay mismatch: %#v diagnostics=%#v", cfg.Orchestration, result.Diagnostics)
	}
}

func TestConfigEnvInvalidBoolAndFloatDiagnostics(t *testing.T) {
	root := t.TempDir()
	isolateGlobalConfig(t)
	t.Setenv("SPECD_ORCHESTRATION_ENABLED", "maybe")
	t.Setenv("SPECD_ORCHESTRATION_COMPACTION_BUDGET_THRESHOLD", "nan?")
	_, diags := LoadConfigStrict(root)
	assertEnvDiag(t, diags, "SPECD_ORCHESTRATION_ENABLED", "orchestration.enabled")
	assertEnvDiag(t, diags, "SPECD_ORCHESTRATION_COMPACTION_BUDGET_THRESHOLD", "orchestration.compactionBudgetThreshold")
}

func TestLoadConfigFromPathDiagnostics(t *testing.T) {
	root := t.TempDir()
	missing := filepath.Join(root, "missing.toml")
	if _, diags := LoadConfigFromPath(missing); len(diags) == 0 || diags[0].Severity != "info" {
		t.Fatalf("missing diagnostic = %#v", diags)
	}
	badYAML := filepath.Join(root, "config.yml")
	writeConfigFile(t, badYAML, " odd: indent\n")
	if _, diags := LoadConfigFromPath(badYAML); !HasErrorDiagnostics(diags) {
		t.Fatalf("bad yaml diagnostic = %#v", diags)
	}
	badExt := filepath.Join(root, "config.toml")
	writeConfigFile(t, badExt, "x=1")
	if _, diags := LoadConfigFromPath(badExt); !HasErrorDiagnostics(diags) {
		t.Fatalf("bad ext diagnostic = %#v", diags)
	}
}

func TestValidateEffectiveConfigReportsScalarErrors(t *testing.T) {
	cfg := DefaultConfig
	cfg.Report.Format = "pdf"
	cfg.Roles.SubagentMode = "remote"
	cfg.Gates.Traceability = "loud"
	cfg.Gates.MaxContextTokens = MaxSoftContextTokens() + 1
	cfg.Verify.Sandbox = "vm"
	cfg.Orchestration.WorkerMode = "agent"
	diags := ValidateEffectiveConfig(cfg)
	if len(diags) < 6 || !HasErrorDiagnostics(diags) {
		t.Fatalf("effective diagnostics = %#v", diags)
	}
}

func TestConfigFormatPreference(t *testing.T) {
	root := t.TempDir()
	isolateGlobalConfig(t)
	writeConfigFile(t, filepath.Join(root, ".specd", "config.yml"), "defaults:\n  verify_command: yaml\n")
	// yaml is the sole accepted preference (v0.2.0, YAML-only) and selects the
	// YAML config.
	t.Setenv("SPECD_CONFIG_FORMAT", "yaml")
	cfg, result := LoadConfigWithDiagnostics(root)
	if cfg.DefaultVerify != "yaml" || filepath.Base(result.ProjectPath) != "config.yml" {
		t.Fatalf("format preference not applied: cfg=%#v path=%q", cfg, result.ProjectPath)
	}
	// Any non-yaml preference (legacy json, toml) is unsupported and warns.
	for _, pref := range []string{"json", "toml"} {
		t.Setenv("SPECD_CONFIG_FORMAT", pref)
		_, result = LoadConfigWithDiagnostics(root)
		found := false
		for _, d := range result.Diagnostics {
			if d.Source == "SPECD_CONFIG_FORMAT" && d.Severity == "warning" {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing invalid format warning for %q: %#v", pref, result.Diagnostics)
		}
	}
}

func assertEnvDiag(t *testing.T, diags []ConfigDiagnostic, source, field string) {
	t.Helper()
	for _, d := range diags {
		if d.Layer == "env" && d.Source == source && d.Field == field {
			return
		}
	}
	t.Fatalf("missing env diagnostic %s -> %s in %#v", source, field, diags)
}
