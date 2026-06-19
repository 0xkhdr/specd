package testharness

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

// FakePinkyWorker is a deterministic host worker for integration tests. It uses
// the public Pinky CLI for host-facing lease/report operations and core's public
// evidence reconciler for the Brain-side acceptance path. It performs no network
// calls and relies on the harness clock for stable timestamps.
type FakePinkyWorker struct {
	H        *Harness
	WorkerID string
	Cfg      core.OrchestrationCfg
}

func NewFakePinkyWorker(h *Harness, workerID string) *FakePinkyWorker {
	h.T.Helper()
	return &FakePinkyWorker{H: h, WorkerID: workerID, Cfg: core.LoadConfig(h.Root).Orchestration}
}

func (w *FakePinkyWorker) Claim(mission core.PinkyMission) core.PinkyClaim {
	w.H.T.Helper()
	path := w.writeMission(mission)
	res := w.H.RunExpect(core.ExitOK, "pinky", "claim", "--mission", path, "--json")
	var claim core.PinkyClaim
	if err := json.Unmarshal([]byte(res.Stdout), &claim); err != nil {
		w.H.T.Fatalf("FakePinkyWorker.Claim: decode: %v\n%s", err, res.Stdout)
	}
	return claim
}

func (w *FakePinkyWorker) Heartbeat(mission core.PinkyMission) Result {
	w.H.T.Helper()
	return w.H.Run("pinky", "heartbeat", "--session", mission.SessionID, "--worker", mission.WorkerID, "--attempt", strconv.Itoa(mission.Attempt), "--json")
}

func (w *FakePinkyWorker) Progress(mission core.PinkyMission, percent int, message string) Result {
	w.H.T.Helper()
	return w.H.Run("pinky", "progress", "--session", mission.SessionID, "--worker", mission.WorkerID, "--spec", mission.Spec, "--task", mission.TaskID, "--attempt", strconv.Itoa(mission.Attempt), "--percent", strconv.Itoa(percent), "--message", message, "--json")
}

func (w *FakePinkyWorker) Block(mission core.PinkyMission, reason string) Result {
	w.H.T.Helper()
	return w.H.Run("pinky", "block", "--session", mission.SessionID, "--worker", mission.WorkerID, "--spec", mission.Spec, "--task", mission.TaskID, "--attempt", strconv.Itoa(mission.Attempt), "--reason", reason, "--json")
}

func (w *FakePinkyWorker) Release(mission core.PinkyMission) Result {
	w.H.T.Helper()
	return w.H.Run("pinky", "release", "--session", mission.SessionID, "--worker", mission.WorkerID, "--attempt", strconv.Itoa(mission.Attempt))
}

func (w *FakePinkyWorker) RunVerify(mission core.PinkyMission) (Result, *core.VerificationRecord) {
	w.H.T.Helper()
	res := w.H.Run("verify", mission.Spec, mission.TaskID)
	loaded, err := core.LoadSpec(w.H.Root, mission.Spec)
	if err != nil {
		w.H.T.Fatalf("FakePinkyWorker.RunVerify: load spec: %v", err)
	}
	rec := loaded.State.Tasks[mission.TaskID].Verification
	return res, rec
}

func (w *FakePinkyWorker) SeedVerification(slug, taskID string, rec *core.VerificationRecord) {
	w.H.T.Helper()
	loaded, err := core.LoadSpec(w.H.Root, slug)
	if err != nil {
		w.H.T.Fatalf("FakePinkyWorker.SeedVerification: load spec: %v", err)
	}
	task := loaded.State.Tasks[taskID]
	task.Verification = rec
	loaded.State.Tasks[taskID] = task
	if err := core.SaveState(w.H.Root, slug, loaded.State); err != nil {
		w.H.T.Fatalf("FakePinkyWorker.SeedVerification: save state: %v", err)
	}
}

func (w *FakePinkyWorker) ReportVerified(mission core.PinkyMission, rec *core.VerificationRecord, summary string) (core.PinkyEvidenceResult, Result, error) {
	w.H.T.Helper()
	report := core.PinkyTerminalReport{
		SessionID:       mission.SessionID,
		WorkerID:        mission.WorkerID,
		Spec:            mission.Spec,
		TaskID:          mission.TaskID,
		Attempt:         mission.Attempt,
		VerificationRef: core.VerificationRef(rec),
		Summary:         summary,
		ChangedFiles:    append([]string{}, rec.ChangedFiles...),
		GitHead:         verificationHead(rec),
		DurationMs:      100,
		HostTokens:      10,
		HostCost:        "0.00",
	}
	res := w.H.Run("pinky", "report", "--session", report.SessionID, "--worker", report.WorkerID, "--spec", report.Spec, "--task", report.TaskID, "--attempt", strconv.Itoa(report.Attempt), "--verification-ref", report.VerificationRef, "--summary", report.Summary, "--changed-files", strings.Join(report.ChangedFiles, ","), "--git-head", report.GitHead, "--duration-ms", strconv.FormatInt(report.DurationMs, 10), "--host-tokens", strconv.Itoa(report.HostTokens), "--host-cost", report.HostCost, "--json")
	if res.Code != core.ExitOK {
		return core.PinkyEvidenceResult{}, res, nil
	}
	accepted, err := core.ReconcilePinkyEvidence(w.H.Root, report, w.Cfg)
	return accepted, res, err
}

func (w *FakePinkyWorker) AcknowledgeCancel(mission core.PinkyMission, reason string) (core.ACPEnvelope, error) {
	w.H.T.Helper()
	return core.AcknowledgePinkyCancellation(w.H.Root, mission.SessionID, mission.WorkerID, mission.Spec, mission.TaskID, mission.Attempt, reason, w.Cfg)
}

func (w *FakePinkyWorker) writeMission(mission core.PinkyMission) string {
	w.H.T.Helper()
	raw, err := json.MarshalIndent(mission, "", "  ")
	if err != nil {
		w.H.T.Fatalf("FakePinkyWorker.writeMission: encode: %v", err)
	}
	dir := filepath.Join(w.H.Root, ".specd", "tmp")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		w.H.T.Fatalf("FakePinkyWorker.writeMission: mkdir: %v", err)
	}
	file, err := os.CreateTemp(dir, "mission-*.json")
	if err != nil {
		w.H.T.Fatalf("FakePinkyWorker.writeMission: create: %v", err)
	}
	if _, err := file.Write(append(raw, '\n')); err != nil {
		_ = file.Close()
		w.H.T.Fatalf("FakePinkyWorker.writeMission: write: %v", err)
	}
	if err := file.Close(); err != nil {
		w.H.T.Fatalf("FakePinkyWorker.writeMission: close: %v", err)
	}
	return file.Name()
}

func verificationHead(rec *core.VerificationRecord) string {
	if rec == nil || rec.GitHead == nil {
		return ""
	}
	return *rec.GitHead
}
