package core

import (
	"path/filepath"
	"testing"
)

func ev(task string, exit int, ts string) EvidenceRecord {
	return EvidenceRecord{TaskID: task, ExitCode: exit, GitHead: "abc", Timestamp: ts}
}

func TestEscalation(t *testing.T) {
	cases := []struct {
		name      string
		evidence  []EvidenceRecord
		overrides []OverrideRecord
		maxFails  int
		wantFails int
		wantEsc   bool
	}{
		{
			name:      "three_consecutive_fails_escalates",
			evidence:  []EvidenceRecord{ev("T1", 1, "2026-01-01T00:00:01Z"), ev("T1", 1, "2026-01-01T00:00:02Z"), ev("T1", 1, "2026-01-01T00:00:03Z")},
			maxFails:  3,
			wantFails: 3,
			wantEsc:   true,
		},
		{
			name:      "pass_resets_the_counter",
			evidence:  []EvidenceRecord{ev("T1", 1, "2026-01-01T00:00:01Z"), ev("T1", 1, "2026-01-01T00:00:02Z"), ev("T1", 0, "2026-01-01T00:00:03Z"), ev("T1", 1, "2026-01-01T00:00:04Z")},
			maxFails:  3,
			wantFails: 1,
			wantEsc:   false,
		},
		{
			name:      "override_resets_the_counter",
			evidence:  []EvidenceRecord{ev("T1", 1, "2026-01-01T00:00:01Z"), ev("T1", 1, "2026-01-01T00:00:02Z"), ev("T1", 1, "2026-01-01T00:00:03Z")},
			overrides: []OverrideRecord{{TaskID: "T1", Reason: "waived", Timestamp: "2026-01-01T00:00:04Z"}},
			maxFails:  3,
			wantFails: 0,
			wantEsc:   false,
		},
		{
			name:      "fails_after_override_re_escalate",
			evidence:  []EvidenceRecord{ev("T1", 1, "2026-01-01T00:00:01Z"), ev("T1", 1, "2026-01-01T00:00:02Z"), ev("T1", 1, "2026-01-01T00:00:05Z"), ev("T1", 1, "2026-01-01T00:00:06Z"), ev("T1", 1, "2026-01-01T00:00:07Z")},
			overrides: []OverrideRecord{{TaskID: "T1", Reason: "waived", Timestamp: "2026-01-01T00:00:03Z"}},
			maxFails:  3,
			wantFails: 3,
			wantEsc:   true,
		},
		{
			name:      "zero_maxfails_disables_ratchet",
			evidence:  []EvidenceRecord{ev("T1", 1, "2026-01-01T00:00:01Z"), ev("T1", 1, "2026-01-01T00:00:02Z"), ev("T1", 1, "2026-01-01T00:00:03Z")},
			maxFails:  0,
			wantFails: 3,
			wantEsc:   false,
		},
		{
			name:      "other_task_fails_do_not_count",
			evidence:  []EvidenceRecord{ev("T2", 1, "2026-01-01T00:00:01Z"), ev("T1", 1, "2026-01-01T00:00:02Z")},
			maxFails:  3,
			wantFails: 1,
			wantEsc:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ConsecutiveVerifyFails(tc.evidence, tc.overrides, "T1")
			if got != tc.wantFails {
				t.Fatalf("fails = %d, want %d", got, tc.wantFails)
			}
			if esc := IsEscalated(got, tc.maxFails); esc != tc.wantEsc {
				t.Fatalf("escalated = %v, want %v", esc, tc.wantEsc)
			}
		})
	}
}

func TestOverrideRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "overrides.jsonl")

	t.Run("empty_reason_rejected", func(t *testing.T) {
		if err := AppendOverride(path, OverrideRecord{TaskID: "T1"}); err == nil {
			t.Fatal("expected empty-reason override to be rejected")
		}
	})

	t.Run("append_and_load_round_trip", func(t *testing.T) {
		if err := AppendOverride(path, OverrideRecord{TaskID: "T1", Reason: "flaky infra", PriorFailCount: 3}); err != nil {
			t.Fatal(err)
		}
		records, err := LoadOverrides(path)
		if err != nil {
			t.Fatal(err)
		}
		if len(records) != 1 {
			t.Fatalf("records = %d, want 1", len(records))
		}
		got := records[0]
		if got.TaskID != "T1" || got.Reason != "flaky infra" || got.PriorFailCount != 3 {
			t.Fatalf("round-trip mismatch: %+v", got)
		}
		if got.Timestamp == "" || got.Actor == "" {
			t.Fatalf("provenance not stamped: %+v", got)
		}
	})
}
