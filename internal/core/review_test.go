package core

import (
	"strings"
	"testing"
)

func TestReviewScaffold(t *testing.T) {
	tasks := []TaskRow{
		{ID: "T1", Files: "a.go", Acceptance: "does the thing"},
		{ID: "T2", Files: "b.go", Acceptance: "does the other thing"},
	}
	out := RenderReviewScaffold("payments", "abc123def456", tasks)

	for _, want := range []string{
		"Review Report — payments",
		"**Git HEAD:** abc123def456",
		"### T1", "a.go", "does the thing",
		"### T2", "b.go", "does the other thing",
		"**Verdict:**",
		"## Findings",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("scaffold missing %q:\n%s", want, out)
		}
	}

	// The scaffold as-written is not an approval: its placeholder verdict fails
	// the strict parse (never a silent approve).
	if _, err := ParseReviewReport(out); err == nil {
		t.Fatal("unedited scaffold must not parse as a valid verdict")
	}
}

func TestReviewParse(t *testing.T) {
	approve := "# Review Report — demo\n\n- **Git HEAD:** deadbeef\n- **Reviewer:** alice\n- **Verdict:** approve\n\n## Findings\n\nChecked evidence and diff.\n"
	report, err := ParseReviewReport(approve)
	if err != nil {
		t.Fatalf("approve parse: %v", err)
	}
	if report.Verdict != ReviewApprove || report.Head != "deadbeef" {
		t.Fatalf("approve fields wrong: %+v", report)
	}
	if !strings.Contains(report.Findings, "Checked evidence") {
		t.Fatalf("findings not extracted: %q", report.Findings)
	}

	reject := "- **Git HEAD:** cafe\n- **Verdict:** reject\n\n## Findings\n\nMissing tests for T2.\n"
	report, err = ParseReviewReport(reject)
	if err != nil {
		t.Fatalf("reject parse: %v", err)
	}
	if report.Verdict != ReviewReject || !strings.Contains(report.Findings, "Missing tests") {
		t.Fatalf("reject fields wrong: %+v", report)
	}

	// Fail-closed cases (R5): each must error, never approve.
	for name, body := range map[string]string{
		"missing_verdict": "- **Git HEAD:** abc\n\n## Findings\n\nx\n",
		"unknown_verdict": "- **Git HEAD:** abc\n- **Verdict:** lgtm\n",
		"missing_head":    "- **Verdict:** approve\n",
		"empty":           "",
	} {
		if _, err := ParseReviewReport(body); err == nil {
			t.Fatalf("%s: expected parse error, got none", name)
		}
	}
}
