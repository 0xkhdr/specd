package cmd

import (
	"encoding/json"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

func TestBrainFailClosedRequiresOrchestrationConfig(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", "")

	err := runBrain(root, []string{"start", "demo"}, map[string]string{})
	if err == nil {
		t.Fatal("expected missing config precondition")
	}
	if !strings.Contains(err.Error(), "orchestration.enabled") {
		t.Fatalf("expected orchestration.enabled error, got %v", err)
	}
	assertNoBrainSession(t, root)
}

func TestBrainFailClosedRequiresOrchestratedSpecMode(t *testing.T) {
	root := newBrainTestRoot(t, "default", "orchestration:\n  enabled: true\n")

	err := runBrain(root, []string{"start", "demo"}, map[string]string{})
	if err == nil {
		t.Fatal("expected mode precondition")
	}
	if !strings.Contains(err.Error(), "spec mode must be orchestrated") {
		t.Fatalf("expected spec mode error, got %v", err)
	}
	assertNoBrainSession(t, root)
}

func TestBrainStartCreatesSessionWhenPreconditionsPass(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", "orchestration:\n  enabled: true\n")

	if err := runBrain(root, []string{"start", "demo"}, map[string]string{}); err != nil {
		t.Fatalf("brain start: %v", err)
	}
	if _, err := os.Stat(brainSessionPath(root)); err != nil {
		t.Fatalf("expected session: %v", err)
	}
}

func TestBrainDispatchCreatesPendingMissionWithoutWorkerLease(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	if err := os.WriteFile(filepath.Join(root, ".specd/specs/demo/tasks.md"), []byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"step", "demo"}, map[string]string{"authority": ""}); err != nil {
		t.Fatal(err)
	}
	s := loadBrainSession(t, root)
	if len(s.PendingMissions) != 1 || s.PendingMissions[0].Status != orchestration.MissionPending {
		t.Fatalf("pending missions = %+v", s.PendingMissions)
	}
	if len(s.Leases) != 0 {
		t.Fatalf("controller minted worker lease: %+v", s.Leases)
	}
	events, err := orchestration.ReadACP(filepath.Join(root, ".specd/specs/demo/acp.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || !strings.Contains(events[0].Payload, `"status":"pending"`) {
		t.Fatalf("ACP = %+v", events)
	}
}

func TestBrainWaitsWhenActiveHarnessHasNoWorker(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	writeBrainSingleTask(t, root)
	if err := os.Remove(filepath.Join(root, ".codex", "agents", "pinky-craftsman.toml")); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error {
		return runBrain(root, []string{"step", "demo"}, map[string]string{"authority": "true"})
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, orchestration.ReasonWaitNoWorker) {
		t.Fatalf("brain no-worker output = %q", out)
	}
	if got := loadBrainSession(t, root).PendingMissions; len(got) != 0 {
		t.Fatalf("brain dispatched without active-harness worker: %+v", got)
	}
}

func TestBrainStatusReportsPreciseWorkerStates(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", "orchestration:\n  enabled: true\n")
	now := time.Now()
	session := orchestration.Session{
		ID:       "demo",
		Revision: 1,
		State:    orchestration.SessionRunning,
		Step:     2,
		Leases: []orchestration.Lease{
			{TaskID: "T1", WorkerID: "worker-active", ExpiresAt: now.Add(time.Hour)},
			{TaskID: "T2", WorkerID: "worker-expired", ExpiresAt: now.Add(-time.Hour)},
		},
	}
	if err := orchestration.SaveSessionCAS(root, brainSessionPath(root), 0, session); err != nil {
		t.Fatal(err)
	}

	out, err := captureStdout(t, func() error {
		return brainStatus(brainSessionPath(root), filepath.Join(root, ".specd", "specs", "demo", "checkpoint.json"), filepath.Join(root, ".specd", "specs", "demo", "acp.jsonl"), "demo")
	})
	if err != nil {
		t.Fatal(err)
	}
	var view brainStatusView
	if err := json.Unmarshal([]byte(out), &view); err != nil {
		t.Fatal(err)
	}
	if view.WorkerStates["active"] != 1 || view.WorkerStates["expired"] != 1 {
		t.Fatalf("worker states=%v", view.WorkerStates)
	}
	if view.Leases[0].State == "" || view.Leases[1].State == "" {
		t.Fatalf("leases missing states: %#v", view.Leases)
	}
}

// TestBrainStepReleasesCompletedLease pins gap 5.4 / R5: one runBrainStep releases
// the lease of a task that has reached TaskComplete, so it stops showing as a
// phantom live worker. Covers the in-step completion branch directly, not the
// transitive `brain resume` clearing the stress script exercises.
func TestBrainStepReleasesCompletedLease(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", "orchestration:\n  enabled: true\n")
	specDir := filepath.Join(root, ".specd", "specs", "demo")
	tasks := "| id | role | files | depends-on | verify | acceptance |\n" +
		"|---|---|---|---|---|---|\n" +
		"| ✅ T1 | craftsman | a.txt | - | printf done | done |\n"
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	session := orchestration.Session{
		ID:     "demo",
		State:  orchestration.SessionRunning,
		Leases: []orchestration.Lease{{TaskID: "T1", WorkerID: "w1", ExpiresAt: time.Now().Add(time.Hour)}},
	}
	if err := orchestration.SaveSessionCAS(root, brainSessionPath(root), 0, session); err != nil {
		t.Fatal(err)
	}

	if _, err := captureStdout(t, func() error {
		_, err := runBrainStep(root, brainSessionPath(root), filepath.Join(specDir, "acp.jsonl"), orchestration.CheckpointPath(root, "demo"), "demo", map[string]string{}, "step")
		return err
	}); err != nil {
		t.Fatalf("brain step: %v", err)
	}

	got, err := orchestration.LoadSession(brainSessionPath(root))
	if err != nil {
		t.Fatal(err)
	}
	for _, lease := range got.Leases {
		if lease.TaskID == "T1" {
			t.Fatalf("lease on completed T1 not released: %#v", got.Leases)
		}
	}
}

// TestBrainRunHaltsOnConfiguredCostBrake pins R4.2: a configured cost threshold
// halts only subsequent dispatch, with the exact reason, minting no lease and
// undoing nothing already on the ledger.
func TestBrainRunHaltsOnConfiguredCostBrake(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig+"routing:\n  max_cost_micros: 100\n")
	writeBrainSingleTask(t, root)
	acpPath := filepath.Join(root, ".specd", "specs", "demo", "acp.jsonl")
	if err := orchestration.AppendACP(acpPath, orchestration.ACPEvent{Kind: orchestration.ACPKindReport, TaskID: "T1", Observation: &orchestration.ObservationV1{Version: "1", Known: true, Source: "host", Unit: "micro-usd", CostMicros: 200}}); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	out, err := captureStdout(t, func() error {
		return runBrain(root, []string{"run", "demo"}, map[string]string{"authority": ""})
	})
	// The brake is permanent, so the run reports non-success (R6.3) while still
	// printing the exact reason.
	if err == nil {
		t.Fatal("cost brake reported success")
	}
	var refusal core.Refusal
	if !errors.As(err, &refusal) || refusal.Code != "BRAIN_ZERO_PROGRESS" {
		t.Fatalf("want BRAIN_ZERO_PROGRESS refusal, got %T: %v", err, err)
	}
	if !strings.Contains(out, "halt") || !strings.Contains(out, "cost limit exceeded") {
		t.Fatalf("expected cost brake halt reason, got %q", out)
	}
	s := loadBrainSession(t, root)
	if len(s.PendingMissions) != 0 || len(s.Leases) != 0 {
		t.Fatalf("cost brake dispatched or leased: %+v", s)
	}
	events, err := orchestration.ReadACP(acpPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 || events[0].Kind != orchestration.ACPKindReport {
		t.Fatalf("cost brake mutated the ledger: %+v", events)
	}
}

// TestBrainRunWithoutLimitsDispatches proves the R4.2 backstop: with no
// configured limit the controller dispatches exactly as it does today.
func TestBrainRunWithoutLimitsDispatches(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	writeBrainSingleTask(t, root)
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := captureStdout(t, func() error {
		return runBrain(root, []string{"run", "demo"}, map[string]string{"authority": ""})
	}); err != nil {
		t.Fatal(err)
	}
	s := loadBrainSession(t, root)
	if len(s.PendingMissions) != 1 {
		t.Fatalf("expected one dispatch without limits, got %+v", s.PendingMissions)
	}
}

func writeBrainSingleTask(t *testing.T, root string) {
	t.Helper()
	tasks := "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n"
	if err := os.WriteFile(filepath.Join(root, ".specd", "specs", "demo", "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
}

func newBrainTestRoot(t *testing.T, mode, projectConfig string) string {
	t.Helper()
	root := t.TempDir()
	if err := core.WriteScaffold(root, "pinky"); err != nil {
		t.Fatal(err)
	}
	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	state := `{"schema_version":1,"slug":"demo","mode":"` + mode + `","status":"design","phase":"analyze","revision":1,"records":{}}`
	if err := os.WriteFile(filepath.Join(specDir, "state.json"), []byte(state), 0o644); err != nil {
		t.Fatal(err)
	}
	if projectConfig != "" {
		if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte(projectConfig), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func assertNoBrainSession(t *testing.T, root string) {
	t.Helper()
	if _, err := os.Stat(brainSessionPath(root)); !os.IsNotExist(err) {
		t.Fatalf("expected no session, stat err=%v", err)
	}
}

func brainSessionPath(root string) string {
	return filepath.Join(root, ".specd", "specs", "demo", "session.json")
}

// A dispatched mission reserves its task even before a worker claims it. Two
// consecutive steps with no claim in between must advance to the second task
// rather than re-issuing the first, which filtering on leases alone did.
func TestBrainStepDoesNotRedispatchUnclaimedMission(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	tasks := "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n" +
		"| T1 | craftsman | a.go | - | printf ok | R1 |\n" +
		"| T2 | craftsman | b.go | - | printf ok | R1 |\n"
	if err := os.WriteFile(filepath.Join(root, ".specd/specs/demo/tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := runBrain(root, []string{"step", "demo"}, map[string]string{"authority": ""}); err != nil {
			t.Fatal(err)
		}
	}
	s := loadBrainSession(t, root)
	if len(s.PendingMissions) != 2 {
		t.Fatalf("pending missions = %d, want 2: %+v", len(s.PendingMissions), s.PendingMissions)
	}
	dispatched := map[string]bool{}
	for _, mission := range s.PendingMissions {
		if dispatched[mission.TaskID] {
			t.Fatalf("task %s dispatched twice while unclaimed: %+v", mission.TaskID, s.PendingMissions)
		}
		dispatched[mission.TaskID] = true
	}
}

// TestMissionLifecycleJourney exercises the full production orchestration journey:
// dispatch, claim, verify, report, complete without profile changes, manual ledger
// edits, or TTL waits (R6.1). Preserves session/lease/authority bindings (R4.5).
func TestMissionLifecycleJourney(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	tasks := "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n"
	if err := os.WriteFile(filepath.Join(root, ".specd/specs/demo/tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "a.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Initialize git repo so evidence can be pinned to HEAD
	execGit(t, root, "init")
	execGit(t, root, "config", "user.email", "test@test.com")
	execGit(t, root, "config", "user.name", "Test")
	execGit(t, root, "add", ".")
	execGit(t, root, "commit", "-m", "initial")

	// Dispatch: start session and run controller step
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatalf("brain start: %v", err)
	}
	if err := runBrain(root, []string{"step", "demo"}, map[string]string{"authority": ""}); err != nil {
		t.Fatalf("brain step: %v", err)
	}
	s := loadBrainSession(t, root)
	if len(s.PendingMissions) != 1 {
		t.Fatalf("expected one pending mission, got %+v", s.PendingMissions)
	}
	mission := s.PendingMissions[0]
	if mission.TaskID != "T1" {
		t.Fatalf("expected T1, got %s", mission.TaskID)
	}

	// Claim: worker claims the mission
	if err := runBrain(root, []string{"claim", "demo", mission.MissionID, "worker-1", "craftsman"}, nil); err != nil {
		t.Fatalf("brain claim: %v", err)
	}
	s = loadBrainSession(t, root)
	if len(s.Leases) != 1 || len(s.PendingMissions) != 0 {
		t.Fatalf("claim did not move mission to lease: sessions=%+v", s)
	}
	lease := s.Leases[0]

	// Heartbeat: worker keeps the lease alive
	if err := runBrain(root, []string{"heartbeat", "demo", lease.LeaseID, "worker-1"}, nil); err != nil {
		t.Fatalf("brain heartbeat: %v", err)
	}

	// Verify: append evidence of successful verification
	head := gitHead(root)
	if err := core.AppendEvidence(core.EvidencePath(root, "demo"), core.EvidenceRecord{
		TaskID: "T1", Command: "printf ok", ExitCode: 0, GitHead: head,
	}); err != nil {
		t.Fatalf("append evidence: %v", err)
	}

	// Report: worker reports completion (brain report also calls complete-task)
	if err := runBrain(root, []string{"report", "demo", lease.LeaseID, "worker-1"}, nil); err != nil {
		t.Fatalf("brain report: %v", err)
	}
	s = loadBrainSession(t, root)
	if len(s.Leases) != 0 {
		t.Fatalf("report did not release lease: %+v", s.Leases)
	}

	// Verify final state: task marker should show complete in tasks.md
	spec, err := loadSpec(root, "demo")
	if err != nil {
		t.Fatalf("load spec: %v", err)
	}
	for _, task := range spec.Tasks {
		if task.ID == "T1" {
			if task.Marker != "✅" && task.Marker != "done" && task.Marker != "complete" {
				t.Fatalf("expected task marker to be complete, got %q", task.Marker)
			}
			return
		}
	}
	t.Fatal("task T1 not found after completion")
}

// TestBrainRunZeroProgressIsNonSuccess pins R6.3: a permanently braked run is a
// distinct non-success outcome that names its durable checkpoint effects, not a
// silent exit 0 that reads like a finished run.
func TestBrainRunZeroProgressIsNonSuccess(t *testing.T) {
	// Requiring telemetry with no reports yet brakes on the very first step.
	config := brainEnabledConfig + "routing:\n  allow_unknown_telemetry: false\n"
	root := newBrainTestRoot(t, "orchestrated", config)
	writeBrainSingleTask(t, root)
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatalf("brain start: %v", err)
	}

	err := runBrain(root, []string{"run", "demo"}, map[string]string{"authority": ""})
	if err == nil {
		t.Fatal("a permanently braked run reported success")
	}
	var refusal core.Refusal
	if !errors.As(err, &refusal) {
		t.Fatalf("want a structured refusal, got %T: %v", err, err)
	}
	if refusal.Code != "BRAIN_ZERO_PROGRESS" {
		t.Errorf("refusal code = %q, want BRAIN_ZERO_PROGRESS", refusal.Code)
	}
	// Zero dispatches happened, so the run must not claim a mutation.
	if refusal.StateChanged {
		t.Errorf("no dispatch occurred but refusal claims a mutation: %+v", refusal)
	}
	if !strings.Contains(refusal.Error(), "halt") {
		t.Errorf("refusal does not name the braking action: %v", refusal)
	}

	// A wait is not a brake: with telemetry allowed and no authority to dispatch,
	// the same run ends legitimately rather than refusing.
	waiting := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	writeBrainSingleTask(t, waiting)
	if err := runBrain(waiting, []string{"start", "demo"}, nil); err != nil {
		t.Fatalf("brain start: %v", err)
	}
	if err := runBrain(waiting, []string{"run", "demo"}, nil); err != nil {
		t.Fatalf("a waiting run must not refuse: %v", err)
	}
}

// newControllerApprovalRoot builds a spec whose execution is finished — one
// task, marked complete — sitting at a lifecycle gate under an orchestrated
// controller. That is the exact situation R4 is about.
func newControllerApprovalRoot(t *testing.T) string {
	t.Helper()
	root := newDemoSpec(t)
	if err := os.WriteFile(filepath.Join(root, "project.yml"), []byte(brainEnabledConfig+"delegation.enabled: true\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTasks(t, root, "demo", "| T1 | scout | spec.md | - | printf ok | controller fixture |")
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-m", "controller fixture")
	for range 2 {
		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("fixture approve: %v", err)
		}
	}
	// The task is completed the only way a task ever is: passing evidence
	// pinned to HEAD, then the gated completion. A marker written by hand
	// would leave the readiness gates refusing, which is a different test.
	if err := Run(root, "verify", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("fixture verify: %v", err)
	}
	if err := Run(root, "complete-task", []string{"demo", "T1"}, nil); err != nil {
		t.Fatalf("fixture complete-task: %v", err)
	}
	if err := Run(root, "mode", []string{"demo", "orchestrated"}, nil); err != nil {
		t.Fatalf("fixture mode: %v", err)
	}
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatalf("brain start: %v", err)
	}
	return root
}

func controllerSession(t *testing.T, root string) orchestration.Session {
	t.Helper()
	session, err := orchestration.LoadSession(brainSessionPath(root))
	if err != nil {
		t.Fatal(err)
	}
	return session
}

// runController runs the controller and returns its refusal (if any) plus the
// printed decision line.
func runController(t *testing.T, root string, flags map[string]string) (string, error) {
	t.Helper()
	all := map[string]string{"authority": ""}
	for key, value := range flags {
		all[key] = value
	}
	var runErr error
	out, err := captureStdout(t, func() error {
		runErr = runBrain(root, []string{"run", "demo"}, all)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out, runErr
}

// TestControllerApprovalHandoff pins R4.1 to R4.4 at the controller: reaching a
// lifecycle gate is a distinct, reportable, non-success halt that names who must
// act; either approval route resumes the same session; an expired, revoked, or
// out-of-scope grant falls back to the human route instead of proceeding; and
// the controller never mints or spends a grant itself.
func TestControllerApprovalHandoff(t *testing.T) {
	t.Run("nogranthaltsforahuman", func(t *testing.T) {
		root := newControllerApprovalRoot(t)
		before := controllerSession(t, root)

		out, err := runController(t, root, nil)
		if err == nil {
			t.Fatal("a run that reached an unapproved lifecycle gate reported success")
		}
		refusal, ok := core.AsRefusal(err)
		if !ok || refusal.Code != "APPROVAL_REQUIRED" {
			t.Fatalf("refusal = %v, want APPROVAL_REQUIRED", err)
		}
		if refusal.ActorRequired != core.RefusalActorHuman || refusal.RecoveryCommand != "specd approve demo" {
			t.Fatalf("refusal does not name the human route: %+v", refusal)
		}
		if refusal.StateChanged {
			t.Fatalf("a halt with no dispatch claims a mutation: %+v", refusal)
		}
		if !strings.Contains(out, string(orchestration.ActionWaitApproval)) || !strings.Contains(out, "tasks") {
			t.Fatalf("controller output does not name the waiting gate: %q", out)
		}

		// R4.3: the halt is a marker beside the session, not a reset of it.
		session := controllerSession(t, root)
		if session.WaitingApproval != "tasks" {
			t.Fatalf("session waiting_approval = %q, want the gate", session.WaitingApproval)
		}
		if session.ID != before.ID || session.Step != before.Step || len(session.Leases) != len(before.Leases) ||
			len(session.PendingMissions) != len(before.PendingMissions) || session.Status() != orchestration.SessionRunning {
			t.Fatalf("halt discarded controller progress: before=%+v after=%+v", before, session)
		}

		// The halt is visible where an operator looks for it.
		status, err := captureStdout(t, func() error { return Run(root, "status", []string{"demo"}, nil) })
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(status, "waiting_approval") || !strings.Contains(status, "specd delegate approve") {
			t.Fatalf("status does not surface the halt: %q", status)
		}
	})

	// R4.2: both routes converge — after either, the same session advances.
	t.Run("resumesafterhumanapproval", func(t *testing.T) {
		root := newControllerApprovalRoot(t)
		if _, err := runController(t, root, nil); err == nil {
			t.Fatal("expected the first run to halt")
		}
		halted := controllerSession(t, root)

		if err := Run(root, "approve", []string{"demo"}, nil); err != nil {
			t.Fatalf("human approve: %v", err)
		}
		if _, err := runController(t, root, nil); err == nil {
			t.Fatal("expected a halt at the next gate")
		}
		resumed := controllerSession(t, root)
		if resumed.WaitingApproval == halted.WaitingApproval {
			t.Fatalf("controller did not advance past the approved gate: %q", resumed.WaitingApproval)
		}
		if resumed.ID != halted.ID || resumed.Step != halted.Step {
			t.Fatalf("resume restarted the session: halted=%+v resumed=%+v", halted, resumed)
		}
	})

	t.Run("resumesafterdelegatedapproval", func(t *testing.T) {
		root := newControllerApprovalRoot(t)
		token := issueGrant(t, root, "demo", "controller-grant", map[string]string{"transitions": "approve.tasks"})

		out, err := runController(t, root, map[string]string{"grant": "controller-grant"})
		if err == nil {
			t.Fatal("a named grant made the controller approve by itself")
		}
		refusal, _ := core.AsRefusal(err)
		if refusal.ActorRequired != core.RefusalActorOperator || !strings.Contains(refusal.RecoveryCommand, "specd delegate approve demo --grant controller-grant") {
			t.Fatalf("valid grant did not surface the delegated route: %+v", refusal)
		}
		if !strings.Contains(out, "delegate approve") {
			t.Fatalf("controller output omits the delegated route: %q", out)
		}
		// R4.4: naming a route is not spending one.
		projection, err := core.LoadGrant(root, "controller-grant")
		if err != nil {
			t.Fatal(err)
		}
		if projection.Uses() != 0 {
			t.Fatalf("the controller spent a grant use: %+v", projection)
		}
		halted := controllerSession(t, root)

		if err := Run(root, "delegate", []string{"approve", "demo"},
			map[string]string{"grant": "controller-grant", "token": token, "reason": "unattended"}); err != nil {
			t.Fatalf("delegated approve: %v", err)
		}
		if _, err := runController(t, root, map[string]string{"grant": "controller-grant"}); err == nil {
			t.Fatal("expected a halt at the next gate")
		}
		resumed := controllerSession(t, root)
		if resumed.WaitingApproval == halted.WaitingApproval {
			t.Fatalf("controller did not resume past the delegated approval: %q", resumed.WaitingApproval)
		}
		if used := grantUses(t, root, "controller-grant"); used.Uses() != 1 {
			t.Fatalf("uses = %d after one delegated approval, want 1", used.Uses())
		}
	})

	// R4.3: a grant that stops being valid while the controller waits must not
	// keep being offered, and must never become an approval.
	t.Run("midrunexpiryandrevocationfallback", func(t *testing.T) {
		for _, tc := range []struct {
			name       string
			invalidate func(t *testing.T, root string)
			flags      map[string]string
		}{
			{"expiry", func(t *testing.T, root string) {}, map[string]string{"expires-in": "1h", "issued-in-the-past": "2h"}},
			{"revocation", func(t *testing.T, root string) {
				if err := Run(root, "delegate", []string{"revoke", "stale-grant"}, map[string]string{"reason": "window closed"}); err != nil {
					t.Fatal(err)
				}
			}, nil},
		} {
			t.Run(tc.name, func(t *testing.T) {
				root := newControllerApprovalRoot(t)
				flags := map[string]string{"transitions": "approve.tasks"}
				for key, value := range tc.flags {
					flags[key] = value
				}
				// An expired grant is minted by issuing it in the past: the
				// grant's own clock is what makes it stale, not a test hook in
				// the controller.
				if rewind, ok := flags["issued-in-the-past"]; ok {
					delete(flags, "issued-in-the-past")
					offset, err := time.ParseDuration(rewind)
					if err != nil {
						t.Fatal(err)
					}
					restore := core.Clock
					core.Clock = func() time.Time { return time.Now().UTC().Add(-offset) }
					issueGrant(t, root, "demo", "stale-grant", flags)
					core.Clock = restore
				} else {
					issueGrant(t, root, "demo", "stale-grant", flags)
				}
				tc.invalidate(t, root)

				_, err := runController(t, root, map[string]string{"grant": "stale-grant"})
				if err == nil {
					t.Fatal("an unusable grant let the controller proceed")
				}
				refusal, _ := core.AsRefusal(err)
				if refusal.ActorRequired != core.RefusalActorHuman || refusal.RecoveryCommand != "specd approve demo" {
					t.Fatalf("unusable grant was still offered as a route: %+v", refusal)
				}
				if specState(t, root, "demo").Status != core.StatusTasks {
					t.Fatal("the controller advanced the lifecycle without an approval")
				}
			})
		}
	})

	// Gate drift outranks any authority: with failing gates no route approves.
	t.Run("gatedrift", func(t *testing.T) {
		root := newControllerApprovalRoot(t)
		issueGrant(t, root, "demo", "drift-grant", map[string]string{"transitions": "approve.tasks"})
		// Evidence disappears from under a completed task: the readiness gates
		// now refuse, and no authority can approve past that.
		if err := os.Remove(core.EvidencePath(root, "demo")); err != nil {
			t.Fatal(err)
		}

		out, err := runController(t, root, map[string]string{"grant": "drift-grant"})
		if err == nil {
			t.Fatal("a drifted spec reported success")
		}
		refusal, _ := core.AsRefusal(err)
		if refusal.RecoveryCommand != "specd check demo" {
			t.Fatalf("drifted gates still pointed at an approval route: %+v", refusal)
		}
		if !strings.Contains(out, "readiness gates refuse") {
			t.Fatalf("controller output does not name the drift: %q", out)
		}
		if used := grantUses(t, root, "drift-grant"); used.Uses() != 0 {
			t.Fatalf("drifted run touched the grant: %+v", used)
		}
	})

	// R4.4, structurally: the controller command path contains no call that
	// mints, revokes, or spends a grant, and holds no bearer token. This fails
	// the moment someone wires issuance or consumption into it.
	t.Run("controllerneverselfgrants", func(t *testing.T) {
		assertNoGrantMutation(t, "brain_run.go")
		root := newControllerApprovalRoot(t)
		issueGrant(t, root, "demo", "audit-grant", map[string]string{"transitions": "approve.tasks"})
		before, err := os.ReadFile(core.GrantLedgerPath(root))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := runController(t, root, map[string]string{"grant": "audit-grant", "token": "irrelevant"}); err == nil {
			t.Fatal("expected the controller to halt")
		}
		after, err := os.ReadFile(core.GrantLedgerPath(root))
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatal("the controller wrote to the grant ledger")
		}
	})
}

// assertNoGrantMutation parses one source file and fails if it names any
// delegation call that creates or spends authority. A comment promising the
// controller never self-grants is not evidence; this is.
func assertNoGrantMutation(t *testing.T, name string) {
	t.Helper()
	forbidden := map[string]bool{
		"IssueDelegationGrant": true, "RevokeDelegationGrant": true, "ReserveGrantUse": true,
		"ConsumeGrantUse": true, "ReleaseGrantUse": true, "NewDelegationToken": true,
		"DelegationTokenDigest": true, "approveSpec": true,
	}
	file, err := parser.ParseFile(token.NewFileSet(), name, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	ast.Inspect(file, func(node ast.Node) bool {
		ident, ok := node.(*ast.Ident)
		if ok && forbidden[ident.Name] {
			t.Errorf("%s reaches %s: the controller must never create, widen, or spend approval authority", name, ident.Name)
		}
		return true
	})
}

// TestWorkerOutOfScopeBrainPopulatesMissionWorker pins that the dispatcher
// copies the selected task row's worker column into the mission (spec R6.4
// population), so the claim-time scope check has a value to enforce.
func TestWorkerOutOfScopeBrainPopulatesMissionWorker(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	tasks := "| id | role | files | depends-on | verify | acceptance | worker |\n|---|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 | w1 |\n"
	if err := os.WriteFile(filepath.Join(root, ".specd/specs/demo/tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"step", "demo"}, map[string]string{"authority": ""}); err != nil {
		t.Fatal(err)
	}
	s := loadBrainSession(t, root)
	if len(s.PendingMissions) != 1 || s.PendingMissions[0].Worker != "w1" {
		t.Fatalf("mission worker not populated from task row: %+v", s.PendingMissions)
	}
}
