package cmd

import (
	"errors"
	"os/exec"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func TestWorkerExitCodeMapping(t *testing.T) {
	if got := workerExitCode(nil); got != 0 {
		t.Fatalf("nil err = %d, want 0", got)
	}
	if got := workerExitCode(errors.New("boom")); got != 1 {
		t.Fatalf("generic err = %d, want 1", got)
	}
	// A real subprocess exit carries a syscall.WaitStatus the mapper must surface.
	err := exec.Command("sh", "-c", "exit 7").Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got %T", err)
	}
	if got := workerExitCode(err); got != 7 {
		t.Fatalf("exit-error code = %d, want 7", got)
	}
}

func TestBootstrapHint(t *testing.T) {
	if got := bootstrapHint(core.PreflightItem{Kind: "spec"}); got != " (or pass --bootstrap)" {
		t.Fatalf("spec hint = %q", got)
	}
	if got := bootstrapHint(core.PreflightItem{Kind: "steering"}); got != "" {
		t.Fatalf("non-spec hint = %q, want empty", got)
	}
}

func TestBrainRunPolicyDefaultsAndOverrides(t *testing.T) {
	root := t.TempDir()

	policy, _, ok := brainRunPolicy(root, cli.ParseArgs(nil))
	if !ok {
		t.Fatal("default policy rejected")
	}
	if policy.ApprovalPolicy != "planning" {
		t.Fatalf("default approval = %q, want planning", policy.ApprovalPolicy)
	}

	policy, _, ok = brainRunPolicy(root, cli.ParseArgs([]string{
		"--approval-policy", "manual", "--max-workers", "3", "--max-retries", "5",
	}))
	if !ok {
		t.Fatal("override policy rejected")
	}
	if policy.ApprovalPolicy != "manual" || policy.MaxWorkers != 3 || policy.MaxRetries != 5 {
		t.Fatalf("overrides not applied: %+v", policy)
	}

	if _, _, ok := brainRunPolicy(root, cli.ParseArgs([]string{"--max-workers", "0"})); ok {
		t.Fatal("accepted non-positive --max-workers")
	}
	if _, _, ok := brainRunPolicy(root, cli.ParseArgs([]string{"--max-retries", "-1"})); ok {
		t.Fatal("accepted negative --max-retries")
	}
}

func TestParseIntFlags(t *testing.T) {
	if _, ok := parsePositiveIntFlag(cli.ParseArgs([]string{"--n", "0"}), "n"); ok {
		t.Fatal("parsePositiveIntFlag accepted 0")
	}
	if n, ok := parsePositiveIntFlag(cli.ParseArgs([]string{"--n", "4"}), "n"); !ok || n != 4 {
		t.Fatalf("parsePositiveIntFlag(4) = %d,%v", n, ok)
	}
	if _, ok := parseNonNegativeIntFlag(cli.ParseArgs([]string{"--n", "-2"}), "n"); ok {
		t.Fatal("parseNonNegativeIntFlag accepted -2")
	}
	if n, ok := parseNonNegativeIntFlag(cli.ParseArgs([]string{"--n", "0"}), "n"); !ok || n != 0 {
		t.Fatalf("parseNonNegativeIntFlag(0) = %d,%v", n, ok)
	}
}

func TestBrainStartSessionID(t *testing.T) {
	id, err := brainStartSessionID(cli.ParseArgs([]string{"--session", "fixed"}))
	if err != nil || id != "fixed" {
		t.Fatalf("explicit session = %q,%v", id, err)
	}
	gen, err := brainStartSessionID(cli.ParseArgs(nil))
	if err != nil || gen == "" {
		t.Fatalf("generated session = %q,%v", gen, err)
	}
}
