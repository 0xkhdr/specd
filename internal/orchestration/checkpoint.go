package orchestration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// Checkpoint is the write-ahead record the controller makes durable BEFORE a
// dispatch becomes visible in the ledger (spec 07 R1). If the process dies after
// the checkpoint but before the ACP dispatch, resume finds the checkpoint's
// mission id absent from the ledger and re-issues exactly that dispatch; if it
// dies after the ledger append, resume finds the mission id present and does not
// re-issue — this is what makes crash recovery converge with zero
// double-dispatch.
type Checkpoint struct {
	SessionID string     `json:"session_id"`
	Step      int        `json:"step"`
	Wave      int        `json:"wave"`
	Decision  string     `json:"decision"`
	MissionID string     `json:"mission_id,omitempty"`
	TaskID    string     `json:"task_id,omitempty"`
	Mission   *MissionV1 `json:"mission,omitempty"`
	Lease     *Lease     `json:"lease,omitempty"`
	Time      time.Time  `json:"time"`
}

// CheckpointPath is the per-spec write-ahead checkpoint file.
func CheckpointPath(root, slug string) string {
	return filepath.Join(core.SpecdDir(root), "specs", slug, "checkpoint.json")
}

// SaveCheckpoint writes the checkpoint durably (atomic write + fsync via
// core.AtomicWrite) under the spec lock. It must return before the caller makes
// the corresponding dispatch visible in the ledger (write-ahead ordering).
func SaveCheckpoint(root, path string, cp Checkpoint) error {
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		data, err := json.MarshalIndent(cp, "", "  ")
		if err != nil {
			return struct{}{}, fmt.Errorf("encode checkpoint: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return struct{}{}, fmt.Errorf("mkdir checkpoint: %w", err)
		}
		return struct{}{}, core.AtomicWrite(path, string(append(data, '\n')))
	})
	return err
}

// LoadCheckpoint reads the checkpoint. The bool is false when none exists yet.
func LoadCheckpoint(path string) (Checkpoint, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Checkpoint{}, false, nil
	}
	if err != nil {
		return Checkpoint{}, false, fmt.Errorf("read checkpoint: %w", err)
	}
	var cp Checkpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return Checkpoint{}, false, fmt.Errorf("decode checkpoint: %w", err)
	}
	return cp, true, nil
}

// MissionID is the deterministic identifier for a dispatch: session id + step +
// task. Determinism is the point — a dispatch re-issued after a crash on resume
// reuses the same mission id, so the ledger's duplicate guard makes re-issue
// idempotent (spec 07 R3).
func MissionID(sessionID string, step int, taskID string) string {
	return fmt.Sprintf("%s.s%d.%s", sessionID, step, taskID)
}
