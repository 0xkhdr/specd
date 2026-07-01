package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

// TestInitSkillTemplatesExist asserts every skill in the scaffold manifest
// ships an embedded SKILL.md template with matching frontmatter.
func TestInitSkillTemplatesExist(t *testing.T) {
	for _, asset := range core.DefaultScaffoldManifest() {
		if !strings.HasPrefix(asset.Target, ".specd/skills/") {
			continue
		}
		content, err := core.ReadTemplate(asset.Template)
		if err != nil {
			t.Errorf("skill template %q is missing: %v", asset.Template, err)
			continue
		}
		parts := strings.Split(asset.Target, "/")
		name := parts[len(parts)-2]
		if !strings.Contains(content, "name:") {
			t.Errorf("skill template %q missing frontmatter name: key", asset.Template)
		}
		if !strings.Contains(content, "name: "+name) {
			t.Errorf("skill template %q frontmatter name does not match dir %q", asset.Template, name)
		}
	}
}

func TestInitRequiredWriteFailure(t *testing.T) {
	t.Run("human_output_fails_closed", func(t *testing.T) {
		root := initTestRoot(t)

		stdout, stderr, code := captureInitOutput(
			t,
			cli.Args{Flags: map[string]string{}},
			initWriteFailure(".specd/steering/reasoning.md"),
		)

		if code != core.ExitGate {
			t.Fatalf("exit = %d, want %d", code, core.ExitGate)
		}
		if !strings.Contains(stderr, ".specd/steering/reasoning.md") {
			t.Fatalf("failed path missing from stderr: %q", stderr)
		}
		if strings.Contains(strings.ToLower(stdout+stderr), "ready, 0 failed") {
			t.Fatalf("failure output claimed readiness: stdout=%q stderr=%q", stdout, stderr)
		}
		if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
			t.Fatalf("AGENTS.md should not be merged after required write failure: %v", err)
		}
	})

	t.Run("json_output_is_one_failed_result", func(t *testing.T) {
		initTestRoot(t)

		stdout, stderr, code := captureInitOutput(
			t,
			cli.Args{Flags: map[string]string{"json": "true"}},
			initWriteFailure(".specd/steering/reasoning.md"),
		)

		if code != core.ExitGate {
			t.Fatalf("exit = %d, want %d", code, core.ExitGate)
		}
		if stderr != "" {
			t.Fatalf("JSON mode wrote diagnostics to stderr: %q", stderr)
		}
		var result core.InitResult
		if err := json.Unmarshal([]byte(stdout), &result); err != nil {
			t.Fatalf("stdout is not one JSON document: %v\n%s", err, stdout)
		}
		if result.Status != "failed" {
			t.Fatalf("status = %q, want failed", result.Status)
		}
		if len(result.Files.Failed) != 1 || result.Files.Failed[0] != ".specd/steering/reasoning.md" {
			t.Fatalf("failed paths = %#v", result.Files.Failed)
		}
		if strings.Contains(stdout, "\x1b[") {
			t.Fatalf("JSON contains ANSI escapes: %q", stdout)
		}
	})
}

func initTestRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	previous, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(root); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(previous) })
	t.Setenv("NO_COLOR", "1")
	t.Setenv("SPECD_JSON", "")
	return root
}

func initWriteFailure(target string) core.InitExecutor {
	executor := core.DefaultInitExecutor()
	executor.WriteFile = func(path, content string) error {
		normalized := filepath.ToSlash(path)
		stageTarget := strings.TrimPrefix(target, ".specd/")
		if strings.HasSuffix(normalized, target) || strings.HasSuffix(normalized, "/"+stageTarget) {
			return errors.New("injected write failure")
		}
		return core.AtomicWrite(path, content)
	}
	return executor
}

func captureInitOutput(t *testing.T, args cli.Args, executor core.InitExecutor) (stdout, stderr string, code int) {
	t.Helper()
	originalOut, originalErr := os.Stdout, os.Stderr
	readOut, writeOut, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	readErr, writeErr, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout, os.Stderr = writeOut, writeErr
	defer func() { os.Stdout, os.Stderr = originalOut, originalErr }()

	outCh := make(chan string, 1)
	errCh := make(chan string, 1)
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, readOut)
		outCh <- b.String()
	}()
	go func() {
		var b bytes.Buffer
		_, _ = io.Copy(&b, readErr)
		errCh <- b.String()
	}()

	code = runInit(args, executor)
	_ = writeOut.Close()
	_ = writeErr.Close()
	stdout, stderr = <-outCh, <-errCh
	_ = readOut.Close()
	_ = readErr.Close()
	return stdout, stderr, code
}

func TestInitDryRunWritesNothingAndListsActions(t *testing.T) {
	root := initTestRoot(t)
	stdout, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--dry-run"}), core.DefaultInitExecutor())
	if code != core.ExitOK || stderr != "" {
		t.Fatalf("exit=%d stderr=%q", code, stderr)
	}
	if _, err := os.Stat(filepath.Join(root, ".specd")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote .specd: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); !os.IsNotExist(err) {
		t.Fatalf("dry-run wrote AGENTS.md: %v", err)
	}
	for _, want := range []string{"would write: .specd/config.yml", "would update: AGENTS.md"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("dry-run output missing %q:\n%s", want, stdout)
		}
	}
}

func TestInitRepairAndRefreshPreserveUserContent(t *testing.T) {
	root := initTestRoot(t)
	if _, _, code := captureInitOutput(t, cli.Args{Flags: map[string]string{}}, core.DefaultInitExecutor()); code != core.ExitOK {
		t.Fatalf("initial init exit=%d", code)
	}
	product := filepath.Join(root, ".specd", "steering", "product.md")
	role := filepath.Join(root, ".specd", "roles", "craftsman.md")
	agents := filepath.Join(root, "AGENTS.md")
	if err := os.WriteFile(product, []byte("user product\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(role); err != nil {
		t.Fatal(err)
	}
	currentAgents, _ := os.ReadFile(agents)
	if err := os.WriteFile(agents, []byte("preamble\n"+string(currentAgents)+"postamble\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--repair"}), core.DefaultInitExecutor()); code != core.ExitOK {
		t.Fatalf("repair exit=%d stderr=%q", code, stderr)
	}
	if got, _ := os.ReadFile(product); string(got) != "user product\n" {
		t.Fatalf("repair overwrote product.md: %q", got)
	}
	if _, err := os.Stat(role); err != nil {
		t.Fatalf("repair did not restore role: %v", err)
	}

	if _, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--refresh"}), core.DefaultInitExecutor()); code != core.ExitOK {
		t.Fatalf("refresh exit=%d stderr=%q", code, stderr)
	}
	if got, _ := os.ReadFile(product); string(got) != "user product\n" {
		t.Fatalf("refresh overwrote authored product.md: %q", got)
	}
	gotAgents, _ := os.ReadFile(agents)
	if !strings.HasPrefix(string(gotAgents), "preamble\n") || !strings.HasSuffix(string(gotAgents), "postamble\n") {
		t.Fatalf("refresh lost AGENTS.md user content:\n%s", gotAgents)
	}
}

func TestInitModeConflictReturnsUsage(t *testing.T) {
	initTestRoot(t)
	_, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--repair", "--refresh"}), core.DefaultInitExecutor())
	if code != core.ExitUsage {
		t.Fatalf("exit=%d want=%d", code, core.ExitUsage)
	}
	if !strings.Contains(stderr, "mutually exclusive") {
		t.Fatalf("stderr=%q", stderr)
	}
}

func TestInitRefreshRejectsMalformedAgentsWithoutWrite(t *testing.T) {
	root := initTestRoot(t)
	if err := os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("<!-- SPECD INIT: BEGIN v1 (do not edit between markers) -->\nbroken\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	_, _, code := captureInitOutput(t, cli.ParseArgs([]string{"--refresh"}), core.DefaultInitExecutor())
	if code != core.ExitGate {
		t.Fatalf("exit=%d want=%d", code, core.ExitGate)
	}
	after, _ := os.ReadFile(filepath.Join(root, "AGENTS.md"))
	if string(after) != string(before) {
		t.Fatal("malformed AGENTS.md changed")
	}
	if _, err := os.Stat(filepath.Join(root, ".specd")); !os.IsNotExist(err) {
		t.Fatalf("preflight failure wrote .specd: %v", err)
	}
}

func TestInitOrchestrationFlags(t *testing.T) {
	t.Run("bare_orchestration_defaults_to_planning", func(t *testing.T) {
		root := initTestRoot(t)
		_, _, code := captureInitOutput(t, cli.ParseArgs([]string{"--non-interactive", "--orchestration"}), core.DefaultInitExecutor())
		if code != core.ExitOK {
			t.Fatalf("exit = %d, want %d", code, core.ExitOK)
		}
		cfg := core.LoadConfig(root)
		if !cfg.Orchestration.Enabled || cfg.Orchestration.ApprovalPolicy != "planning" {
			t.Fatalf("unexpected orchestration config: %+v", cfg.Orchestration)
		}
		if cfg.Orchestration.MaxWorkers != 4 || cfg.Orchestration.MaxRetries != 2 || cfg.Orchestration.SessionTimeoutMinutes != 120 {
			t.Fatalf("unexpected orchestration defaults: %+v", cfg.Orchestration)
		}
	})

	t.Run("orchestration_session_full_autonomy_with_custom_workers", func(t *testing.T) {
		root := initTestRoot(t)
		_, _, code := captureInitOutput(t, cli.ParseArgs([]string{
			"--non-interactive",
			"--orchestration", "session",
			"--orchestration-workers", "8",
			"--orchestration-retries", "3",
			"--orchestration-timeout", "240",
			"--orchestration-cost-limit", "10.5",
			"--orchestration-mode", "delegate",
		}), core.DefaultInitExecutor())
		if code != core.ExitOK {
			t.Fatalf("exit = %d, want %d", code, core.ExitOK)
		}
		cfg := core.LoadConfig(root)
		if !cfg.Orchestration.Enabled || cfg.Orchestration.ApprovalPolicy != "session" {
			t.Fatalf("unexpected orchestration config: %+v", cfg.Orchestration)
		}
		if cfg.Orchestration.MaxWorkers != 8 {
			t.Fatalf("maxWorkers = %d, want 8", cfg.Orchestration.MaxWorkers)
		}
		if cfg.Orchestration.MaxRetries != 3 {
			t.Fatalf("maxRetries = %d, want 3", cfg.Orchestration.MaxRetries)
		}
		if cfg.Orchestration.SessionTimeoutMinutes != 240 {
			t.Fatalf("sessionTimeoutMinutes = %d, want 240", cfg.Orchestration.SessionTimeoutMinutes)
		}
		if cfg.Orchestration.HostReportedCostLimitUSD != 10.5 {
			t.Fatalf("hostReportedCostLimitUSD = %f, want 10.5", cfg.Orchestration.HostReportedCostLimitUSD)
		}
		if cfg.Roles.SubagentMode != "delegate" {
			t.Fatalf("roles.subagentMode = %q, want delegate", cfg.Roles.SubagentMode)
		}
	})

	t.Run("invalid_policy_returns_usage_exit", func(t *testing.T) {
		initTestRoot(t)
		_, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{"--non-interactive", "--orchestration", "foo"}), core.DefaultInitExecutor())
		if code != core.ExitUsage {
			t.Fatalf("exit = %d, want %d", code, core.ExitUsage)
		}
		if !strings.Contains(stderr, "invalid policy") {
			t.Fatalf("unexpected error message: %q", stderr)
		}
	})

	t.Run("workers_out_of_bounds_clamped_with_warning", func(t *testing.T) {
		root := initTestRoot(t)
		stdout, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{
			"--non-interactive",
			"--orchestration", "session",
			"--orchestration-workers", "100",
		}), core.DefaultInitExecutor())
		if code != core.ExitOK {
			t.Fatalf("exit = %d, want %d, stderr=%q", code, core.ExitOK, stderr)
		}
		cfg := core.LoadConfig(root)
		if cfg.Orchestration.MaxWorkers != 64 {
			t.Fatalf("maxWorkers = %d, want 64", cfg.Orchestration.MaxWorkers)
		}
		if !strings.Contains(stdout+stderr, "outside [1,64] — using 64") {
			t.Fatalf("warning missing from output: stdout=%q stderr=%q", stdout, stderr)
		}
	})

	t.Run("dry_run_does_not_write_but_lists_write_config_yml", func(t *testing.T) {
		root := initTestRoot(t)
		stdout, _, code := captureInitOutput(t, cli.ParseArgs([]string{
			"--dry-run",
			"--orchestration", "session",
		}), core.DefaultInitExecutor())
		if code != core.ExitOK {
			t.Fatalf("exit = %d, want %d", code, core.ExitOK)
		}
		if _, err := os.Stat(filepath.Join(root, ".specd", "config.json")); !os.IsNotExist(err) {
			t.Fatalf("config.yml should not exist in dry run")
		}
		if !strings.Contains(stdout, "would write: .specd/config.yml") {
			t.Fatalf("dry-run missing would write config.yml: %s", stdout)
		}
	})

	t.Run("invalid_cost_limit_returns_usage_error", func(t *testing.T) {
		initTestRoot(t)
		_, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{
			"--non-interactive",
			"--orchestration", "session",
			"--orchestration-cost-limit", "abc",
		}), core.DefaultInitExecutor())
		if code != core.ExitUsage {
			t.Fatalf("exit = %d, want %d", code, core.ExitUsage)
		}
		if !strings.Contains(stderr, "invalid cost limit") {
			t.Fatalf("unexpected error message: %q", stderr)
		}
	})

	t.Run("sandbox_selection_bwrap_fails_closed_if_bwrap_not_in_path", func(t *testing.T) {
		originalPath := os.Getenv("PATH")
		os.Setenv("PATH", "")
		defer os.Setenv("PATH", originalPath)

		initTestRoot(t)
		_, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{
			"--non-interactive",
			"--orchestration", "session",
			"--orchestration-sandbox", "bwrap",
		}), core.DefaultInitExecutor())
		if code != core.ExitGate {
			t.Fatalf("exit = %d, want %d", code, core.ExitGate)
		}
		if !strings.Contains(stderr, "bubblewrap not found on PATH") {
			t.Fatalf("unexpected error message: %q", stderr)
		}
	})

	t.Run("updating_orchestration_configuration_on_already_initialized_project", func(t *testing.T) {
		root := initTestRoot(t)
		if _, _, code := captureInitOutput(t, cli.ParseArgs([]string{"--non-interactive", "--agent", "none"}), core.DefaultInitExecutor()); code != core.ExitOK {
			t.Fatalf("initial init exit=%d", code)
		}
		cfgBefore := core.LoadConfig(root)
		if cfgBefore.Orchestration.Enabled {
			t.Fatalf("expected orchestration disabled initially")
		}

		stdout, stderr, code := captureInitOutput(t, cli.ParseArgs([]string{
			"--non-interactive",
			"--orchestration", "session",
			"--orchestration-workers", "12",
			"--agent", "none",
		}), core.DefaultInitExecutor())
		if code != core.ExitOK {
			t.Fatalf("exit = %d, want %d, stderr=%q stdout=%q", code, core.ExitOK, stderr, stdout)
		}

		cfgAfter := core.LoadConfig(root)
		if !cfgAfter.Orchestration.Enabled || cfgAfter.Orchestration.ApprovalPolicy != "session" {
			t.Fatalf("expected orchestration enabled after update")
		}
		if cfgAfter.Orchestration.MaxWorkers != 12 {
			t.Fatalf("maxWorkers = %d, want 12", cfgAfter.Orchestration.MaxWorkers)
		}
	})
}
