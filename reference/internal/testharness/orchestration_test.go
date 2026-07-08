package testharness_test

import (
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// dispatchOneTask authors a single-task spec, starts + approves it, and returns
// the harness, host, and the dispatch step ready to be claimed.
func dispatchOneTask(t *testing.T, sessionID string) (*th.Harness, *th.FakeOrchestrationHost, core.OrchestrationStepResult) {
	t.Helper()
	h := th.New(t)
	h.Spec("demo").
		Req("demo", "As a user, I want demo.", "THE SYSTEM SHALL satisfy demo.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "do demo", Files: "pass.flag", Verify: "test -f pass.flag", Requirements: []int{1}}).
		Status(core.StatusExecuting).
		Gate(core.GateAwaitingApproval).
		Build()

	host := th.NewFakeOrchestrationHost(h)
	if approval := host.StartSpec("demo", sessionID); approval.Decision.Action != core.OrchestrationRequestApproval {
		t.Fatalf("start decision = %s, want approval", approval.Decision.Action)
	}
	h.RunExpect(core.ExitOK, "approve", "demo")

	dispatch := host.StepSpec("demo", sessionID)
	if dispatch.Decision.Action != core.OrchestrationDispatch || dispatch.Decision.TaskID != "T1" {
		t.Fatalf("dispatch = %#v, want T1 dispatch", dispatch.Decision)
	}
	return h, host, dispatch
}

// TestFakeOrchestrationHostHappyPath drives the harness's FakeOrchestrationHost
// through a spec to completion: approval, dispatch, claim+verify+report
// (via Complete), and the wind-down to complete-session.
func TestFakeOrchestrationHostHappyPath(t *testing.T) {
	sessionID := strings.Repeat("a", 32)
	h, host, dispatch := dispatchOneTask(t, sessionID)

	// PolicyArgs exposes the session's policy flags.
	if args := host.PolicyArgs(sessionID); len(args) == 0 {
		t.Error("PolicyArgs returned no args")
	}

	// Make verify pass, then Complete claims, verifies, and reports in one shot.
	if err := os.WriteFile(h.Path("pass.flag"), []byte("ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	accepted := host.Complete(dispatch, "done")
	if accepted.Completion.Status != core.TaskComplete {
		t.Fatalf("completion = %#v, want complete", accepted.Completion)
	}

	if done := host.StepSpec("demo", sessionID); done.Decision.Action != core.OrchestrationCompleteSession {
		t.Fatalf("final decision = %s, want complete-session", done.Decision.Action)
	}
}

// TestFakePinkyWorkerLeaseOps exercises the host-facing worker lease verbs that
// the Complete fast-path does not: claim, heartbeat, progress, then release.
func TestFakePinkyWorkerLeaseOps(t *testing.T) {
	sessionID := strings.Repeat("b", 32)
	h, host, dispatch := dispatchOneTask(t, sessionID)

	mission := host.Mission(dispatch)
	worker := th.NewFakePinkyWorker(h, mission.WorkerID)
	worker.Claim(mission)

	if res := worker.Heartbeat(mission); res.Code != core.ExitOK {
		t.Errorf("heartbeat code = %d: %s", res.Code, res.Out())
	}
	if res := worker.Progress(mission, 50, "halfway"); res.Code != core.ExitOK {
		t.Errorf("progress code = %d: %s", res.Code, res.Out())
	}
	if res := worker.Release(mission); res.Code != core.ExitOK {
		t.Errorf("release code = %d: %s", res.Code, res.Out())
	}

	// Reclaiming after release is a no-op but must not error.
	host.ExpireLeasesAndReclaim(sessionID)
}

// TestFakePinkyWorkerBlock covers the block verb on a freshly claimed mission.
func TestFakePinkyWorkerBlock(t *testing.T) {
	sessionID := strings.Repeat("c", 32)
	h, host, dispatch := dispatchOneTask(t, sessionID)

	mission := host.Mission(dispatch)
	worker := th.NewFakePinkyWorker(h, mission.WorkerID)
	worker.Claim(mission)
	if res := worker.Block(mission, "needs input"); res.Code != core.ExitOK {
		t.Errorf("block code = %d: %s", res.Code, res.Out())
	}
}
