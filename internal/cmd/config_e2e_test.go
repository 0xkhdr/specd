package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/integration"
)

func TestConfigInitDoctorMigrateE2E(t *testing.T) {
	t.Run("init_creates_project_and_global_yaml", func(t *testing.T) {
		root := initTestRoot(t)
		xdg := filepath.Join(t.TempDir(), "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)
		t.Setenv("HOME", filepath.Join(t.TempDir(), "home"))
		_, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--agent", "none"}), core.DefaultInitExecutor())
		if code != core.ExitOK || stderr != "" {
			t.Fatalf("exit=%d stderr=%q", code, stderr)
		}
		for _, p := range []string{filepath.Join(root, ".specd", "config.yml"), filepath.Join(xdg, "specd", "config.yml")} {
			if _, err := os.Stat(p); err != nil {
				t.Fatalf("missing YAML config %s: %v", p, err)
			}
		}
	})

	t.Run("doctor_json_reports_env_diagnostics_without_ansi", func(t *testing.T) {
		initTestRoot(t)
		if _, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--agent", "none"}), core.DefaultInitExecutor()); code != core.ExitOK || stderr != "" {
			t.Fatalf("init exit=%d stderr=%q", code, stderr)
		}
		t.Setenv("SPECD_VERIFY_SANDBOX", "bwrap")
		runtime := doctorRuntime{Registry: integration.MustRegistry(), Probe: passingProbe}
		stdout, stderr, code := captureOutput(t, func() int { return runDoctor(cli.ParseArgs([]string{"--json"}), runtime) })
		if code != core.ExitOK || stderr != "" || strings.Contains(stdout, "\x1b[") {
			t.Fatalf("exit=%d stderr=%q stdout=%q", code, stderr, stdout)
		}
		var result doctorResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range result.ConfigDiagnostics {
			if d.Source == "SPECD_VERIFY_SANDBOX" && d.Field == "verify.sandbox" {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing env diagnostic: %#v", result.ConfigDiagnostics)
		}
	})

	t.Run("migrate_converts_legacy_json_and_writes_backup", func(t *testing.T) {
		root := initTestRoot(t)
		if err := os.MkdirAll(filepath.Join(root, ".specd"), 0o755); err != nil {
			t.Fatal(err)
		}
		legacy := filepath.Join(root, ".specd", "config.json")
		if err := os.WriteFile(legacy, []byte(`{"defaultVerify":"go test ./...","orchestration":{"enabled":true,"approvalPolicy":"planning"}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		stdout, stderr, code := captureOutput(t, func() int { return RunMigrate(cli.ParseArgs([]string{"config", "--json"})) })
		if code != core.ExitOK || stderr != "" || strings.Contains(stdout, "\x1b[") {
			t.Fatalf("exit=%d stderr=%q stdout=%q", code, stderr, stdout)
		}
		if _, err := os.Stat(filepath.Join(root, ".specd", "config.yml")); err != nil {
			t.Fatalf("missing migrated YAML: %v", err)
		}
		if _, err := os.Stat(legacy + ".bak"); err != nil {
			t.Fatalf("missing backup: %v", err)
		}
		cfg := core.LoadConfig(root)
		if cfg.DefaultVerify != "go test ./..." || cfg.Orchestration.ApprovalPolicy != "planning" {
			t.Fatalf("migrated config mismatch: %#v", cfg)
		}
	})
}
