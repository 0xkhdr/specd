package cmd_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/testharness"
)

// enableResilience rewrites the workspace config so the resilience checkpoint
// gate is on for a CLI test. It keeps orchestration enabled and leaves every
// other field to defaults via LoadConfig's partial merge.
func enableResilience(t *testing.T, root string) {
	t.Helper()
	raw := []byte(`{"orchestration":{"enabled":true,"resilience":{"checkpointEnabled":true}}}`)
	if err := os.WriteFile(core.ConfigPath(root), raw, 0o644); err != nil {
		t.Fatalf("write resilience config: %v", err)
	}
}

// claimActiveLease builds and claims a Pinky mission so the session has one
// active lease the checkpoint commands can act on.
func claimActiveLease(t *testing.T, root, slug, sessionID, worker, task string, cfg core.OrchestrationCfg) {
	t.Helper()
	mission, err := core.BuildPinkyMission(root, slug, sessionID, worker, task, 1, cfg)
	if err != nil {
		t.Fatalf("build mission: %v", err)
	}
	if _, err := core.ClaimPinkyMission(root, mission, cfg); err != nil {
		t.Fatalf("claim mission: %v", err)
	}
}

func checkpointEventCount(t *testing.T, root, sessionID string) int {
	t.Helper()
	dir := filepath.Join(root, ".specd", "runtime", "sessions", sessionID, "events")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read events dir: %v", err)
	}
	n := 0
	for _, entry := range entries {
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read event %s: %v", entry.Name(), err)
		}
		var env core.ACPEnvelope
		if err := json.Unmarshal(raw, &env); err != nil {
			continue
		}
		if env.Type == core.ACPMessageCheckpoint {
			n++
		}
	}
	return n
}

// TestPinkyCheckpointPersistsAndReleases (W1.4): `specd pinky checkpoint` writes
// a record, emits a checkpoint event, and clears the worker's lease.
func TestPinkyCheckpointPersistsAndReleases(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "pinky-cp")
	enableResilience(t, h.Root)
	sessionID := repeat("e")
	cfg := core.LoadConfig(h.Root).Orchestration
	claimActiveLease(t, h.Root, slug, sessionID, "pinky-a", "T1", cfg)

	res := h.RunExpect(core.ExitOK, "pinky", "checkpoint",
		"--session", sessionID, "--worker", "pinky-a", "--spec", slug,
		"--task", "T1", "--attempt", "1", "--percent", "55",
		"--notes", "scaffolding done", "--changed-files", "internal/core/demo.go",
		"--reason", "host /clear", "--json")
	var rec core.CheckpointRecord
	if err := json.Unmarshal([]byte(res.Stdout), &rec); err != nil {
		t.Fatalf("checkpoint JSON: %v\n%s", err, res.Stdout)
	}
	if rec.ProgressPercent != 55 || rec.WorkingNotes != "scaffolding done" {
		t.Fatalf("unexpected record: %#v", rec)
	}

	paths, err := core.NewACPRuntimePaths(h.Root)
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
	if got := checkpointEventCount(t, h.Root, sessionID); got != 1 {
		t.Fatalf("checkpoint events = %d, want 1", got)
	}
	store, _ := core.NewACPStore(h.Root)
	if err := store.ValidateActiveLease(sessionID, "pinky-a", slug, "T1", 1); err == nil {
		t.Fatal("expected lease cleared after checkpoint")
	}
}

// TestPinkyCheckpointDisabledNoop: with the gate off the command is a no-op that
// exits 0 and writes nothing.
func TestPinkyCheckpointDisabledNoop(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "pinky-cp-off")
	sessionID := repeat("f")
	res := h.RunExpect(core.ExitOK, "pinky", "checkpoint",
		"--session", sessionID, "--worker", "pinky-a", "--spec", slug,
		"--task", "T1", "--attempt", "1", "--percent", "55")
	if _, err := os.Stat(filepath.Join(h.Root, ".specd", "runtime", "sessions", sessionID, "checkpoints")); !os.IsNotExist(err) {
		t.Fatalf("disabled checkpoint must not write records, stat err=%v", err)
	}
	_ = res
}

// TestBrainCheckpointForcesAllActive (W1.5): `specd brain checkpoint` records a
// checkpoint per active worker, each carrying the supplied reason.
func TestBrainCheckpointForcesAllActive(t *testing.T) {
	h := testharness.New(t)
	slug := h.Spec("brain-cp").
		Req("cp", "As an operator I can checkpoint all workers.", "THE SYSTEM SHALL checkpoint every active worker.").
		FullDesign().
		Status(core.StatusExecuting).
		Orchestrated().
		AddTask(testharness.TaskSpec{ID: "T1"}).
		AddTask(testharness.TaskSpec{ID: "T2"}).
		Build()
	enableResilience(t, h.Root)
	sessionID := repeat("a")
	cfg := core.LoadConfig(h.Root).Orchestration
	claimActiveLease(t, h.Root, slug, sessionID, "pinky-a", "T1", cfg)
	claimActiveLease(t, h.Root, slug, sessionID, "pinky-b", "T2", cfg)

	res := h.RunExpect(core.ExitOK, "brain", "checkpoint", slug,
		"--session", sessionID, "--reason", "host-clear", "--json")
	var records []core.CheckpointRecord
	if err := json.Unmarshal([]byte(res.Stdout), &records); err != nil {
		t.Fatalf("brain checkpoint JSON: %v\n%s", err, res.Stdout)
	}
	if len(records) != 2 {
		t.Fatalf("checkpointed %d workers, want 2", len(records))
	}
	for _, r := range records {
		if r.Reason != "host-clear" {
			t.Fatalf("record %s missing reason: %#v", r.TaskID, r)
		}
	}
	if got := checkpointEventCount(t, h.Root, sessionID); got != 2 {
		t.Fatalf("checkpoint events = %d, want 2", got)
	}
}

// TestBrainCheckpointNoActiveWorkers: exits 0 with an empty array when nothing is
// leased.
func TestBrainCheckpointNoActiveWorkers(t *testing.T) {
	h := testharness.New(t)
	slug := recoverySpec(h, "brain-cp-empty")
	enableResilience(t, h.Root)
	sessionID := repeat("b")
	res := h.RunExpect(core.ExitOK, "brain", "checkpoint", slug,
		"--session", sessionID, "--json")
	var records []core.CheckpointRecord
	if err := json.Unmarshal([]byte(res.Stdout), &records); err != nil {
		t.Fatalf("brain checkpoint JSON: %v\n%s", err, res.Stdout)
	}
	if len(records) != 0 {
		t.Fatalf("want empty array, got %#v", records)
	}
}
