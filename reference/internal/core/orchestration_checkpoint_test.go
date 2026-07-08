package core

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"
)

func validCheckpointRecord() CheckpointRecord {
	return CheckpointRecord{
		Version:         OrchestrationModelVersion,
		SessionID:       strings.Repeat("7", 32),
		Spec:            "demo",
		TaskID:          "T1",
		Attempt:         1,
		WorkerID:        "pinky-a",
		ProgressPercent: 70,
		ContextManifest: "manifest",
		WorkingNotes:    "wrote the parser, tests pending",
		ChangedFiles:    []string{"internal/core/demo.go"},
		GitHead:         strings.Repeat("a", 40),
		Reason:          "host /clear",
		CreatedAt:       "2026-06-18T12:00:00Z",
	}
}

func TestValidateCheckpointRecord(t *testing.T) {
	if err := ValidateCheckpointRecord(validCheckpointRecord()); err != nil {
		t.Fatalf("valid record rejected: %v", err)
	}
	cases := map[string]func(*CheckpointRecord){
		"bad version":    func(r *CheckpointRecord) { r.Version = 0 },
		"bad session":    func(r *CheckpointRecord) { r.SessionID = "short" },
		"bad spec":       func(r *CheckpointRecord) { r.Spec = "Bad Slug" },
		"bad task":       func(r *CheckpointRecord) { r.TaskID = "t1" },
		"attempt zero":   func(r *CheckpointRecord) { r.Attempt = 0 },
		"progress over":  func(r *CheckpointRecord) { r.ProgressPercent = 101 },
		"progress under": func(r *CheckpointRecord) { r.ProgressPercent = -1 },
		"bad worker":     func(r *CheckpointRecord) { r.WorkerID = "BAD" },
		"empty changed":  func(r *CheckpointRecord) { r.ChangedFiles = []string{""} },
		"bad created at": func(r *CheckpointRecord) { r.CreatedAt = "not-a-time" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			rec := validCheckpointRecord()
			mutate(&rec)
			if err := ValidateCheckpointRecord(rec); err == nil {
				t.Fatalf("%s: expected validation error, got nil", name)
			}
		})
	}
}

func TestCheckpointRecordCanonicalRoundTripStable(t *testing.T) {
	rec := validCheckpointRecord()
	first, err := CanonicalOrchestrationJSON(rec)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalOrchestrationJSON(rec)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("canonical JSON not byte-stable:\n%s\n%s", first, second)
	}
}

func TestRecordCheckpointReleasesLeaseAndEmitsEvent(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("7", 32)
	cfg := DefaultConfig.Orchestration
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	mission, err := BuildPinkyMission(root, "demo", sessionID, "pinky-a", "T1", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimPinkyMission(root, mission, cfg); err != nil {
		t.Fatal(err)
	}

	rec := validCheckpointRecord()
	rec.SessionID = sessionID
	rec.CreatedAt = ""
	if _, err := RecordCheckpoint(root, rec, cfg); err != nil {
		t.Fatalf("RecordCheckpoint: %v", err)
	}

	// Record file written at the deterministic path.
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	path, err := paths.CheckpointPath(sessionID, "T1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("checkpoint record not written: %v", err)
	}

	// A checkpoint event exists.
	store, err := NewACPStore(root)
	if err != nil {
		t.Fatal(err)
	}
	events, err := store.readAllEvents(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range events {
		if e.Type == ACPMessageCheckpoint {
			found = true
		}
	}
	if !found {
		t.Fatalf("no checkpoint event recorded; events=%d", len(events))
	}

	// Lease is released: the worker no longer holds the task.
	if err := store.ValidateActiveLease(sessionID, "pinky-a", "demo", "T1", 1); err == nil {
		t.Fatal("expected lease to be released after checkpoint")
	}
}

func TestRecordCheckpointWithoutLeaseFails(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("7", 32)
	cfg := DefaultConfig.Orchestration
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	rec := validCheckpointRecord()
	rec.SessionID = sessionID
	if _, err := RecordCheckpoint(root, rec, cfg); err == nil {
		t.Fatal("expected error when caller holds no active lease")
	}

	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	path, err := paths.CheckpointPath(sessionID, "T1", 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("no-lease checkpoint must not write a record, stat err=%v", err)
	}
}
