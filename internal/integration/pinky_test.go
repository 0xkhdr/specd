package integration_test

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
)

func TestFakePinkyWorkerSuccessAndDuplicateReport(t *testing.T) {
	h := newPinkyHarness(t, "true")
	cfg := core.LoadConfig(h.Root).Orchestration
	mission := buildMission(t, h, strings.Repeat("1", 32), "t1-a1", 1, cfg)
	worker := testharness.NewFakePinkyWorker(h, mission.WorkerID)

	worker.Claim(mission)
	if res := worker.Progress(mission, 50, "halfway"); res.Code != core.ExitOK {
		t.Fatalf("progress exit=%d out=%s", res.Code, res.Out())
	}
	verify, rec := worker.RunVerify(mission)
	if verify.Code != core.ExitOK || rec == nil || !rec.Verified {
		t.Fatalf("verify exit=%d rec=%#v out=%s", verify.Code, rec, verify.Out())
	}
	accepted, report, err := worker.ReportVerified(mission, rec, "done")
	if report.Code != core.ExitOK || err != nil {
		t.Fatalf("report exit=%d err=%v out=%s", report.Code, err, report.Out())
	}
	if accepted.Completion.Status != core.TaskComplete || accepted.Completion.AlreadyComplete {
		t.Fatalf("completion=%#v, want fresh complete", accepted.Completion)
	}

	duplicate, dupReport, err := worker.ReportVerified(mission, rec, "done again")
	if dupReport.Code != core.ExitOK || err != nil {
		t.Fatalf("duplicate report exit=%d err=%v out=%s", dupReport.Code, err, dupReport.Out())
	}
	if !duplicate.Completion.AlreadyComplete || duplicate.Event.MessageID != accepted.Event.MessageID {
		t.Fatalf("duplicate=%#v, want idempotent same event", duplicate)
	}
}

func TestFakePinkyWorkerBlockCancelAndLeaseExpiry(t *testing.T) {
	h := newPinkyHarness(t, "true")
	cfg := core.LoadConfig(h.Root).Orchestration
	mission := buildMission(t, h, strings.Repeat("2", 32), "t1-a1", 1, cfg)
	worker := testharness.NewFakePinkyWorker(h, mission.WorkerID)

	worker.Claim(mission)
	if res := worker.Block(mission, "needs human input"); res.Code != core.ExitOK {
		t.Fatalf("block exit=%d out=%s", res.Code, res.Out())
	}
	cancelled, err := worker.AcknowledgeCancel(mission, "stopped")
	if err != nil || cancelled.Type != core.ACPMessageCancelled {
		t.Fatalf("cancel ack=%#v err=%v", cancelled, err)
	}

	h.Clock.Advance(time.Duration(cfg.Transport.LeaseSeconds+1) * time.Second)
	late := worker.Progress(mission, 80, "late")
	if late.Code != core.ExitGate || !strings.Contains(late.Out(), "expired") {
		t.Fatalf("late progress exit=%d out=%s, want expired lease gate", late.Code, late.Out())
	}
}

func TestFakePinkyWorkerRetryAfterExpiredFailedAttempt(t *testing.T) {
	h := newPinkyHarness(t, "false")
	cfg := core.LoadConfig(h.Root).Orchestration
	sessionID := strings.Repeat("3", 32)
	policyArgs := []string{"--session", sessionID, "--approval-policy", "manual", "--max-workers", "1", "--max-retries", "2", "--timeout-seconds", "7200", "--json"}

	start := h.RunExpect(core.ExitOK, "brain", append([]string{"start", "demo"}, policyArgs...)...)
	var first core.OrchestrationStepResult
	if err := json.Unmarshal([]byte(start.Stdout), &first); err != nil {
		t.Fatal(err)
	}
	if first.Decision.Action != core.OrchestrationDispatch || first.Decision.Attempt != 1 {
		t.Fatalf("first decision=%#v, want attempt 1 dispatch", first.Decision)
	}

	mission := buildMission(t, h, sessionID, "t1-a1", 1, cfg)
	worker := testharness.NewFakePinkyWorker(h, mission.WorkerID)
	worker.Claim(mission)
	if res, rec := worker.RunVerify(mission); res.Code != core.ExitGate || rec == nil || rec.Verified {
		t.Fatalf("failed verify exit=%d rec=%#v out=%s", res.Code, rec, res.Out())
	}
	h.Clock.Advance(time.Duration(cfg.Transport.LeaseSeconds+1) * time.Second)
	if reclaimed, err := core.ReclaimExpiredLeases(h.Root, sessionID); err != nil || reclaimed != 1 {
		t.Fatalf("reclaim=%d err=%v, want 1", reclaimed, err)
	}

	step := h.RunExpect(core.ExitOK, "brain", append([]string{"step", "demo"}, policyArgs...)...)
	var second core.OrchestrationStepResult
	if err := json.Unmarshal([]byte(step.Stdout), &second); err != nil {
		t.Fatal(err)
	}
	if second.Decision.Action != core.OrchestrationDispatch || second.Decision.Attempt != 2 {
		t.Fatalf("second decision=%#v, want retry dispatch attempt 2", second.Decision)
	}
}

func TestFakePinkyWorkerScopeViolationFailsClosed(t *testing.T) {
	h := newPinkyHarness(t, "true")
	if err := core.AtomicWrite(core.ConfigPath(h.Root), `{"gates":{"scope":"error"}}`+"\n"); err != nil {
		t.Fatal(err)
	}
	cfg := core.LoadConfig(h.Root).Orchestration
	mission := buildMission(t, h, strings.Repeat("4", 32), "t1-a1", 1, cfg)
	worker := testharness.NewFakePinkyWorker(h, mission.WorkerID)
	worker.Claim(mission)

	head := "deadbeefcafefeed"
	rec := &core.VerificationRecord{Command: "true", ExitCode: 0, Verified: true, RanAt: core.NowISO(), GitHead: &head, ChangedFiles: []string{"evil/outside.go"}}
	worker.SeedVerification("demo", "T1", rec)
	_, report, err := worker.ReportVerified(mission, rec, "out of scope")
	if report.Code != core.ExitOK || err == nil || !strings.Contains(err.Error(), "outside its declared files contract") {
		t.Fatalf("report exit=%d err=%v out=%s, want scope rejection after immutable report", report.Code, err, report.Out())
	}
	loaded, err := core.LoadSpec(h.Root, "demo")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.State.Tasks["T1"].Status == core.TaskComplete {
		t.Fatal("scope violation completed task")
	}
}

func newPinkyHarness(t *testing.T, verify string) *testharness.Harness {
	t.Helper()
	h := testharness.New(t)
	h.Spec("demo").
		Req("demo", "As a user, I want demo.", "THE SYSTEM SHALL satisfy demo.").
		FullDesign().
		AddTask(testharness.TaskSpec{ID: "T1", Title: "do demo", Files: "internal/core/demo.go", Verify: verify}).
		Status(core.StatusExecuting).
		Build()
	return h
}

func buildMission(t *testing.T, h *testharness.Harness, sessionID, workerID string, attempt int, cfg core.OrchestrationCfg) core.PinkyMission {
	t.Helper()
	mission, err := core.BuildPinkyMission(h.Root, "demo", sessionID, workerID, "T1", attempt, cfg)
	if err != nil {
		t.Fatal(err)
	}
	return mission
}
