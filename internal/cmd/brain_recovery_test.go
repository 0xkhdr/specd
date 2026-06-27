package cmd_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
	"github.com/0xkhdr/specd/internal/worker"
)

// errSimulatedDeath is the sentinel a recordingRunner returns when it simulates
// a worker process dying mid-run while holding its lease.
var errSimulatedDeath = errors.New("recordingRunner: simulated worker death")

// dispatchKey is one task attempt handed to the runner — the unit that must
// never be dispatched twice.
type dispatchKey struct {
	Task    string
	Attempt int
}

// recordingRunner is a test-only worker.Runner injected at the cmd/driver seam.
// It records every mission it is asked to run and, for a designated target task,
// simulates a crash on the first attempt (claims the lease, then returns a
// sentinel without reporting). Every other run completes the task with passing
// verified evidence — the same state transition the golden core driver test
// uses — so the drive reaches a terminal outcome without spawning real `sh`.
type recordingRunner struct {
	root string
	cfg  core.OrchestrationCfg

	mu         sync.Mutex
	seen       []dispatchKey
	killTask   string          // task to die on (first attempt only)
	killed     map[string]bool // tasks already crashed once
	noComplete bool            // when set, never complete — leave the drive to stall
}

func newRecordingRunner(root string, killTask string) *recordingRunner {
	return &recordingRunner{
		root:     root,
		cfg:      core.LoadConfig(root).Orchestration,
		killTask: killTask,
		killed:   map[string]bool{},
	}
}

func (r *recordingRunner) Run(_ context.Context, m worker.Mission) (worker.Result, error) {
	pm, ok := m.Payload.(core.PinkyMission)
	if !ok {
		return worker.Result{}, errors.New("recordingRunner: mission payload is not a core.PinkyMission")
	}

	r.mu.Lock()
	r.seen = append(r.seen, dispatchKey{Task: pm.TaskID, Attempt: pm.Attempt})
	die := pm.TaskID == r.killTask && !r.killed[pm.TaskID]
	if die {
		r.killed[pm.TaskID] = true
	}
	r.mu.Unlock()

	if die {
		// A real worker self-claims its lease, then the process dies before it
		// can report. Leave the lease for the driver/restart to reclaim.
		store, err := core.NewACPStore(r.root)
		if err != nil {
			return worker.Result{}, err
		}
		leaseDur := time.Duration(r.cfg.Transport.LeaseSeconds) * time.Second
		if _, err := store.ClaimLease(pm.SessionID, pm.WorkerID, pm.Spec, pm.TaskID, pm.Attempt, leaseDur, core.Clock().UTC().Add(time.Hour)); err != nil {
			return worker.Result{}, err
		}
		return worker.Result{}, errSimulatedDeath
	}

	if r.noComplete {
		// Report success without finishing the task, so the drive makes no real
		// progress and the session stays running (for resume-path coverage).
		return worker.Result{}, nil
	}

	return worker.Result{}, completeTask(r.root, pm)
}

// observed returns a copy of the recorded dispatch keys under the lock.
func (r *recordingRunner) observed() []dispatchKey {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]dispatchKey(nil), r.seen...)
}

// completeTask marks a task done with a passing verified record and moves the
// spec to verifying — the same transition the core driver golden test performs
// in lieu of real creative work, so the reference loop can converge.
func completeTask(root string, pm core.PinkyMission) error {
	st, err := core.LoadState(root, pm.Spec)
	if err != nil {
		return err
	}
	ts := st.Tasks[pm.TaskID]
	ts.Status = core.TaskComplete
	ts.Verification = &core.VerificationRecord{Verified: true, ExitCode: 0, Command: "true"}
	st.Tasks[pm.TaskID] = ts
	st.Status = core.StatusVerifying
	st.Phase = core.PhaseForStatus(core.StatusVerifying)
	return core.SaveState(root, pm.Spec, st)
}

func recoverySpec(h *testharness.Harness, slug string) string {
	h.Init()
	return h.Spec(slug).
		Req("recover", "As an operator I can recover from worker crashes.", "THE SYSTEM SHALL reclaim leases held by dead workers.").
		FullDesign().
		Status(core.StatusExecuting).
		Orchestrated().
		AddTask(testharness.TaskSpec{ID: "T1"}).
		Build()
}

func assertNoDoubleDispatch(t *testing.T, keys []dispatchKey) {
	t.Helper()
	seen := map[dispatchKey]int{}
	for _, k := range keys {
		seen[k]++
		if seen[k] > 1 {
			t.Fatalf("task %s attempt %d dispatched %d times — double dispatch: %v", k.Task, k.Attempt, seen[k], keys)
		}
	}
}

func activeLeaseCount(t *testing.T, root, slug, sessionID string) int {
	t.Helper()
	policy, err := core.NewOrchestrationPolicy(core.LoadConfig(root).Orchestration)
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	snap, err := core.SenseOrchestration(root, slug, sessionID, policy)
	if err != nil {
		t.Fatalf("SenseOrchestration: %v", err)
	}
	return len(snap.ActiveLeases)
}

// TestBrainDriverKillReclaimsLease (W1.8): a worker dies on its first attempt
// holding a lease; the live driver reclaims it, retries, and converges — with no
// lease left dangling.
func TestBrainDriverKillReclaimsLease(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "kill-reclaim")
	sessionID := repeat("a")

	rec := newRecordingRunner(h.Root, "T1")
	defer cmd.SetBrainRunner(rec)()

	h.RunExpect(core.ExitOK, "brain", "run", slug,
		"--session", sessionID, "--worker-cmd", "true",
		"--max-workers", "1", "--max-retries", "2")

	session, err := core.LoadOrchestrationSession(h.Root, sessionID)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if session.Status != core.OrchestrationSessionComplete {
		t.Fatalf("session status = %s, want complete", session.Status)
	}
	if n := activeLeaseCount(t, h.Root, slug, sessionID); n != 0 {
		t.Fatalf("active leases after reclaim = %d, want 0", n)
	}
	keys := rec.observed()
	assertNoDoubleDispatch(t, keys)
	if len(keys) < 2 {
		t.Fatalf("expected at least a crashed attempt and a retry, got %v", keys)
	}
}

// TestBrainDriverNoDoubleDispatchAcrossKillResume (W1.9): a crash in one drive
// followed by a fresh drive of the same session must never dispatch the same
// task attempt twice.
func TestBrainDriverNoDoubleDispatchAcrossKillResume(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "no-double")
	sessionID := repeat("b")

	rec := newRecordingRunner(h.Root, "T1")
	defer cmd.SetBrainRunner(rec)()

	// Drive #1 bounded to a single step: dispatch T1, the worker dies, the drive
	// stops. The driver reclaims the dead worker's lease on drain.
	h.RunExpect(core.ExitOK, "brain", "run", slug,
		"--session", sessionID, "--worker-cmd", "true",
		"--max-workers", "1", "--max-retries", "2", "--max-steps", "1")

	// Drive #2 (fresh driver, same session): re-dispatch and converge.
	h.RunExpect(core.ExitOK, "brain", "run", slug,
		"--session", sessionID, "--worker-cmd", "true",
		"--max-workers", "1", "--max-retries", "2")

	session, err := core.LoadOrchestrationSession(h.Root, sessionID)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	if session.Status != core.OrchestrationSessionComplete {
		t.Fatalf("session status = %s, want complete", session.Status)
	}
	assertNoDoubleDispatch(t, rec.observed())
	if n := activeLeaseCount(t, h.Root, slug, sessionID); n != 0 {
		t.Fatalf("active leases after resume = %d, want 0", n)
	}
}

// TestBrainResumeIdempotent: resuming an already-reconciled session twice yields
// byte-identical session state and writes no new events.
func TestBrainResumeIdempotent(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "idempotent")
	sessionID := repeat("c")

	rec := newRecordingRunner(h.Root, "")
	defer cmd.SetBrainRunner(rec)()

	h.RunExpect(core.ExitOK, "brain", "run", slug,
		"--session", sessionID, "--worker-cmd", "true", "--max-workers", "1")

	// Reconciliation must converge to a fixed point: resuming a terminal session
	// changes neither the reconciliation cursor (lastSequence), the event log, nor
	// dispatches new work — only the heartbeat (updatedAt) moves.
	wantSeq, wantStatus := sessionCursor(t, h.Root, sessionID)
	wantEvents := eventFileCount(t, h.Root, sessionID)
	dispatchedBefore := len(rec.observed())

	for i := 0; i < 2; i++ {
		h.RunExpect(core.ExitOK, "brain", "resume", "--session", sessionID, "--json")
		gotSeq, gotStatus := sessionCursor(t, h.Root, sessionID)
		if gotSeq != wantSeq || gotStatus != wantStatus {
			t.Fatalf("resume %d moved reconciliation: seq %d→%d status %s→%s", i+1, wantSeq, gotSeq, wantStatus, gotStatus)
		}
		if got := eventFileCount(t, h.Root, sessionID); got != wantEvents {
			t.Fatalf("resume %d wrote new events: before=%d after=%d", i+1, wantEvents, got)
		}
	}
	if after := len(rec.observed()); after != dispatchedBefore {
		t.Fatalf("resume dispatched new work: before=%d after=%d", dispatchedBefore, after)
	}
}

// TestBrainCheckpointResumeNoWorkRedone (W1.10) proves the full checkpoint→resume
// loop: a worker reaches 70%, checkpoints (shedding context), and the Brain then
// prefers resume-from-checkpoint over a fresh dispatch — handing a fresh worker
// the prior progress and notes, with the same attempt's lease released and
// re-issued exactly once. No work is re-done because the brief says "resume".
func TestBrainCheckpointResumeNoWorkRedone(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "checkpoint-resume")
	sessionID := repeat("d")

	cfg := core.LoadConfig(h.Root).Orchestration
	cfg.Enabled = true
	cfg.Resilience = &core.ResilienceCfg{CheckpointEnabled: true}
	policy, err := core.NewOrchestrationPolicy(cfg)
	if err != nil {
		t.Fatalf("policy: %v", err)
	}
	if _, err := core.StartOrchestrationSession(h.Root, slug, sessionID, "test", policy); err != nil {
		t.Fatalf("StartOrchestrationSession: %v", err)
	}

	// 1. First decision dispatches T1 attempt 1 (no checkpoint yet).
	snap, err := core.SenseOrchestration(h.Root, slug, sessionID, policy)
	if err != nil {
		t.Fatalf("sense: %v", err)
	}
	dec, err := core.DecideOrchestration(snap, policy)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if dec.Action != core.OrchestrationDispatch || dec.TaskID != "T1" || dec.Attempt != 1 {
		t.Fatalf("first decision = %s %s/%d, want dispatch T1/1", dec.Action, dec.TaskID, dec.Attempt)
	}

	// 2. Worker claims, reports 70%, then checkpoints and sheds context.
	mission, err := core.BuildPinkyMission(h.Root, slug, sessionID, "pinky-a", "T1", 1, cfg)
	if err != nil {
		t.Fatalf("build mission: %v", err)
	}
	if mission.Resume != nil {
		t.Fatal("fresh dispatch mission must not carry a resume payload")
	}
	if _, err := core.ClaimPinkyMission(h.Root, mission, cfg); err != nil {
		t.Fatalf("claim: %v", err)
	}
	if _, err := core.RecordPinkyProgress(h.Root, core.PinkyProgressReport{
		SessionID: sessionID, WorkerID: "pinky-a", Spec: slug, TaskID: "T1",
		Attempt: 1, Percent: 70, Message: "parser written, tests pending",
	}, cfg); err != nil {
		t.Fatalf("progress: %v", err)
	}
	if _, err := core.RecordCheckpoint(h.Root, core.CheckpointRecord{
		SessionID: sessionID, Spec: slug, TaskID: "T1", Attempt: 1, WorkerID: "pinky-a",
		ProgressPercent: 70, WorkingNotes: "wrote the parser, tests pending",
		ChangedFiles: []string{"internal/core/demo.go"}, Reason: "host /clear",
	}, cfg); err != nil {
		t.Fatalf("checkpoint: %v", err)
	}

	// 3. Brain senses the checkpoint with no active lease and decides resume.
	snap2, err := core.SenseOrchestration(h.Root, slug, sessionID, policy)
	if err != nil {
		t.Fatalf("sense after checkpoint: %v", err)
	}
	if len(snap2.ActiveLeases) != 0 {
		t.Fatalf("checkpoint must release the lease, got %d active", len(snap2.ActiveLeases))
	}
	if len(snap2.Checkpoints) != 1 || snap2.Checkpoints[0].TaskID != "T1" || snap2.Checkpoints[0].ProgressPercent != 70 {
		t.Fatalf("snapshot checkpoints = %#v, want one T1@70%%", snap2.Checkpoints)
	}
	dec2, err := core.DecideOrchestration(snap2, policy)
	if err != nil {
		t.Fatalf("decide after checkpoint: %v", err)
	}
	if dec2.Action != core.OrchestrationResume || dec2.TaskID != "T1" || dec2.Attempt != 1 {
		t.Fatalf("second decision = %s %s/%d, want resume-from-checkpoint T1/1", dec2.Action, dec2.TaskID, dec2.Attempt)
	}

	// 4. The fresh worker's brief carries the prior progress and notes — and the
	//    same attempt's lease is re-issued exactly once.
	resumeMission, err := core.BuildPinkyMission(h.Root, slug, sessionID, "pinky-b", "T1", 1, cfg)
	if err != nil {
		t.Fatalf("build resume mission: %v", err)
	}
	if resumeMission.Resume == nil || resumeMission.Resume.ProgressPercent != 70 {
		t.Fatalf("resume mission must carry the 70%% checkpoint, got %#v", resumeMission.Resume)
	}
	brief := core.RenderMissionBrief(resumeMission)
	if !strings.Contains(brief, "Resuming from checkpoint") || !strings.Contains(brief, "wrote the parser") {
		t.Fatalf("resume brief missing resume header/notes:\n%s", brief)
	}
	if _, err := core.ClaimPinkyMission(h.Root, resumeMission, cfg); err != nil {
		t.Fatalf("resume re-claim of the same attempt must succeed exactly once: %v", err)
	}
}

func sessionCursor(t *testing.T, root, sessionID string) (uint64, core.OrchestrationSessionStatus) {
	t.Helper()
	session, err := core.LoadOrchestrationSession(root, sessionID)
	if err != nil {
		t.Fatalf("load session: %v", err)
	}
	return session.LastSequence, session.Status
}

func repeat(s string) string {
	out := make([]byte, 32)
	for i := range out {
		out[i] = s[0]
	}
	return string(out)
}
