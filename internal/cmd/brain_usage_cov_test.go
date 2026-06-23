package cmd_test

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
)

// TestBrainRoutingUsage exercises RunBrain's command routing and arg-count
// guards: every malformed invocation must fail with a usage exit.
func TestBrainRoutingUsage(t *testing.T) {
	h := testharness.New(t)
	h.Init()

	cases := [][]string{
		{"brain"},
		{"brain", "bogus"},
		{"brain", "start"},
		{"brain", "start", "--program", "extra"},
		{"brain", "run"},
		{"brain", "run", "--program", "extra"},
		{"brain", "step", "slug"},
		{"brain", "step", "--program"},
		{"brain", "why", "a", "b"},
		{"brain", "status"},
		{"brain", "status", "--program"},
	}
	for _, c := range cases {
		if res := h.Run(c[0], c[1:]...); res.Code != core.ExitUsage {
			t.Fatalf("%v exit = %d, want usage; out=%s", c, res.Code, res.Out())
		}
	}
}

// TestBrainRefusesBaseSpecGate covers requireOrchestratedSpec's gate branch: Brain
// must refuse a base-mode spec for both start and run.
func TestBrainRefusesBaseSpecGate(t *testing.T) {
	h := testharness.New(t)
	h.Init()
	slug := h.Spec("base-spec").
		Req("base", "As an operator I keep a base spec.", "THE SYSTEM SHALL stay base.").
		FullDesign().
		Status(core.StatusExecuting).
		AddTask(testharness.TaskSpec{ID: "T1"}).
		Build()

	host := testharness.NewFakeOrchestrationHost(h)
	if res := h.Run("brain", append([]string{"start", slug}, host.PolicyArgs(repeat("a"))...)...); res.Code != core.ExitGate {
		t.Fatalf("base start exit = %d, want gate; out=%s", res.Code, res.Out())
	}
	if res := h.Run("brain", "run", slug, "--worker-cmd", "true"); res.Code != core.ExitGate {
		t.Fatalf("base run exit = %d, want gate; out=%s", res.Code, res.Out())
	}
	if res := h.Run("brain", append([]string{"step", slug}, host.PolicyArgs(repeat("b"))...)...); res.Code != core.ExitGate {
		t.Fatalf("base step exit = %d, want gate; out=%s", res.Code, res.Out())
	}
}

// TestBrainSessionControlEdges covers brainSessionControl's usage and not-found
// branches and brainPolicy's cost-limit handling.
func TestBrainSessionControlEdges(t *testing.T) {
	h := testharness.New(t)
	h.Init()

	// Missing --session is a usage error.
	if res := h.Run("brain", "cancel"); res.Code != core.ExitUsage {
		t.Fatalf("cancel without session exit = %d, want usage", res.Code)
	}
	// Unknown session is a clean not-found.
	if res := h.Run("brain", "cancel", "--session", repeat("c")); res.Code != core.ExitNotFound {
		t.Fatalf("cancel unknown session exit = %d, want not-found; out=%s", res.Code, res.Out())
	}

	// brainPolicy rejects a malformed --cost-limit.
	slug := h.Spec("cost-spec").
		Req("cost", "As an operator I bound cost.", "THE SYSTEM SHALL bound cost.").
		FullDesign().Status(core.StatusExecuting).Orchestrated().
		AddTask(testharness.TaskSpec{ID: "T1"}).Build()
	if res := h.Run("brain", "start", slug,
		"--approval-policy", "manual", "--max-workers", "1", "--max-retries", "0",
		"--timeout-seconds", "60", "--cost-limit", "nope"); res.Code == core.ExitOK {
		t.Fatalf("bad --cost-limit accepted: %s", res.Out())
	}
}
