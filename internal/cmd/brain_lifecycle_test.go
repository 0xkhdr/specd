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

const brainEnabledConfig = "orchestration:\n  enabled: true\n"

// TestBrainCancel drives a started session to the terminal cancelled state and
// asserts the invariants: the session is cancelled with its lease released, a
// second cancel is idempotent, step/run are refused, and neither task nor
// evidence state is touched (spec 07 R2).
func TestBrainCancel(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	statePath := filepath.Join(root, ".specd", "specs", "demo", "state.json")
	before, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"start", "demo"}, map[string]string{}); err != nil {
		t.Fatalf("start: %v", err)
	}

	t.Run("cancel_marks_session_terminal_and_releases_lease", func(t *testing.T) {
		if err := runBrain(root, []string{"cancel", "demo"}, map[string]string{}); err != nil {
			t.Fatalf("cancel: %v", err)
		}
		session := loadBrainSession(t, root)
		if session.Status() != orchestration.SessionCancelled {
			t.Fatalf("expected cancelled, got %q", session.Status())
		}
		for _, lease := range session.Leases {
			if lease.State != orchestration.LeaseRevoked {
				t.Fatalf("expected retained revoked leases, got %v", session.Leases)
			}
		}
	})

	t.Run("second_cancel_is_idempotent", func(t *testing.T) {
		if err := runBrain(root, []string{"cancel", "demo"}, map[string]string{}); err != nil {
			t.Fatalf("second cancel: %v", err)
		}
	})

	t.Run("step_refused_after_cancel", func(t *testing.T) {
		err := runBrain(root, []string{"step", "demo"}, map[string]string{})
		if err == nil || !strings.Contains(err.Error(), "session is cancelled") {
			t.Fatalf("expected refusal, got %v", err)
		}
	})

	t.Run("task_and_evidence_state_untouched", func(t *testing.T) {
		after, err := os.ReadFile(statePath)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatalf("state.json was mutated by the brain lifecycle:\nbefore=%s\nafter=%s", before, after)
		}
	})
}

// TestBrainResumeCommand exercises the resume command over an on-disk checkpoint
// that outran the ledger: resume re-issues exactly that mission, and a second
// resume does not double-dispatch it (spec 07 R3).
func TestBrainResumeCommand(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	if err := runBrain(root, []string{"start", "demo"}, map[string]string{}); err != nil {
		t.Fatalf("start: %v", err)
	}
	checkpointPath := orchestration.CheckpointPath(root, "demo")
	acpPath := filepath.Join(root, ".specd", "specs", "demo", "acp.jsonl")
	// A checkpoint whose mission never reached the ledger: the crash between
	// write-ahead checkpoint and dispatch.
	writeCheckpoint(t, root, checkpointPath, orchestration.Checkpoint{
		SessionID: "demo", Step: 1, MissionID: "demo.s1.T1", TaskID: "T1", Time: time.Now(),
	})

	if err := runBrain(root, []string{"resume", "demo"}, map[string]string{}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := countDispatch(t, acpPath, "demo.s1.T1"); got != 1 {
		t.Fatalf("expected one dispatch after resume, got %d", got)
	}

	// A racing / repeated resume must not append the mission a second time; the
	// duplicate guard makes re-issue idempotent.
	if err := runBrain(root, []string{"resume", "demo"}, map[string]string{}); err != nil {
		t.Fatalf("second resume: %v", err)
	}
	if got := countDispatch(t, acpPath, "demo.s1.T1"); got != 1 {
		t.Fatalf("expected still one dispatch after second resume, got %d", got)
	}
}

// TestBrainResumeRefusesLiveLease refuses resume when the session is running with
// a live lease — a controller mid-flight, not a crash to recover (spec 07 R5).
func TestBrainResumeRefusesLiveLease(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	sessionPath := brainSessionPath(root)
	if err := orchestration.SaveSessionCAS(root, sessionPath, 0, orchestration.Session{
		ID:     "demo",
		Leases: []orchestration.Lease{{TaskID: "T1", WorkerID: "brain", ExpiresAt: time.Now().Add(time.Hour)}},
	}); err != nil {
		t.Fatal(err)
	}
	err := runBrain(root, []string{"resume", "demo"}, map[string]string{})
	if err == nil || !strings.Contains(err.Error(), "live lease") {
		t.Fatalf("expected live-lease refusal, got %v", err)
	}
}

func loadBrainSession(t *testing.T, root string) orchestration.Session {
	t.Helper()
	session, err := orchestration.LoadSession(brainSessionPath(root))
	if err != nil {
		t.Fatal(err)
	}
	return session
}

func writeCheckpoint(t *testing.T, root, path string, cp orchestration.Checkpoint) {
	t.Helper()
	if err := orchestration.SaveCheckpoint(root, path, cp); err != nil {
		t.Fatal(err)
	}
}

func countDispatch(t *testing.T, acpPath, missionID string) int {
	t.Helper()
	data, err := os.ReadFile(acpPath)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if line == "" {
			continue
		}
		var event orchestration.ACPEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			t.Fatal(err)
		}
		if event.MissionID == missionID {
			n++
		}
	}
	return n
}
