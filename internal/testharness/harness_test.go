package testharness_test

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// These tests validate the harness itself — its determinism and capture
// guarantees — so failures in the infrastructure surface here rather than as
// confusing failures in the suites that depend on it.

func TestHarnessIsolation(t *testing.T) {
	h := th.New(t)
	h.AssertFileExists(".specd/specs")
	if h.Root == "" {
		t.Fatal("harness root is empty")
	}
}

func TestFakeClockIsDeterministic(t *testing.T) {
	th.New(t) // installs the fake clock over core.Clock
	first := core.NowISO()
	if !strings.HasPrefix(first, "2026-01-02T03:04:05") {
		t.Errorf("NowISO under fake clock = %q, want Epoch-based timestamp", first)
	}
	// Auto-advance yields a distinct, ordered next stamp.
	if second := core.NowISO(); second <= first {
		t.Errorf("clock did not advance: %q then %q", first, second)
	}
}

func TestFakeClockFreeze(t *testing.T) {
	h := th.New(t)
	h.Clock.Freeze()
	if a, b := core.NowISO(), core.NowISO(); a != b {
		t.Errorf("frozen clock changed: %q vs %q", a, b)
	}
}

func TestRunCapturesStreamsAndExit(t *testing.T) {
	h := th.New(t)
	res := h.Run("new") // missing slug → usage error on stderr
	if res.Code != core.ExitUsage {
		t.Errorf("exit = %d, want %d", res.Code, core.ExitUsage)
	}
	if res.Stdout != "" {
		t.Errorf("expected empty stdout, got %q", res.Stdout)
	}
	if !strings.Contains(res.Stderr, "usage") {
		t.Errorf("expected usage text on stderr, got %q", res.Stderr)
	}
}

func TestSpecBuilderProducesCheckCleanSpec(t *testing.T) {
	h := th.New(t)
	h.Spec("demo").
		Req("Core", "As a user, I want the thing", "THE SYSTEM SHALL do the thing.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
		Status(core.StatusTasks).
		Build()

	if res := h.Run("check", "demo"); res.Code != core.ExitOK {
		t.Fatalf("craftsman spec failed check (%d): %s", res.Code, res.Out())
	}
	h.State("demo").Status(core.StatusTasks).TaskStatus("T1", core.TaskPending)
}

func TestSpecBuilderSeedsCompletedTask(t *testing.T) {
	h := th.New(t)
	h.Spec("demo").
		Req("Core", "story", "THE SYSTEM SHALL do the thing.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Verify: "true", Status: core.TaskComplete}).
		Status(core.StatusVerifying).
		Build()

	// A seeded complete task carries evidence + a verified record, so check's
	// evidence gate stays green.
	if res := h.Run("check", "demo"); res.Code != core.ExitOK {
		t.Fatalf("seeded-complete spec failed check (%d): %s", res.Code, res.Out())
	}
	h.State("demo").TaskStatus("T1", core.TaskComplete)
}
