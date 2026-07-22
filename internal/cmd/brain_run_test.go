package cmd

import (
	"encoding/json"
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
	if err != nil {
		t.Fatal(err)
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
