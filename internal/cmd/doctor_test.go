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

func TestDoctorHealthFixAndJSON(t *testing.T) {
	t.Run("missing_scaffold_reports_deterministic_unhealthy_json", func(t *testing.T) {
		initTestRoot(t)
		runtime := doctorRuntime{Registry: integration.MustRegistry(), Probe: passingProbe}
		stdout, stderr, code := captureOutput(t, func() int {
			return runDoctor(cli.ParseArgs([]string{"--json"}), runtime)
		})
		if code != core.ExitGate || stderr != "" {
			t.Fatalf("exit=%d stderr=%q", code, stderr)
		}
		var result doctorResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		if result.Status != "unhealthy" || result.Checks == nil || result.Hosts == nil || result.Remediations == nil {
			t.Fatalf("result=%+v", result)
		}
	})

	t.Run("fix_repairs_scaffold_and_owned_project_registration", func(t *testing.T) {
		initTestRoot(t)
		host := &onboardingAdapter{name: "codex", detected: true, scopes: []integration.Scope{integration.ScopeProject}}
		runtime := doctorRuntime{Registry: integration.MustRegistry(host), Probe: passingProbe}
		_, stderr, code := captureOutput(t, func() int {
			return runDoctor(cli.ParseArgs([]string{"--fix", "--agent", "codex"}), runtime)
		})
		if code != core.ExitOK || stderr != "" || host.installs != 1 {
			t.Fatalf("exit=%d stderr=%q installs=%d", code, stderr, host.installs)
		}
	})

	t.Run("reports_orchestration_capability_separately_from_host_lifecycle", func(t *testing.T) {
		initTestRoot(t)
		host := &onboardingAdapter{name: "vscode", detected: true, scopes: []integration.Scope{integration.ScopeProject}, registered: true, owned: true}
		runtime := doctorRuntime{Registry: integration.MustRegistry(host), Probe: passingProbe}
		stdout, stderr, code := captureOutput(t, func() int {
			return runDoctor(cli.ParseArgs([]string{"--json", "--agent", "vscode"}), runtime)
		})
		if code != core.ExitGate || stderr != "" {
			t.Fatalf("exit=%d stderr=%q", code, stderr)
		}
		var result doctorResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		if result.Orchestration.ServerCapability != "available" {
			t.Fatalf("serverCapability=%q, want available", result.Orchestration.ServerCapability)
		}
		if strings.Join(result.Orchestration.Tools, ",") != "specd_brain,specd_pinky" {
			t.Fatalf("orchestration tools=%v", result.Orchestration.Tools)
		}
		if !strings.Contains(result.Orchestration.HostLifecycle, "does not spawn") {
			t.Fatalf("host lifecycle boundary missing spawn disclaimer: %q", result.Orchestration.HostLifecycle)
		}
		var mcpDetail string
		for _, check := range result.Checks {
			if check.Name == "mcp" {
				mcpDetail = check.Detail
			}
		}
		if !strings.Contains(mcpDetail, "specd_brain") || !strings.Contains(mcpDetail, "specd_pinky") {
			t.Fatalf("mcp detail missing orchestration tools: %q", mcpDetail)
		}
		if len(result.Hosts) != 1 || !result.Hosts[0].ReloadRequired || !result.Hosts[0].TrustRequired {
			t.Fatalf("host lifecycle flags=%+v", result.Hosts)
		}
		if !strings.Contains(result.Hosts[0].LifecycleSupport, "does not spawn Pinky agents") {
			t.Fatalf("host lifecycle support missing boundary: %q", result.Hosts[0].LifecycleSupport)
		}
	})

	// Spec A6, Req 2.2 — doctor must flag (not silently resolve) a dual
	// config.yml + config.json so a stale lower-priority file is never hidden.
	t.Run("flags_dual_config_file_conflict", func(t *testing.T) {
		root := initTestRoot(t)
		specd := filepath.Join(root, ".specd")
		if err := os.MkdirAll(specd, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(specd, "config.yml"), []byte("gates:\n  maxContextTokens: 7000\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(specd, "config.json"), []byte(`{"gates":{"maxContextTokens":1000}}`), 0o644); err != nil {
			t.Fatal(err)
		}
		runtime := doctorRuntime{Registry: integration.MustRegistry(), Probe: passingProbe}
		stdout, _, _ := captureOutput(t, func() int {
			return runDoctor(cli.ParseArgs([]string{"--json"}), runtime)
		})
		var result doctorResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		flagged := false
		for _, d := range result.ConfigDiagnostics {
			if strings.Contains(d.Message, "ignored lower-priority") && strings.HasSuffix(d.Source, "config.json") {
				flagged = true
			}
		}
		if !flagged {
			t.Fatalf("doctor did not flag dual-file conflict; diagnostics=%+v", result.ConfigDiagnostics)
		}
	})

	// Security-hardening spec (S1) Requirement 3 — a configured sandbox backend
	// whose dependency is missing must surface as an advisory finding, never as
	// a fatal one: SelectRunner already fails closed at verify time, so doctor
	// only explains why ahead of time and must still exit healthy/0 otherwise.
	t.Run("missing_sandbox_dependency_is_advisory_not_fatal", func(t *testing.T) {
		root := initTestRoot(t)
		runtime := doctorRuntime{Registry: integration.MustRegistry(), Probe: passingProbe}
		if _, stderr, code := captureOutput(t, func() int {
			return runDoctor(cli.ParseArgs([]string{"--fix"}), runtime)
		}); code != core.ExitOK || stderr != "" {
			t.Fatalf("baseline --fix: exit=%d stderr=%q", code, stderr)
		}

		// --fix scaffolds .specd/config.yml with `verify: sandbox: "none"`; flip
		// it to "bwrap" so the missing-dependency path is exercised.
		configPath := filepath.Join(root, ".specd", "config.yml")
		content, err := os.ReadFile(configPath)
		if err != nil {
			t.Fatal(err)
		}
		updated := strings.Replace(string(content), `sandbox: "none"`, `sandbox: "bwrap"`, 1)
		if updated == string(content) {
			t.Fatalf("config.yml did not contain expected sandbox: \"none\" line: %s", content)
		}
		if err := os.WriteFile(configPath, []byte(updated), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Setenv("PATH", t.TempDir()) // bwrap deliberately absent

		stdout, stderr, code := captureOutput(t, func() int {
			return runDoctor(cli.ParseArgs([]string{"--json"}), runtime)
		})
		if code != core.ExitOK || stderr != "" {
			t.Fatalf("exit=%d (want ExitOK — advisory must not gate), stderr=%q stdout=%s", code, stderr, stdout)
		}
		var result doctorResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatal(err)
		}
		if result.Status != "healthy" {
			t.Fatalf("status=%q, want healthy (advisory finding must not flip overall status)", result.Status)
		}
		var found *doctorCheck
		for i := range result.Checks {
			if result.Checks[i].Name == "verify-sandbox" {
				found = &result.Checks[i]
			}
		}
		if found == nil {
			t.Fatalf("missing verify-sandbox check: %+v", result.Checks)
		}
		if found.Status != "advisory" {
			t.Errorf("verify-sandbox status=%q, want advisory", found.Status)
		}
		if !strings.Contains(found.Detail, "bubblewrap") || !strings.Contains(found.Detail, "not on PATH") {
			t.Errorf("verify-sandbox detail=%q, want it to name the missing dependency", found.Detail)
		}
		if found.Remediation == "" {
			t.Errorf("verify-sandbox remediation empty, want an install hint")
		}
	})
}
