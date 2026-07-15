package core

import (
	"strings"
	"testing"
	"time"
)

func TestParseReviewReportTableDriven(t *testing.T) {
	full := ScaffoldReviewReport("auth")
	approve := strings.Replace(full, "Verdict: revise", "Verdict: approve", 1)
	cases := []struct {
		name    string
		body    string
		wantErr bool
		verdict ReviewVerdict
	}{
		{"scaffold parses (revise)", full, false, ReviewRevise},
		{"approve verdict", approve, false, ReviewApprove},
		{"missing section", "## Summary\n## Verdict\nVerdict: approve\n", true, ""},
		{"no verdict", "## Summary\n## Bugs\n## Security\n## Hallucinated Dependencies\n## Style\n## Verdict\nTBD\n", true, ""},
		{"invalid verdict value", strings.Replace(full, "Verdict: revise", "Verdict: maybe", 1), true, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r, err := ParseReviewReport(tc.body)
			if tc.wantErr != (err != nil) {
				t.Fatalf("err = %v, wantErr %v", err, tc.wantErr)
			}
			if !tc.wantErr && r.Verdict != tc.verdict {
				t.Fatalf("verdict = %q, want %q", r.Verdict, tc.verdict)
			}
		})
	}
}

func TestEvaluateReviewGate(t *testing.T) {
	approve := strings.Replace(ScaffoldReviewReport("auth"), "Verdict: revise", "Verdict: approve", 1)
	fin := "2026-07-02T12:00:00Z"
	state := &State{Tasks: map[string]TaskState{"T1": {FinishedAt: &fin}}}
	completion, _ := time.Parse(time.RFC3339Nano, fin)

	t.Run("absent_report_blocks", func(t *testing.T) {
		if r := EvaluateReviewGate(state, nil, time.Time{}); r.OK {
			t.Fatal("expected block on absent report")
		}
	})
	t.Run("stale_report_blocks", func(t *testing.T) {
		stale := completion.Add(-time.Hour)
		r := EvaluateReviewGate(state, &approve, stale)
		if r.OK || !strings.Contains(r.Problem, "stale") {
			t.Fatalf("expected stale block, got %+v", r)
		}
	})
	t.Run("revise_verdict_blocks", func(t *testing.T) {
		revise := ScaffoldReviewReport("auth")
		r := EvaluateReviewGate(state, &revise, completion.Add(time.Hour))
		if r.OK || !strings.Contains(r.Problem, "verdict") {
			t.Fatalf("expected verdict block, got %+v", r)
		}
	})
	t.Run("fresh_approve_passes", func(t *testing.T) {
		r := EvaluateReviewGate(state, &approve, completion.Add(time.Hour))
		if !r.OK || !r.Fresh || r.Verdict != ReviewApprove {
			t.Fatalf("expected pass, got %+v", r)
		}
	})
}

func TestReviewChecklistExtraction(t *testing.T) {
	design := "# Design\n## Data model\ntext\n## Error handling\ntext\n"
	doc, err := ParseTasks("# Tasks — Auth\n\n## Wave 1\n\n- [ ] T1 — Impl\n  - why: build it\n  - role: engineer\n  - files: internal/x.go\n  - contract: does x\n  - acceptance: 1.1\n  - verify: go test\n  - depends: none\n")
	if err != nil {
		t.Fatal(err)
	}
	items := ReviewChecklist(design, &doc)
	joined := strings.Join(items, "\n")
	for _, want := range []string{"Data model", "Error handling", "task T1", "internal/x.go", "go test"} {
		if !strings.Contains(joined, want) {
			t.Errorf("checklist missing %q:\n%s", want, joined)
		}
	}
}
