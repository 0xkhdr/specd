package orchestration

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// TestRecoverTornACPLineConverges asserts a crash mid-append to the ACP ledger
// (a partial final line with no newline) does not break recovery: ReadACP drops
// the torn line and yields the prior complete events, so PlanResume still finds
// the last durable mission and re-issue stays idempotent (spec 07 R2.4).
func TestRecoverTornACPLineConverges(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "acp.jsonl")
	if err := AppendACP(path, ACPEvent{Kind: ACPKindDispatch, MissionID: "demo.s1.T1", TaskID: "T1", Time: time.Unix(1, 0)}); err != nil {
		t.Fatal(err)
	}
	// Simulate a torn append: a partial JSON line with no trailing newline.
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"kind":"dispatch","mission_id":"demo.s2.T2","task_`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	events, err := ReadACP(path)
	if err != nil {
		t.Fatalf("torn ACP line not tolerated: %v", err)
	}
	if len(events) != 1 || events[0].MissionID != "demo.s1.T1" {
		t.Fatalf("prior mission lost after torn append: %+v", events)
	}
	// The prior mission is present, so a checkpoint naming it is a no-op (no
	// duplicate re-issue); the torn mission never became durable.
	if plan := PlanResume(Checkpoint{MissionID: "demo.s1.T1", TaskID: "T1"}, true, events); plan.Reissue || plan.Conflict != "" {
		t.Fatalf("expected noop for durable mission, got %+v", plan)
	}
	if plan := PlanResume(Checkpoint{MissionID: "demo.s2.T2", TaskID: "T2"}, true, events); !plan.Reissue {
		t.Fatalf("torn (never-durable) mission should re-issue, got %+v", plan)
	}
}

// TestRecoverRunLedgerTornAppend asserts the shared run ledger survives a crash
// mid-append the same way: the torn final line is dropped, the prior attempts
// remain, and the next allocation continues the chain monotonically with no
// duplicate attempt (spec 07 R2.4).
func TestRecoverRunLedgerTornAppend(t *testing.T) {
	root := t.TempDir()
	if _, err := core.AllocateRun(root, "demo", "T1", "base0", "a", "", core.TelemetrySourceWorker); err != nil {
		t.Fatal(err)
	}
	path := core.RunLedgerPath(root, "demo")
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"run_id":"x","task_id":"T1","attempt":`); err != nil {
		t.Fatal(err)
	}
	f.Close()

	runs, err := core.ReadRuns(path)
	if err != nil {
		t.Fatalf("torn run line not tolerated: %v", err)
	}
	if len(runs) != 1 || runs[0].Attempt != 1 {
		t.Fatalf("prior attempt lost after torn append: %+v", runs)
	}
	next, err := core.AllocateRun(root, "demo", "T1", "base0", "a", "", core.TelemetrySourceWorker)
	if err != nil {
		t.Fatal(err)
	}
	if next.Attempt != 2 || next.RunID != runs[0].RunID {
		t.Fatalf("chain did not continue monotonically after crash: %+v", next)
	}
}
