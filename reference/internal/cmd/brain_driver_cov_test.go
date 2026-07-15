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

// TestBrainDriverProgressWeightedWaits proves R6: a driver wait does not count
// toward the stall limit while an in-flight worker has reported progress within
// resilience.progressTimeoutSeconds, but the same state stalls once weighting is
// disabled. T1 is leased but no driver goroutine runs it (the slow- or
// rate-limit-suspended-worker case), so the engine returns wait every step.
func TestBrainDriverProgressWeightedWaits(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "progress-weighted")
	sessionID := repeat("b")

	cfg := core.LoadConfig(h.Root).Orchestration
	cfg.Enabled = true
	cfg.Resilience = &core.ResilienceCfg{ProgressTimeoutSeconds: 300}
	policy, err := core.NewOrchestrationPolicy(cfg)
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	if _, err := core.StartOrchestrationSession(h.Root, slug, sessionID, "test", policy); err != nil {
		t.Fatalf("start session: %v", err)
	}

	// A worker claims T1 and reports progress; no driver goroutine runs it, so the
	// task stays leased and every step the engine returns wait.
	mission, err := core.BuildPinkyMission(h.Root, slug, sessionID, "pinky-a", "T1", 1, cfg)
	if err != nil {
		t.Fatalf("build mission: %v", err)
	}
	if _, err := core.ClaimPinkyMission(h.Root, mission, cfg); err != nil {
		t.Fatalf("claim mission: %v", err)
	}
	if _, err := core.RecordPinkyProgress(h.Root, core.PinkyProgressReport{
		SessionID: sessionID, WorkerID: "pinky-a", Spec: slug, TaskID: "T1",
		Attempt: 1, Percent: 40, Message: "long compile in progress",
	}, cfg); err != nil {
		t.Fatalf("record progress: %v", err)
	}

	// MaxWaits below MaxSteps: an unweighted drive stalls before the ceiling. The
	// worker callback never fires because T1 is already leased.
	opts := core.DriverOptions{
		MaxSteps: 6,
		MaxWaits: 2,
		Worker:   func(core.DriverDispatch) error { return nil },
	}

	// Fresh progress within the window: waits are not counted, so the drive runs
	// to the MaxSteps ceiling instead of falsely stalling.
	res, err := core.DriveOrchestration(h.Root, slug, sessionID, policy, cfg, opts)
	if err != nil {
		t.Fatalf("weighted drive: %v", err)
	}
	if res.Outcome != core.DriverMaxSteps {
		t.Fatalf("weighted drive outcome = %s, want max-steps (no false stall)", res.Outcome)
	}

	// Same state, weighting disabled (resilience block absent): the wait counter
	// advances and the drive stalls at MaxWaits.
	cfgOff := cfg
	cfgOff.Resilience = nil
	res2, err := core.DriveOrchestration(h.Root, slug, sessionID, policy, cfgOff, opts)
	if err != nil {
		t.Fatalf("unweighted drive: %v", err)
	}
	if res2.Outcome != core.DriverStalled {
		t.Fatalf("unweighted drive outcome = %s, want stalled", res2.Outcome)
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
