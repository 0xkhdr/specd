package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestHasResumableCheckpointAttemptGuard proves checkpoint-protocol Req 6.2: a
// checkpoint whose Attempt is older than the task's current attempt is ignored,
// so a stale mid-progress record can never resurrect a retried task. Only an
// exact (taskID, attempt) match resumes.
func TestHasResumableCheckpointAttemptGuard(t *testing.T) {
	task := OrchestrationTaskSnapshot{ID: "T1", Attempt: 2}
	cps := []OrchestrationCheckpointSnapshot{
		{TaskID: "T1", Attempt: 1, ProgressPercent: 70}, // stale attempt
	}
	if hasResumableCheckpoint(task, cps) {
		t.Fatal("stale-attempt checkpoint must NOT be resumable (Req 6.2)")
	}

	// The matching attempt is resumable.
	cps = append(cps, OrchestrationCheckpointSnapshot{TaskID: "T1", Attempt: 2, ProgressPercent: 40})
	if !hasResumableCheckpoint(task, cps) {
		t.Fatal("current-attempt checkpoint must be resumable")
	}

	// A checkpoint for a different task never matches.
	other := OrchestrationTaskSnapshot{ID: "T2", Attempt: 2}
	if hasResumableCheckpoint(other, cps) {
		t.Fatal("checkpoint for a different task must not match")
	}
}

// TestCleanupCheckpointRemovesAllAttempts proves checkpoint-protocol Req 6.1: a
// completed task's checkpoints are deleted so it cannot be re-resumed. Cleanup
// removes every attempt of the task and is best-effort (a missing directory is
// not an error).
func TestCleanupCheckpointRemovesAllAttempts(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("7", 32)
	cfg := DefaultConfig.Orchestration

	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}

	// Best-effort: cleanup before any checkpoint dir exists is a no-op, not error.
	if err := CleanupCheckpoint(root, sessionID, "T1"); err != nil {
		t.Fatalf("cleanup on missing dir should be a no-op: %v", err)
	}

	// Persist two attempts of T1 and one of T2 directly via the canonical writer.
	for _, attempt := range []int{1, 2} {
		rec := validCheckpointRecord()
		rec.SessionID = sessionID
		rec.TaskID = "T1"
		rec.Attempt = attempt
		writeCheckpointFile(t, paths, rec, cfg)
	}
	recT2 := validCheckpointRecord()
	recT2.SessionID = sessionID
	recT2.TaskID = "T2"
	writeCheckpointFile(t, paths, recT2, cfg)

	// Cleanup T1: both T1 attempts gone, T2 untouched.
	if err := CleanupCheckpoint(root, sessionID, "T1"); err != nil {
		t.Fatalf("CleanupCheckpoint: %v", err)
	}
	for _, attempt := range []int{1, 2} {
		p, _ := paths.CheckpointPath(sessionID, "T1", attempt)
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("T1 attempt %d checkpoint should be deleted, stat err=%v", attempt, err)
		}
	}
	pT2, _ := paths.CheckpointPath(sessionID, "T2", 1)
	if _, err := os.Stat(pT2); err != nil {
		t.Fatalf("T2 checkpoint must survive cleanup of T1: %v", err)
	}

	// A second cleanup of the same task is idempotent.
	if err := CleanupCheckpoint(root, sessionID, "T1"); err != nil {
		t.Fatalf("second cleanup should be idempotent: %v", err)
	}
}

// writeCheckpointFile persists a record at its canonical path, creating the
// checkpoints directory as needed. It bypasses the lease/event side effects of
// RecordCheckpoint so a test can stage arbitrary on-disk checkpoints.
func writeCheckpointFile(t *testing.T, paths ACPRuntimePaths, rec CheckpointRecord, cfg OrchestrationCfg) {
	t.Helper()
	rec.Version = OrchestrationModelVersion
	if rec.CreatedAt == "" {
		rec.CreatedAt = "2026-06-18T12:00:00Z"
	}
	path, err := paths.CheckpointPath(rec.SessionID, rec.TaskID, rec.Attempt)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	data, err := CanonicalOrchestrationJSON(rec)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
