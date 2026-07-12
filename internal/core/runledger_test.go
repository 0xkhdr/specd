package core

import (
	"os"
	"sync"
	"testing"
)

// TestRunIDDeterministicNotTraceID pins run_id as a pure function of
// spec/task/baseline: stable for equal inputs, distinct for different tasks or
// baselines, and derived only from those inputs — never a provider trace ID
// (spec 07 R2.1).
func TestRunIDDeterministicNotTraceID(t *testing.T) {
	a := RunID("demo", "T1", "base0")
	if a != RunID("demo", "T1", "base0") {
		t.Fatal("run_id not deterministic for equal inputs")
	}
	if a == RunID("demo", "T2", "base0") {
		t.Fatal("distinct tasks collided")
	}
	if a == RunID("demo", "T1", "base1") {
		t.Fatal("distinct baselines collided")
	}
	if a == "" {
		t.Fatal("empty run_id")
	}
}

// TestRunLedgerChainMonotonicAttempts asserts two failures and a pass on one task
// remain three attempts on one run chain: one stable run_id, attempts 1, 2, 3
// (spec 07 R2.2). The ledger is append-only — every allocation is retained.
func TestRunLedgerChainMonotonicAttempts(t *testing.T) {
	root := t.TempDir()
	var runID string
	for i := 1; i <= 3; i++ {
		run, err := AllocateRun(root, "demo", "T1", "base0", "actor", "worker-1", TelemetrySourceWorker)
		if err != nil {
			t.Fatal(err)
		}
		if run.Attempt != i {
			t.Fatalf("attempt %d, want %d", run.Attempt, i)
		}
		if i == 1 {
			runID = run.RunID
		} else if run.RunID != runID {
			t.Fatalf("run_id drifted: %s != %s", run.RunID, runID)
		}
	}
	runs, err := ReadRuns(RunLedgerPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != 3 {
		t.Fatalf("ledger holds %d records, want 3 (append-only)", len(runs))
	}
	// A second task starts its own chain with a distinct run_id at attempt 1.
	other, err := AllocateRun(root, "demo", "T2", "base0", "actor", "", TelemetrySourceWorker)
	if err != nil {
		t.Fatal(err)
	}
	if other.Attempt != 1 || other.RunID == runID {
		t.Fatalf("second task did not start a fresh chain: %+v", other)
	}
}

// TestRunLedgerTornTrailingLine asserts a crash mid-append (a partial final line
// with no newline) yields the prior complete records rather than a decode
// failure (spec 07 R2.4).
func TestRunLedgerTornTrailingLine(t *testing.T) {
	root := t.TempDir()
	if _, err := AllocateRun(root, "demo", "T1", "base0", "a", "", TelemetrySourceWorker); err != nil {
		t.Fatal(err)
	}
	path := RunLedgerPath(root, "demo")
	// Simulate a torn append: a partial JSON line with no trailing newline.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"run_id":"deadbeef","task_id":"T1","attempt":`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	runs, err := ReadRuns(path)
	if err != nil {
		t.Fatalf("torn trailing line not tolerated: %v", err)
	}
	if len(runs) != 1 || runs[0].Attempt != 1 {
		t.Fatalf("prior record lost after torn append: %+v", runs)
	}
}

// TestRunLedgerUnknownEnvelopeFailsClosed asserts an unknown required schema
// version on a complete line fails closed (spec 07 R2.1 mirroring R1.2).
func TestRunLedgerUnknownEnvelopeFailsClosed(t *testing.T) {
	root := t.TempDir()
	path := RunLedgerPath(root, "demo")
	if err := AppendRun(path, RunV1{EnvelopeVersion: "v9", RunID: "x", TaskID: "T1", Attempt: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRuns(path); err == nil {
		t.Fatal("unknown run envelope version accepted")
	}
}

// TestRunLedgerConcurrentAppendNoDuplicateAttempt asserts racing writers on one
// task chain do not duplicate an attempt: N concurrent allocations yield the
// attempts 1..N exactly once each under one run_id (spec 07 R2.4).
func TestRunLedgerConcurrentAppendNoDuplicateAttempt(t *testing.T) {
	root := t.TempDir()
	const n = 12
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := AllocateRun(root, "demo", "T1", "base0", "a", "", TelemetrySourceWorker); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}
	runs, err := ReadRuns(RunLedgerPath(root, "demo"))
	if err != nil {
		t.Fatal(err)
	}
	if len(runs) != n {
		t.Fatalf("got %d records, want %d", len(runs), n)
	}
	seen := map[int]bool{}
	runID := runs[0].RunID
	for _, r := range runs {
		if r.RunID != runID {
			t.Fatalf("run_id drifted under race: %s != %s", r.RunID, runID)
		}
		if seen[r.Attempt] {
			t.Fatalf("duplicate attempt %d under race", r.Attempt)
		}
		seen[r.Attempt] = true
	}
	for i := 1; i <= n; i++ {
		if !seen[i] {
			t.Fatalf("attempt %d missing; chain not monotonic under race", i)
		}
	}
}
