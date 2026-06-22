package testharness

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// FakeOrchestrationHost is a deterministic Brain/Pinky host loop for
// integration tests. It drives only public CLI/core contracts, never provider
// SDKs or network calls.
type FakeOrchestrationHost struct {
	H      *Harness
	Policy core.OrchestrationPolicy
	Cfg    core.OrchestrationCfg
}

func NewFakeOrchestrationHost(h *Harness) *FakeOrchestrationHost {
	h.T.Helper()
	cfg := core.LoadConfig(h.Root).Orchestration
	cfg.Enabled = true
	policy, err := core.NewOrchestrationPolicy(cfg)
	if err != nil {
		h.T.Fatalf("NewFakeOrchestrationHost: policy: %v", err)
	}
	return &FakeOrchestrationHost{H: h, Policy: policy, Cfg: cfg}
}

func (host *FakeOrchestrationHost) PolicyArgs(sessionID string) []string {
	host.H.T.Helper()
	return []string{
		"--session", sessionID,
		"--approval-policy", host.Policy.ApprovalPolicy,
		"--max-workers", strconv.Itoa(host.Policy.MaxWorkers),
		"--max-retries", strconv.Itoa(host.Policy.MaxRetries),
		"--timeout-seconds", strconv.Itoa(host.Policy.SessionTimeoutSeconds),
		"--json",
	}
}

func (host *FakeOrchestrationHost) StartSpec(slug, sessionID string) core.OrchestrationStepResult {
	host.H.T.Helper()
	host.EnsureOrchestrated(slug)
	res := host.H.RunExpect(core.ExitOK, "brain", append([]string{"start", slug}, host.PolicyArgs(sessionID)...)...)
	return decodeStep(host.H, res.Stdout)
}

// EnsureOrchestrated records executionMode=orchestrated on the spec so the Brain
// CLI gate (which refuses Base specs) lets the orchestration host drive it. The
// host models an already-capable, opted-in project, so it writes the recorded
// fact directly rather than going through the capability-gated `specd mode --set`.
func (host *FakeOrchestrationHost) EnsureOrchestrated(slug string) {
	host.H.T.Helper()
	if _, err := core.WithSpecLock[int](host.H.Root, slug, func() (int, error) {
		state, err := core.LoadState(host.H.Root, slug)
		if err != nil || state == nil {
			return 0, err
		}
		if state.EffectiveMode() == core.ModeOrchestrated {
			return 0, nil
		}
		state.ExecutionMode = core.ModeOrchestrated
		state.ModeOrigin = core.OriginUser
		return 0, core.SaveState(host.H.Root, slug, state)
	}); err != nil {
		host.H.T.Fatalf("EnsureOrchestrated(%s): %v", slug, err)
	}
}

func (host *FakeOrchestrationHost) StepSpec(slug, sessionID string) core.OrchestrationStepResult {
	host.H.T.Helper()
	res := host.H.RunExpect(core.ExitOK, "brain", append([]string{"step", slug}, host.PolicyArgs(sessionID)...)...)
	return decodeStep(host.H, res.Stdout)
}

func (host *FakeOrchestrationHost) StartProgram(sessionID string) core.ProgramStepResult {
	host.H.T.Helper()
	res := host.H.RunExpect(core.ExitOK, "brain", append([]string{"start", "--program"}, host.PolicyArgs(sessionID)...)...)
	return decodeProgramStep(host.H, res.Stdout)
}

func (host *FakeOrchestrationHost) StepProgram(sessionID string) core.ProgramStepResult {
	host.H.T.Helper()
	res := host.H.RunExpect(core.ExitOK, "brain", append([]string{"step", "--program"}, host.PolicyArgs(sessionID)...)...)
	return decodeProgramStep(host.H, res.Stdout)
}

func (host *FakeOrchestrationHost) ClaimAndVerify(step core.OrchestrationStepResult) (core.PinkyMission, Result, *core.VerificationRecord) {
	host.H.T.Helper()
	mission := host.Mission(step)
	worker := NewFakePinkyWorker(host.H, mission.WorkerID)
	worker.Claim(mission)
	verify, rec := worker.RunVerify(mission)
	return mission, verify, rec
}

func (host *FakeOrchestrationHost) Complete(step core.OrchestrationStepResult, summary string) core.PinkyEvidenceResult {
	host.H.T.Helper()
	mission, verify, rec := host.ClaimAndVerify(step)
	if verify.Code != core.ExitOK || rec == nil || !rec.Verified {
		host.H.T.Fatalf("FakeOrchestrationHost.Complete: verify exit=%d rec=%#v out=%s", verify.Code, rec, verify.Out())
	}
	worker := NewFakePinkyWorker(host.H, mission.WorkerID)
	accepted, report, err := worker.ReportVerified(mission, rec, summary)
	if report.Code != core.ExitOK || err != nil {
		host.H.T.Fatalf("FakeOrchestrationHost.Complete: report exit=%d err=%v out=%s", report.Code, err, report.Out())
	}
	return accepted
}

func (host *FakeOrchestrationHost) Mission(step core.OrchestrationStepResult) core.PinkyMission {
	host.H.T.Helper()
	if step.Decision.Action != core.OrchestrationDispatch && step.Decision.Action != core.OrchestrationRetry {
		host.H.T.Fatalf("FakeOrchestrationHost.Mission: decision = %s, want dispatch/retry", step.Decision.Action)
	}
	mission, err := core.BuildPinkyMission(host.H.Root, step.Decision.Spec, step.Snapshot.SessionID, workerID(step.Decision), step.Decision.TaskID, step.Decision.Attempt, host.Cfg)
	if err != nil {
		host.H.T.Fatalf("FakeOrchestrationHost.Mission: %v", err)
	}
	return mission
}

func (host *FakeOrchestrationHost) ExpireLeasesAndReclaim(sessionID string) int {
	host.H.T.Helper()
	host.H.Clock.Advance(time.Duration(host.Cfg.Transport.LeaseSeconds+1) * time.Second)
	reclaimed, err := core.ReclaimExpiredLeases(host.H.Root, sessionID)
	if err != nil {
		host.H.T.Fatalf("FakeOrchestrationHost.ExpireLeasesAndReclaim: %v", err)
	}
	return reclaimed
}

func decodeStep(h *Harness, raw string) core.OrchestrationStepResult {
	h.T.Helper()
	var step core.OrchestrationStepResult
	if err := json.Unmarshal([]byte(raw), &step); err != nil {
		h.T.Fatalf("decode orchestration step: %v\n%s", err, raw)
	}
	return step
}

func decodeProgramStep(h *Harness, raw string) core.ProgramStepResult {
	h.T.Helper()
	var step core.ProgramStepResult
	if err := json.Unmarshal([]byte(raw), &step); err != nil {
		h.T.Fatalf("decode program step: %v\n%s", err, raw)
	}
	return step
}

func workerID(decision core.OrchestrationDecision) string {
	return strings.ToLower(decision.TaskID) + "-a" + strconv.Itoa(decision.Attempt)
}
