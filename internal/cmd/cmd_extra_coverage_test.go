package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// TestDeployJSONAndRollbackEmpty covers the JSON output path and the
// nothing-to-roll-back branch.
func TestDeployJSONAndRollbackEmpty(t *testing.T) {
	h := th.New(t)
	completeSpec(h, "svc")
	marker := filepath.Join(h.Root, "m")
	writeDeployPlan(t, h, "staging", `{
		"steps": [{"name": "one", "command": "printf x >> `+marker+`", "timeoutSeconds": 10}]
	}`)
	res := h.RunExpect(core.ExitOK, "deploy", "svc", "--env", "staging", "--json")
	if !strings.Contains(res.Out(), `"outcome"`) {
		t.Errorf("expected JSON outcome, got %q", res.Out())
	}
	// Nothing to roll back (steps had no rollbackCommand).
	res = h.RunExpect(core.ExitOK, "deploy", "rollback", "svc", "--env", "staging", "--json")
	if !strings.Contains(res.Out(), `"rolledBack"`) {
		t.Errorf("expected rolledBack JSON, got %q", res.Out())
	}
}

// TestDeployUsageErrors covers missing env and unknown env config.
func TestDeployUsageErrors(t *testing.T) {
	h := th.New(t)
	completeSpec(h, "svc")
	h.RunExpect(core.ExitUsage, "deploy", "svc") // no --env
	h.RunExpect(core.ExitNotFound, "deploy", "svc", "--env", "nope")
	h.RunExpect(core.ExitUsage, "deploy", "rollback", "svc") // rollback no --env
}

// TestObserveJSONAndUsage covers observe JSON output and usage errors.
func TestObserveJSONAndUsage(t *testing.T) {
	h := th.New(t)
	h.Spec("svc").Req("X", "As a user, I want X", "THE SYSTEM SHALL do X.").FullDesign().Status(core.StatusExecuting).Build()
	h.RunExpect(core.ExitUsage, "observe")              // no subcommand
	h.RunExpect(core.ExitUsage, "observe", "correlate") // no file
	h.RunExpect(core.ExitNotFound, "observe", "correlate", "missing.json")

	payload := filepath.Join(h.Root, "p.json")
	_ = os.WriteFile(payload, []byte(`{"severity":"warning","message":"w"}`), 0o644)
	res := h.RunExpect(core.ExitOK, "observe", "correlate", payload, "--spec", "svc", "--json")
	if !strings.Contains(res.Out(), `"confidence"`) {
		t.Errorf("expected JSON confidence, got %q", res.Out())
	}
}

// TestObserveListenConfigGuards covers the listener's pre-Serve refusals (which
// return without blocking): no token, and a non-loopback bind address.
func TestObserveListenConfigGuards(t *testing.T) {
	h := th.New(t)
	// No token configured → refused.
	h.RunExpect(core.ExitGate, "observe", "--listen")

	// Non-loopback addr → refused.
	cfg := filepath.Join(h.Root, ".specd", "config.yml")
	_ = os.WriteFile(cfg, []byte("version: 1\nobserve:\n  token: t\n  addr: 0.0.0.0:9000\n"), 0o644)
	h.RunExpect(core.ExitGate, "observe", "--listen")
}

// TestApproveDeployUsage covers the approve --deploy usage/validation branches.
func TestApproveDeployUsage(t *testing.T) {
	h := th.New(t)
	completeSpec(h, "svc")
	h.RunExpect(core.ExitUsage, "approve", "svc", "--deploy")                  // no env
	h.RunExpect(core.ExitUsage, "approve", "svc", "--deploy", "--env", "../x") // bad env
	h.RunExpect(core.ExitOK, "approve", "svc", "--deploy", "--env", "staging", "--json")
}

// TestIngestGitScoping exercises the git ls-files path (tracked files only) and
// the JSON output.
func TestIngestGitScoping(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	h := th.New(t)
	sub := filepath.Join(h.Root, "app")
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(filepath.Join(sub, "tracked.go"), []byte("package app\n"), 0o644)
	_ = os.WriteFile(filepath.Join(sub, "untracked.go"), []byte("package app\n"), 0o644)

	// Init a git repo and track only one file.
	for _, args := range [][]string{
		{"init"}, {"config", "user.email", "t@t"}, {"config", "user.name", "t"},
		{"add", "app/tracked.go"}, {"commit", "-m", "x"},
	} {
		c := exec.Command("git", args...)
		c.Dir = h.Root
		if out, err := c.CombinedOutput(); err != nil {
			t.Skipf("git setup failed: %v: %s", err, out)
		}
	}

	res := h.RunExpect(core.ExitOK, "ingest", "new", "app-spec", "--path", "app", "--json")
	if !strings.Contains(res.Out(), `"files"`) {
		t.Errorf("expected JSON files, got %q", res.Out())
	}
	inv, _ := core.LoadInventory(h.Root, "app-spec")
	if inv == nil {
		t.Fatal("no inventory")
	}
	for _, f := range inv.Files {
		if strings.Contains(f.Path, "untracked") {
			t.Errorf("untracked file leaked into git-scoped inventory: %s", f.Path)
		}
	}
}

// TestDeployJSONVariants covers the JSON render branches for blocked, dry-run,
// and step-failure outcomes.
func TestDeployJSONVariants(t *testing.T) {
	h := th.New(t)
	completeSpec(h, "svc")

	// Blocked (gate refusal) in JSON.
	writeDeployPlan(t, h, "staging", `{
		"requiresGates": ["review"],
		"steps": [{"name": "one", "command": "true", "timeoutSeconds": 10}]
	}`)
	res := h.RunExpect(core.ExitGate, "deploy", "svc", "--env", "staging", "--json")
	if !strings.Contains(res.Out(), `"blocked"`) {
		t.Errorf("expected blocked JSON, got %q", res.Out())
	}

	// Dry-run JSON.
	writeDeployPlan(t, h, "dev", `{
		"steps": [{"name": "one", "command": "true", "timeoutSeconds": 10}]
	}`)
	res = h.RunExpect(core.ExitOK, "deploy", "svc", "--env", "dev", "--dry-run", "--json")
	if !strings.Contains(res.Out(), `"dryRun"`) {
		t.Errorf("expected dryRun JSON, got %q", res.Out())
	}

	// Step-failure JSON.
	writeDeployPlan(t, h, "qa", `{
		"steps": [{"name": "boom", "command": "exit 1", "timeoutSeconds": 10}]
	}`)
	res = h.RunExpect(core.ExitGate, "deploy", "svc", "--env", "qa", "--json")
	if !strings.Contains(res.Out(), `"step-failed"`) {
		t.Errorf("expected step-failed JSON, got %q", res.Out())
	}
}

// TestDeployRollbackStepFailure: a failing rollback step halts and exits 3.
func TestDeployRollbackStepFailure(t *testing.T) {
	h := th.New(t)
	completeSpec(h, "svc")
	writeDeployPlan(t, h, "staging", `{
		"steps": [
			{"name": "one", "command": "true", "rollbackCommand": "exit 1", "timeoutSeconds": 10},
			{"name": "two", "command": "exit 1", "rollbackCommand": "true", "timeoutSeconds": 10}
		]
	}`)
	h.RunExpect(core.ExitGate, "deploy", "svc", "--env", "staging")
	// Rollback of step one fails → exit 3 (blocked).
	h.RunExpect(core.ExitNotFound, "deploy", "rollback", "svc", "--env", "staging")
}

// TestIngestTitleAndIncludeIgnored covers the --title and --include-ignored flags.
func TestIngestTitleAndIncludeIgnored(t *testing.T) {
	h := th.New(t)
	sub := filepath.Join(h.Root, "src")
	_ = os.MkdirAll(sub, 0o755)
	_ = os.WriteFile(filepath.Join(sub, "a.go"), []byte("package a\n"), 0o644)
	h.RunExpect(core.ExitOK, "ingest", "new", "s", "--path", "src", "--title", "My Legacy", "--include-ignored")
	if st := h.State("s").Raw(); st.Title != "My Legacy" {
		t.Errorf("title = %q, want My Legacy", st.Title)
	}
}

// TestIngestUsageErrors covers the usage/validation refusals.
func TestIngestUsageErrors(t *testing.T) {
	h := th.New(t)
	h.RunExpect(core.ExitUsage, "ingest")             // no subcommand
	h.RunExpect(core.ExitUsage, "ingest", "new")      // no slug
	h.RunExpect(core.ExitUsage, "ingest", "new", "s") // no --path
	h.RunExpect(core.ExitNotFound, "ingest", "new", "s", "--path", "nope")
}
