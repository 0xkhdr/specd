package core

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	verifyexec "github.com/0xkhdr/specd/internal/core/verify"
)

func TestSubmitGateCheck(t *testing.T) {
	model := ReportModel{
		Slug:  "demo",
		Total: 2,
		Tasks: []ReportTask{
			{ID: "T1", Status: TaskComplete},
			{ID: "T2", Status: TaskPending},
		},
	}

	// Gate failures and incomplete tasks are both enumerated (R1).
	blockers := SubmitBlockers(model, []string{"gate evidence: T2 is complete without passing evidence"})
	if len(blockers) != 2 {
		t.Fatalf("want 2 blockers, got %d: %v", len(blockers), blockers)
	}
	if blockers[0] != "gate evidence: T2 is complete without passing evidence" {
		t.Fatalf("gate failure not first blocker: %q", blockers[0])
	}

	// All gates green and all tasks complete ⇒ no blockers.
	ready := ReportModel{Slug: "demo", Total: 1, Tasks: []ReportTask{{ID: "T1", Status: TaskComplete}}}
	if got := SubmitBlockers(ready, nil); len(got) != 0 {
		t.Fatalf("want submittable, got blockers: %v", got)
	}
}

func TestSubmitExec(t *testing.T) {
	// The summary is streamed on stdin through the one sandboxed exec path (R2):
	// `cat` echoes exactly what submit piped in.
	result, err := verifyexec.Run(context.Background(), verifyexec.Options{
		Command: "cat",
		Stdin:   "## specd report: demo\n",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("exit = %d, want 0", result.ExitCode)
	}
	if result.Stdout != "## specd report: demo\n" {
		t.Fatalf("stdin not streamed to command: stdout = %q", result.Stdout)
	}
}

func TestSubmitLedger(t *testing.T) {
	path := filepath.Join(t.TempDir(), "submissions.jsonl")
	withClock(t, time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC))

	if err := AppendSubmission(path, SubmissionRecord{GitHead: "abc123", SummaryHash: SummaryHash("body"), Command: "cat", Exit: 0}); err != nil {
		t.Fatalf("append: %v", err)
	}
	records, err := LoadSubmissions(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 1 {
		t.Fatalf("want 1 record, got %d", len(records))
	}
	rec := records[0]
	if rec.Type != submissionType {
		t.Fatalf("type = %q, want %q", rec.Type, submissionType)
	}
	if rec.Actor == "" || rec.Timestamp == "" {
		t.Fatalf("provenance not stamped: %+v", rec)
	}
	if rec.SummaryHash != SummaryHash("body") {
		t.Fatalf("summary hash not persisted")
	}

	// Idempotence guard (R5): a successful submission at the same HEAD is seen.
	if !AlreadySubmittedAt(records, "abc123") {
		t.Fatalf("same-HEAD submission not detected")
	}
	if AlreadySubmittedAt(records, "def456") {
		t.Fatalf("different HEAD wrongly treated as submitted")
	}

	// A failed submission does not count as submitted.
	if err := AppendSubmission(path, SubmissionRecord{GitHead: "def456", SummaryHash: SummaryHash("body"), Command: "false", Exit: 1}); err != nil {
		t.Fatalf("append fail: %v", err)
	}
	records, _ = LoadSubmissions(path)
	if AlreadySubmittedAt(records, "def456") {
		t.Fatalf("failed submission wrongly counts as submitted")
	}

	// An unresolved HEAD is never treated as already submitted.
	if AlreadySubmittedAt(records, UnknownHead) {
		t.Fatalf("unresolved HEAD must not match")
	}
}
