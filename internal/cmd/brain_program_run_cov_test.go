package cmd_test

import (
	"context"
	"strings"
	"sync"
	"testing"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
	"github.com/0xkhdr/specd/internal/worker"
)

// selectiveRunner completes only the child specs whose slug is in complete, and
// reports bare success (no completion) for the rest. It records which specs it
// dispatched so a test can assert the resume driver does not re-dispatch a child
// that was already complete (cross-spec recovery).
type selectiveRunner struct {
	root string

	mu         sync.Mutex
	complete   map[string]bool
	dispatched []string
}

func (r *selectiveRunner) Run(_ context.Context, m worker.Mission) (worker.Result, error) {
	pm, ok := m.Payload.(core.PinkyMission)
	if !ok {
		return worker.Result{}, nil
	}
	r.mu.Lock()
	r.dispatched = append(r.dispatched, pm.Spec)
	finish := r.complete[pm.Spec]
	r.mu.Unlock()
	if !finish {
		return worker.Result{}, nil
	}
	if err := completeTask(r.root, pm); err != nil {
		return worker.Result{}, err
	}
	st, err := core.LoadState(r.root, pm.Spec)
	if err != nil {
		return worker.Result{}, err
	}
	st.Status = core.StatusComplete
	st.Phase = core.PhaseForStatus(core.StatusComplete)
	return worker.Result{}, core.SaveState(r.root, pm.Spec, st)
}

func (r *selectiveRunner) dispatchedSpecs() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.dispatched...)
}

// TestBrainRunProgramJSONAndMaxSteps covers brainRunProgram's --json output and
// the bounded --max-steps path (the existing program test exercises only the
// default text-output completion).
func TestBrainRunProgramJSONAndMaxSteps(t *testing.T) {
	h := testharness.New(t)
	h.Init()
	for _, slug := range []string{"pj-a", "pj-b"} {
		h.Spec(slug).
			Req("pj", "As an operator I drive a program.", "THE SYSTEM SHALL drive specs.").
			FullDesign().
			Status(core.StatusExecuting).
			Orchestrated().
			AddTask(testharness.TaskSpec{ID: "T1"}).
			Build()
	}

	rec := newRecordingRunner(h.Root, "")
	rec.noComplete = true // bounded run: stop at max-steps without completing
	defer cmd.SetBrainRunner(rec)()

	sessionID := repeat("c")
	res := h.RunExpect(core.ExitOK, "brain", "run", "--program",
		"--session", sessionID, "--worker-cmd", "true",
		"--max-workers", "1", "--max-steps", "2", "--json")
	if !strings.Contains(res.Out(), sessionID) {
		t.Fatalf("program --json output missing session id: %s", res.Out())
	}
}

// TestBrainResumeProgramReconstructsFrontier proves cross-spec recovery: a
// 3-child program is interrupted mid-frontier (one child complete, the others
// not), then `brain resume --program` reads the persisted program-state, rebuilds
// the frontier, and drives the remaining children to completion without
// re-dispatching the already-complete child. A second resume is idempotent.
func TestBrainResumeProgramReconstructsFrontier(t *testing.T) {
	h := testharness.New(t)
	h.Init()
	for _, slug := range []string{"tcr-a", "tcr-b", "tcr-c"} {
		h.Spec(slug).
			Req("tcr", "As an operator I recover a program.", "THE SYSTEM SHALL recover programs.").
			FullDesign().
			Status(core.StatusExecuting).
			Orchestrated().
			AddTask(testharness.TaskSpec{ID: "T1"}).
			Build()
	}
	sessionID := repeat("9")

	// Phase 1: only tcr-a is allowed to complete; the program is bounded so it
	// stops with tcr-a done and the rest still in flight.
	partial := &selectiveRunner{root: h.Root, complete: map[string]bool{"tcr-a": true}}
	restore := cmd.SetBrainRunner(partial)
	h.RunExpect(core.ExitOK, "brain", "run", "--program",
		"--session", sessionID, "--worker-cmd", "true",
		"--max-workers", "1", "--max-steps", "12")
	restore()

	state, err := core.LoadProgramState(h.Root, sessionID)
	if err != nil {
		t.Fatalf("load program-state after interrupt: %v", err)
	}
	if state.ChildStatus["tcr-a"] != core.StatusComplete {
		t.Fatalf("tcr-a status = %q, want complete; frontier=%+v", state.ChildStatus["tcr-a"], state.ChildStatus)
	}
	if got := state.CompleteChildCount(); got != 1 {
		t.Fatalf("complete child count = %d, want 1; frontier=%+v", got, state.ChildStatus)
	}

	// Phase 2: resume with a runner that completes everything. The frontier
	// reconstructs and only the unfinished children advance.
	full := &selectiveRunner{root: h.Root, complete: map[string]bool{"tcr-a": true, "tcr-b": true, "tcr-c": true}}
	restore = cmd.SetBrainRunner(full)
	res := h.RunExpect(core.ExitOK, "brain", "resume", "--program",
		"--session", sessionID, "--worker-cmd", "true", "--max-workers", "1")
	restore()
	if !strings.Contains(res.Out(), "reconstructed frontier") {
		t.Fatalf("resume missing frontier reconstruction notice: %s", res.Out())
	}

	for _, slug := range full.dispatchedSpecs() {
		if slug == "tcr-a" {
			t.Fatalf("resume re-dispatched already-complete child tcr-a: %v", full.dispatchedSpecs())
		}
	}

	session, err := core.LoadProgramSession(h.Root, sessionID)
	if err != nil {
		t.Fatalf("load program session: %v", err)
	}
	if session.Status != core.OrchestrationSessionComplete {
		t.Fatalf("program status = %q, want complete", session.Status)
	}

	// Idempotency: a second resume does not re-drive a completed program.
	restore = cmd.SetBrainRunner(&selectiveRunner{root: h.Root, complete: map[string]bool{}})
	again := h.RunExpect(core.ExitOK, "brain", "resume", "--program",
		"--session", sessionID, "--worker-cmd", "true", "--max-workers", "1")
	restore()
	if !strings.Contains(again.Out(), "nothing to resume") {
		t.Fatalf("second resume should no-op on a complete program: %s", again.Out())
	}
}
