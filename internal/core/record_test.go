package core

import (
	"testing"
	"time"
)

// TestStampRecord asserts the provenance triple is filled from the injectable
// Clock (determinism), the passed git HEAD, and $SPECD_ACTOR when set (R3.1).
func TestStampRecord(t *testing.T) {
	fixed := time.Date(2026, 7, 4, 12, 0, 0, 0, time.UTC)
	orig := Clock
	Clock = func() time.Time { return fixed }
	t.Cleanup(func() { Clock = orig })
	t.Setenv("SPECD_ACTOR", "alice")

	rec := StampRecord(Record{Kind: "decision", Text: "use CAS"}, "deadbeef")

	if rec.Timestamp != "2026-07-04T12:00:00Z" {
		t.Fatalf("timestamp = %q, want RFC3339 from Clock", rec.Timestamp)
	}
	if rec.GitHead != "deadbeef" {
		t.Fatalf("git_head = %q", rec.GitHead)
	}
	if rec.Actor != "alice" {
		t.Fatalf("actor = %q, want $SPECD_ACTOR", rec.Actor)
	}
	if rec.Kind != "decision" || rec.Text != "use CAS" {
		t.Fatalf("content mutated: %+v", rec)
	}
}
