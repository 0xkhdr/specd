package cmd_test

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
)

// TestBrainStepDrivesSingleStep covers brainStep via the CLI: one bounded
// decision against a started session.
func TestBrainStepDrivesSingleStep(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "step-spec")
	host := testharness.NewFakeOrchestrationHost(h)
	sessionID := repeat("d")

	host.StartSpec(slug, sessionID)
	res := h.RunExpect(core.ExitOK, "brain", append([]string{"step", slug}, host.PolicyArgs(sessionID)...)...)
	if res.Stdout == "" {
		t.Fatalf("brain step produced no output")
	}
}

// TestBrainDirectiveRecordsEvent covers brainDirective end-to-end.
func TestBrainDirectiveRecordsEvent(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "directive-spec")
	host := testharness.NewFakeOrchestrationHost(h)
	sessionID := repeat("e")
	step := host.StartSpec(slug, sessionID)

	// A directive targets an in-flight worker, so claim the lease first.
	mission := host.Mission(step)
	if _, err := core.ClaimPinkyMission(h.Root, mission, host.Cfg); err != nil {
		t.Fatalf("claim mission: %v", err)
	}

	h.RunExpect(core.ExitOK, "brain", "directive",
		"--session", sessionID, "--worker", mission.WorkerID, "--spec", slug,
		"--task", "T1", "--attempt", "1", "--action", "continue",
		"--reason", "keep going", "--json")

	// Bad attempt is rejected at the usage layer.
	if res := h.Run("brain", "directive", "--session", sessionID, "--worker", "w",
		"--spec", slug, "--task", "T1", "--attempt", "0", "--action", "continue",
		"--reason", "x"); res.Code != core.ExitUsage {
		t.Fatalf("bad --attempt exit = %d, want usage", res.Code)
	}
}

// TestBrainRunProgramDrivesToCompletion covers brainRunProgram and
// brainRunProgramWorker through the injected runner seam.
func TestBrainRunProgramDrivesToCompletion(t *testing.T) {
	h := testharness.New(t)
	h.Init()
	for _, slug := range []string{"prog-a", "prog-b"} {
		h.Spec(slug).
			Req("prog", "As an operator I drive a program.", "THE SYSTEM SHALL drive specs.").
			FullDesign().
			Status(core.StatusExecuting).
			Orchestrated().
			AddTask(testharness.TaskSpec{ID: "T1"}).
			Build()
	}

	rec := newRecordingRunner(h.Root, "")
	defer cmd.SetBrainRunner(rec)()

	sessionID := repeat("f")
	res := h.RunExpect(core.ExitOK, "brain", "run", "--program",
		"--session", sessionID, "--worker-cmd", "true", "--max-workers", "1")
	if res.Stdout == "" && res.Stderr == "" {
		t.Fatalf("brain run --program produced no output")
	}
}

// TestBrainRunResumesActiveSession covers brainRunSession's resume branches: a
// drive that stalls leaves the session running, and a second `brain run` (with
// and without an explicit --session) reattaches to it instead of starting anew.
func TestBrainRunResumesActiveSession(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "resume-active")
	sessionID := repeat("a")

	rec := newRecordingRunner(h.Root, "")
	rec.noComplete = true
	defer cmd.SetBrainRunner(rec)()

	// First drive stalls (no progress) but leaves the session running.
	h.RunExpect(core.ExitOK, "brain", "run", slug,
		"--session", sessionID, "--worker-cmd", "true", "--max-workers", "1", "--max-steps", "2")
	session, err := core.LoadOrchestrationSession(h.Root, sessionID)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if session.Status != core.OrchestrationSessionRunning {
		t.Fatalf("session status = %s, want running", session.Status)
	}

	// Explicit --session re-attaches to the active session.
	if res := h.RunExpect(core.ExitOK, "brain", "run", slug,
		"--session", sessionID, "--worker-cmd", "true", "--max-workers", "1", "--max-steps", "1"); !strings.Contains(res.Out(), "resuming active session") {
		t.Fatalf("explicit resume missing notice: %s", res.Out())
	}

	// No --session: the active session for the spec is discovered and resumed.
	if res := h.RunExpect(core.ExitOK, "brain", "run", slug,
		"--worker-cmd", "true", "--max-workers", "1", "--max-steps", "1"); !strings.Contains(res.Out(), "resuming active session") {
		t.Fatalf("implicit resume missing notice: %s", res.Out())
	}
}

// TestBrainRunBootstrapBlocks covers brainRunBootstrap's blocked branch (a
// non-spec preflight gap) and bootstrapHint's empty-hint path.
func TestBrainRunBootstrapBlocks(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "blocked-spec")

	// Remove steering so preflight reports a non-spec, non-bootstrappable gap.
	if err := os.RemoveAll(h.Path(".specd/steering")); err != nil {
		t.Fatalf("remove steering: %v", err)
	}

	res := h.Run("brain", "run", slug, "--worker-cmd", "true")
	if res.Code != core.ExitGate {
		t.Fatalf("blocked run exit = %d, want %d; out=%s", res.Code, core.ExitGate, res.Out())
	}
}

// TestBrainProgramSessionControl covers brainProgramSessionControl pause/resume.
func TestBrainProgramSessionControl(t *testing.T) {
	h := testharness.New(t)
	h.Init()
	h.Spec("ctl-spec").
		Req("ctl", "As an operator I control a program.", "THE SYSTEM SHALL control specs.").
		FullDesign().
		Status(core.StatusExecuting).
		Orchestrated().
		AddTask(testharness.TaskSpec{ID: "T1"}).
		Build()
	host := testharness.NewFakeOrchestrationHost(h)
	sessionID, err := core.NewACPID()
	if err != nil {
		t.Fatalf("NewACPID: %v", err)
	}
	host.StartProgram(sessionID)

	h.RunExpect(core.ExitOK, "brain", "pause", "--program", "--session", sessionID, "--json")
	h.RunExpect(core.ExitOK, "brain", "resume", "--program", "--session", sessionID, "--json")

	// Missing --session is a usage error.
	if res := h.Run("brain", "pause", "--program"); res.Code != core.ExitUsage {
		t.Fatalf("missing --session exit = %d, want usage; out=%s", res.Code, res.Out())
	}
}
