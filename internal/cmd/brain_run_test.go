package cmd

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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
		"| ✅ T1 | builder | a.txt | - | printf ok | done |\n"
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

func newBrainTestRoot(t *testing.T, mode, projectConfig string) string {
	t.Helper()
	root := t.TempDir()
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
