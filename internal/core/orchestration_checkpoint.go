package core

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// RecordCheckpoint persists a worker's mid-task progress checkpoint, then hands
// the task back to the Brain by releasing the worker's lease and appending a
// `checkpoint` ACP event. It mirrors RecordPinkyProgress's discipline: the
// caller must still own an active lease for (session, worker, spec, task,
// attempt) — a forged or expired worker is rejected before anything is written
// (Req 2.3). The on-disk CheckpointRecord is written canonically so a later
// SenseOrchestration can resume from it byte-stably.
//
// The sequence is write record -> append event -> clear lease. It is
// "atomic-ish": if the process dies between steps the worst case is a checkpoint
// file with a still-held lease, which the next lease expiry reconciles; no work
// is double-counted because the record is keyed on (task, attempt).
func RecordCheckpoint(root string, rec CheckpointRecord, cfg OrchestrationCfg) (CheckpointRecord, error) {
	if rec.Version == 0 {
		rec.Version = OrchestrationModelVersion
	}
	if rec.CreatedAt == "" {
		rec.CreatedAt = Clock().UTC().Format(time.RFC3339Nano)
	}
	if err := ValidateCheckpointRecord(rec); err != nil {
		return CheckpointRecord{}, err
	}

	store, err := NewACPStore(root)
	if err != nil {
		return CheckpointRecord{}, err
	}
	// Lease gate: only the worker that currently holds the task may checkpoint it.
	if err := store.ValidateActiveLease(rec.SessionID, rec.WorkerID, rec.Spec, rec.TaskID, rec.Attempt); err != nil {
		return CheckpointRecord{}, err
	}

	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return CheckpointRecord{}, err
	}
	path, err := paths.CheckpointPath(rec.SessionID, rec.TaskID, rec.Attempt)
	if err != nil {
		return CheckpointRecord{}, err
	}
	raw, err := CanonicalOrchestrationJSON(rec)
	if err != nil {
		return CheckpointRecord{}, err
	}
	if err := atomicWritePrivate(path, raw); err != nil {
		return CheckpointRecord{}, fmt.Errorf("record checkpoint: write record: %w", err)
	}

	// Append the checkpoint event (pinky -> brain), mirroring appendPinkyEvent.
	payload := ACPCheckpointPayload{
		Percent:      rec.ProgressPercent,
		Reason:       rec.Reason,
		ChangedFiles: append([]string{}, rec.ChangedFiles...),
		GitHead:      rec.GitHead,
	}
	envelope, err := NewACPEnvelope(ACPMessageCheckpoint, payload)
	if err != nil {
		return CheckpointRecord{}, err
	}
	messageID, err := NewACPID()
	if err != nil {
		return CheckpointRecord{}, err
	}
	now := Clock().UTC()
	envelope.MessageID = messageID
	envelope.SessionID = rec.SessionID
	envelope.CreatedAt = now.Format(time.RFC3339Nano)
	envelope.ExpiresAt = now.Add(time.Duration(cfg.Transport.MessageTTLSeconds) * time.Second).Format(time.RFC3339Nano)
	envelope.From = "pinky-" + rec.WorkerID
	envelope.To = "brain"
	envelope.Spec = rec.Spec
	envelope.Task = rec.TaskID
	envelope.Attempt = rec.Attempt
	if _, err := store.WriteEvent(envelope); err != nil {
		return CheckpointRecord{}, fmt.Errorf("record checkpoint: append event: %w", err)
	}

	// Hand the task back so the Brain can resume rather than wait on a held lease.
	// Clear (not just release) the lease: a checkpoint is a cooperative
	// continuation, so the same attempt must remain re-claimable by the resuming
	// worker rather than being burned like a crashed attempt.
	if err := store.ClearLease(rec.SessionID, rec.WorkerID, rec.Attempt); err != nil {
		return CheckpointRecord{}, fmt.Errorf("record checkpoint: clear lease: %w", err)
	}
	return rec, nil
}

// ForceCheckpointAll checkpoints every worker that currently holds an active
// lease in a session, so the host can shed all in-flight context (a /clear)
// without losing work (Req 3). Each recorded checkpoint carries the supplied
// reason, releases that worker's lease, and emits a checkpoint event — the same
// RecordCheckpoint path a worker uses voluntarily. Workers without progress
// detail are checkpointed at 0%, which still hands the task back for resume.
// Returns the records written; an empty slice (no error) when nothing is active.
func ForceCheckpointAll(root, sessionID, reason string, cfg OrchestrationCfg) ([]CheckpointRecord, error) {
	if err := validateACPOpaqueID("session ID", sessionID); err != nil {
		return nil, err
	}
	store, err := NewACPStore(root)
	if err != nil {
		return nil, err
	}
	leases, err := store.loadSessionLeases(sessionID)
	if err != nil {
		return nil, err
	}
	now := Clock().UTC()
	out := make([]CheckpointRecord, 0, len(leases))
	for _, lease := range leases {
		if !leaseIsActive(lease, now) {
			continue
		}
		rec := CheckpointRecord{
			SessionID:    sessionID,
			Spec:         lease.Spec,
			TaskID:       lease.Task,
			Attempt:      lease.Attempt,
			WorkerID:     lease.WorkerID,
			ChangedFiles: []string{},
			Reason:       reason,
		}
		saved, err := RecordCheckpoint(root, rec, cfg)
		if err != nil {
			return nil, fmt.Errorf("force checkpoint worker %s: %w", lease.WorkerID, err)
		}
		out = append(out, saved)
	}
	return out, nil
}

// loadSessionCheckpoints reads every persisted CheckpointRecord for a session,
// in deterministic (taskId, attempt) order. A missing checkpoints directory
// yields an empty slice. Unreadable or malformed records are skipped rather than
// failing the caller: a single corrupt checkpoint must never wedge a sense pass
// or block resume of the other tasks.
func loadSessionCheckpoints(root, sessionID string) ([]CheckpointRecord, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return nil, err
	}
	dir, err := paths.CheckpointDir(sessionID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []CheckpointRecord{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("load checkpoints: read dir: %w", err)
	}
	out := make([]CheckpointRecord, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		var rec CheckpointRecord
		if err := decodeACPStrict(raw, &rec); err != nil {
			continue
		}
		if err := ValidateCheckpointRecord(rec); err != nil {
			continue
		}
		if rec.SessionID != sessionID {
			continue
		}
		out = append(out, rec)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].TaskID != out[j].TaskID {
			return taskOrdinalLess(out[i].TaskID, out[j].TaskID)
		}
		return out[i].Attempt < out[j].Attempt
	})
	return out, nil
}

// loadCheckpointForAttempt loads the checkpoint record for one exact (task,
// attempt), returning ok=false when no record exists. A malformed record is
// reported as an error rather than silently dropped: a worker about to resume
// from it deserves to fail loudly rather than restart with a partial payload.
func loadCheckpointForAttempt(root, sessionID, taskID string, attempt int) (CheckpointRecord, bool, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return CheckpointRecord{}, false, err
	}
	path, err := paths.CheckpointPath(sessionID, taskID, attempt)
	if err != nil {
		return CheckpointRecord{}, false, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return CheckpointRecord{}, false, nil
	}
	if err != nil {
		return CheckpointRecord{}, false, fmt.Errorf("load checkpoint: read record: %w", err)
	}
	var rec CheckpointRecord
	if err := decodeACPStrict(raw, &rec); err != nil {
		return CheckpointRecord{}, false, fmt.Errorf("load checkpoint: decode record: %w", err)
	}
	if err := ValidateCheckpointRecord(rec); err != nil {
		return CheckpointRecord{}, false, err
	}
	return rec, true, nil
}

// CleanupCheckpoint removes every checkpoint record for a completed task so a
// verified-done task can never be resurrected by a stale checkpoint (Req 6). It
// removes all attempts of the task, not just one, and is best-effort: a missing
// directory or already-deleted record is not an error.
func CleanupCheckpoint(root, sessionID, taskID string) error {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return err
	}
	dir, err := paths.CheckpointDir(sessionID)
	if err != nil {
		return err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("cleanup checkpoint: read dir: %w", err)
	}
	prefix := taskID + "-"
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, ".json") {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("cleanup checkpoint: remove %s: %w", name, err)
		}
	}
	return nil
}
