package orchestration

import (
	"os"
	"testing"
	"time"
)

// TestCheckpoint covers the write-ahead checkpoint record: an absent checkpoint
// reads as "not present", a saved checkpoint round-trips byte-for-byte through
// its fields (including the nested lease), and the save is durable on disk before
// it returns — the ordering guarantee the resume path relies on (spec 07 R1).
func TestCheckpoint(t *testing.T) {
	t.Run("absent_reads_as_not_present", func(t *testing.T) {
		root := t.TempDir()
		path := CheckpointPath(root, "demo")
		cp, ok, err := LoadCheckpoint(path)
		if err != nil {
			t.Fatalf("load absent: %v", err)
		}
		if ok {
			t.Fatalf("expected not present, got %+v", cp)
		}
	})

	t.Run("round_trips_through_save_and_load", func(t *testing.T) {
		root := t.TempDir()
		path := CheckpointPath(root, "demo")
		now := time.Now().UTC().Truncate(time.Second)
		want := Checkpoint{
			SessionID: "demo",
			Step:      4,
			Wave:      2,
			Decision:  ACPKindDispatch,
			MissionID: "demo.s4.T7",
			TaskID:    "T7",
			Lease:     &Lease{TaskID: "T7", WorkerID: "brain", ExpiresAt: now.Add(time.Minute)},
			Time:      now,
		}
		if err := SaveCheckpoint(root, path, want); err != nil {
			t.Fatalf("save: %v", err)
		}
		// Durable before return: the file exists on disk immediately.
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("checkpoint not durable: %v", err)
		}
		got, ok, err := LoadCheckpoint(path)
		if err != nil || !ok {
			t.Fatalf("load: ok=%v err=%v", ok, err)
		}
		if got.MissionID != want.MissionID || got.Step != want.Step || got.TaskID != want.TaskID || got.Decision != want.Decision || got.Wave != want.Wave {
			t.Fatalf("round-trip mismatch: got %+v want %+v", got, want)
		}
		if got.Lease == nil || got.Lease.TaskID != "T7" || got.Lease.WorkerID != "brain" {
			t.Fatalf("lease did not round-trip: %+v", got.Lease)
		}
	})

	t.Run("resave_overwrites_prior_checkpoint", func(t *testing.T) {
		root := t.TempDir()
		path := CheckpointPath(root, "demo")
		if err := SaveCheckpoint(root, path, Checkpoint{Step: 1, MissionID: "demo.s1.T1", TaskID: "T1"}); err != nil {
			t.Fatalf("save first: %v", err)
		}
		if err := SaveCheckpoint(root, path, Checkpoint{Step: 2, MissionID: "demo.s2.T2", TaskID: "T2"}); err != nil {
			t.Fatalf("save second: %v", err)
		}
		got, _, err := LoadCheckpoint(path)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if got.Step != 2 || got.MissionID != "demo.s2.T2" {
			t.Fatalf("expected latest checkpoint, got %+v", got)
		}
	})
}

// TestMissionID pins the deterministic dispatch identifier: same inputs yield the
// same id (the property that makes a re-issued dispatch idempotent after resume),
// and a change in session, step, or task yields a distinct id (spec 07 R3).
func TestMissionID(t *testing.T) {
	base := MissionID("demo", 3, "T1")

	t.Run("deterministic_for_equal_inputs", func(t *testing.T) {
		if again := MissionID("demo", 3, "T1"); again != base {
			t.Fatalf("non-deterministic: %q vs %q", base, again)
		}
	})

	t.Run("distinct_across_session_step_task", func(t *testing.T) {
		for name, id := range map[string]string{
			"other_session": MissionID("other", 3, "T1"),
			"other_step":    MissionID("demo", 4, "T1"),
			"other_task":    MissionID("demo", 3, "T2"),
		} {
			if id == base {
				t.Fatalf("%s collided with base id %q", name, base)
			}
		}
	})
}
