package core

import (
	"path/filepath"
	"testing"
	"time"
)

// withClock pins the injectable Clock for deterministic timestamps and restores
// it afterwards.
func withClock(t *testing.T, at time.Time) {
	t.Helper()
	prev := Clock
	Clock = func() time.Time { return at }
	t.Cleanup(func() { Clock = prev })
}

func TestCriterionRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "criteria.jsonl")

	base := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)
	withClock(t, base)
	if err := AppendCriterion(path, CriterionRecord{Criterion: "1.2", Status: CriterionStatusPass, Evidence: "covered by T3", GitHead: "abc123"}); err != nil {
		t.Fatalf("append pass: %v", err)
	}

	// A later fail is retained; a later pass does not erase the fail (R4).
	withClock(t, base.Add(time.Minute))
	if err := AppendCriterion(path, CriterionRecord{Criterion: "1.2", Status: CriterionStatusFail, Evidence: "regressed", GitHead: "def456"}); err != nil {
		t.Fatalf("append fail: %v", err)
	}
	withClock(t, base.Add(2*time.Minute))
	if err := AppendCriterion(path, CriterionRecord{Criterion: "1.2", Status: CriterionStatusPass, Evidence: "fixed", GitHead: "ghi789"}); err != nil {
		t.Fatalf("append repass: %v", err)
	}

	records, err := LoadCriteria(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("want 3 append-only records, got %d", len(records))
	}
	if records[0].Type != "criterion" {
		t.Fatalf("record type = %q, want criterion", records[0].Type)
	}
	if records[1].Status != CriterionStatusFail {
		t.Fatalf("fail record not retained: %+v", records)
	}
	if records[0].GitHead != "abc123" {
		t.Fatalf("git head not pinned: %+v", records[0])
	}
	if records[0].Actor == "" || records[0].Timestamp == "" {
		t.Fatalf("record not stamped: %+v", records[0])
	}

	// CurrentPassing: latest record after `since` wins. `since` before all ⇒
	// the final pass counts.
	if p := CurrentPassing(records, base.Add(-time.Hour)); !p["1.2"] {
		t.Fatalf("latest pass should be current, got %v", p)
	}
	// A `since` after every record ⇒ nothing counts (stale attestations
	// invalidated by a fresh requirements approval, R5).
	if p := CurrentPassing(records, base.Add(time.Hour)); p["1.2"] {
		t.Fatalf("records before `since` must not count, got %v", p)
	}

	// Validation: empty evidence / bad status / missing id are rejected.
	for _, bad := range []CriterionRecord{
		{Criterion: "1.1", Status: CriterionStatusPass},
		{Criterion: "1.1", Status: "maybe", Evidence: "x"},
		{Status: CriterionStatusPass, Evidence: "x"},
	} {
		if err := AppendCriterion(path, bad); err == nil {
			t.Fatalf("expected rejection for %+v", bad)
		}
	}
}
