package core

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RunEnvelopeV1 is the run-ledger schema version (spec 07 R2). Any other value
// is an unknown *required* schema and fails closed, mirroring the telemetry
// envelope (R1.2).
const RunEnvelopeV1 = "v1"

// RunV1 is one attempt in a task's run chain (spec 07 R2.1). run_id is a
// deterministic function of spec/task/baseline — never a provider trace ID — and
// is stable across the whole retry/recovery chain: two failures and a pass on one
// task share one run_id and carry attempts 1, 2, 3 (R2.2). The ledger
// (runs.jsonl) is append-only and additive: completion authority stays
// verify/eval, so the evidence gate passes or fails identically whether the
// ledger is present or absent (R2.3).
type RunV1 struct {
	EnvelopeVersion string `json:"envelope_version"`
	RunID           string `json:"run_id"`
	SpecID          string `json:"spec_id"`
	TaskID          string `json:"task_id"`
	Attempt         int    `json:"attempt"`
	StartedAt       string `json:"started_at"`
	GitHead         string `json:"git_head,omitempty"`
	Actor           string `json:"actor,omitempty"`
	WorkerID        string `json:"worker_id,omitempty"`
	TelemetrySource string `json:"telemetry_source,omitempty"`
}

// RunLedgerPath is the per-spec append-only run/attempt ledger.
func RunLedgerPath(root, slug string) string {
	return filepath.Join(SpecdDir(root), "specs", slug, "runs.jsonl")
}

// RunID derives the deterministic run-chain identity from the spec, task, and a
// baseline (the git HEAD at chain start). It is a content address — never a
// provider trace ID — so the same task/baseline always yields the same key
// (spec 07 R2.1). The 16-hex prefix is ample: collisions across one spec's tasks
// are astronomically unlikely and a chain is disambiguated by spec+task anyway.
func RunID(specID, taskID, baseline string) string {
	return Digest([]byte(specID + "\x00" + taskID + "\x00" + baseline))[:16]
}

// ReadRuns loads the run ledger. A torn *trailing* line — the signature of a
// crash mid-append (AppendRun writes the record and its newline in one fsynced
// write) — is dropped, so a crash yields the prior complete records rather than a
// decode failure (spec 07 R2.4). Corruption anywhere but the final line is a real
// error. A missing ledger is not an error: the ledger is additive (R2.3).
func ReadRuns(path string) ([]RunV1, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("open run ledger: %w", err)
	}
	lines := bytes.Split(data, []byte{'\n'})
	var runs []RunV1
	for i, line := range lines {
		if len(bytes.TrimSpace(line)) == 0 {
			continue
		}
		var run RunV1
		if err := json.Unmarshal(line, &run); err != nil {
			if i == len(lines)-1 {
				// Torn final line from a crash mid-append: drop it and keep the
				// prior complete records (R2.4).
				break
			}
			return nil, fmt.Errorf("decode run ledger line %d: %w", i+1, err)
		}
		if run.EnvelopeVersion != RunEnvelopeV1 {
			return nil, fmt.Errorf("unknown run envelope version %q", run.EnvelopeVersion)
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// AppendRun appends one record to the ledger with a single fsynced write, so a
// crash leaves either the prior complete record or one complete new record —
// never a partial line (spec 07 R2.4).
func AppendRun(path string, run RunV1) error {
	data, err := json.Marshal(run)
	if err != nil {
		return fmt.Errorf("encode run: %w", err)
	}
	return AppendFile(path, string(data)+"\n")
}

// nextRunIdentity derives the run_id and attempt for a new allocation on taskID.
// A task's chain is every run bearing that task id: the run_id is reused from the
// chain's existing records (so it stays stable through retry/recovery), and the
// attempt is one past the chain's highest (monotonic, R2.2). A fresh chain seeds
// run_id deterministically from spec/task/baseline (R2.1).
func nextRunIdentity(runs []RunV1, specID, taskID, baseline string) (string, int) {
	runID, attempt := "", 0
	for _, r := range runs {
		if r.TaskID != taskID {
			continue
		}
		if runID == "" {
			runID = r.RunID
		}
		if r.Attempt > attempt {
			attempt = r.Attempt
		}
	}
	if runID == "" {
		runID = RunID(specID, taskID, baseline)
	}
	return runID, attempt + 1
}

// AllocateRun allocates and durably appends the next run/attempt identity for a
// task under the spec lock (spec 07 R2.1). Manual verify and Brain runs call this
// one allocator, so a task's attempts accrue on a single run chain regardless of
// who drove them (R2.2). The read-derive-append happens under WithSpecLock, so
// racing writers cannot duplicate an attempt (R2.4). gitHead doubles as the
// chain baseline for a fresh chain. An empty actor resolves to the host actor.
func AllocateRun(root, slug, taskID, gitHead, actor, workerID, source string) (RunV1, error) {
	if taskID == "" {
		return RunV1{}, errors.New("run allocation requires a task id")
	}
	if actor == "" {
		actor = recordActor()
	}
	return WithSpecLock(root, func() (RunV1, error) {
		path := RunLedgerPath(root, slug)
		runs, err := ReadRuns(path)
		if err != nil {
			return RunV1{}, err
		}
		runID, attempt := nextRunIdentity(runs, slug, taskID, gitHead)
		run := RunV1{
			EnvelopeVersion: RunEnvelopeV1,
			RunID:           runID,
			SpecID:          slug,
			TaskID:          taskID,
			Attempt:         attempt,
			StartedAt:       Clock().Format(time.RFC3339),
			GitHead:         gitHead,
			Actor:           actor,
			WorkerID:        workerID,
			TelemetrySource: source,
		}
		if err := AppendRun(path, run); err != nil {
			return RunV1{}, err
		}
		return run, nil
	})
}
