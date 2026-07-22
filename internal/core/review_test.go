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

	// The auditor sees the canonical declared-path projection (spec 05 R1.1), not
	// the raw cell: duplicates collapse, order is stable, delimiters agree.
	legacy := RenderReviewScaffold("payments", "abc123", []TaskRow{{ID: "T1", Files: "b.go;./a.go;b.go"}})
	if !strings.Contains(legacy, "- files: a.go, b.go\n") {
		t.Fatalf("review scaffold did not normalize declared files:\n%s", legacy)
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

func TestReviewRestampPreservesBody(t *testing.T) {
	// R5.2: restamp preserves human body bytes byte-for-byte, updates only machine
	// fields (Git HEAD). Verdict and note are parsed separately (R5.3).
	oldHead := "abc123def456"
	newHead := "fedcba654321"
	humanBody := `# Review Report — demo

- **Git HEAD:** {{HEAD}}
- **Reviewer:** alice
- **Verdict:** approve needs-update

## Tasks under review

### T1

- files: main.go
- acceptance: works

## Findings

Checked the logic carefully.
Updated test coverage.
Found edge case in error handling.
`

	// Fill in the old HEAD
	oldReport := strings.ReplaceAll(humanBody, "{{HEAD}}", oldHead)

	// Restamp to new HEAD
	newReport, err := RestampReviewReport(oldReport, newHead)
	if err != nil {
		t.Fatalf("restamp failed: %v", err)
	}

	// The new report should have the new HEAD
	if !strings.Contains(newReport, "- **Git HEAD:** "+newHead) {
		t.Fatalf("new HEAD not found in restamped report:\n%s", newReport)
	}

	// The new report should NOT have the old HEAD
	if strings.Contains(newReport, "- **Git HEAD:** "+oldHead) {
		t.Fatalf("old HEAD still in restamped report:\n%s", newReport)
	}

	// Extract the human body (everything except the HEAD line) from both reports
	extractBody := func(report string) string {
		lines := strings.Split(report, "\n")
		var result []string
		for _, line := range lines {
			if !strings.Contains(line, "- **Git HEAD:**") {
				result = append(result, line)
			}
		}
		return strings.Join(result, "\n")
	}

	oldBody := extractBody(oldReport)
	newBody := extractBody(newReport)

	// The human-written parts (everything except HEAD) should be identical
	if oldBody != newBody {
		t.Fatalf("body not preserved:\nold:\n%s\n\nnew:\n%s", oldBody, newBody)
	}

	// Parse the restamped report - verdict should have a note
	parsed, err := ParseReviewReport(newReport)
	if err != nil {
		t.Fatalf("parse restamped report failed: %v", err)
	}
	if parsed.Head != newHead {
		t.Fatalf("parsed HEAD = %q, want %q", parsed.Head, newHead)
	}
	// Verdict should be strict token "approve" separate from note "needs-update"
	if parsed.Verdict != ReviewApprove {
		t.Fatalf("verdict = %q, want %q", parsed.Verdict, ReviewApprove)
	}
	if parsed.Note != "needs-update" {
		t.Fatalf("note = %q, want %q", parsed.Note, "needs-update")
	}

	// R5.3: the token is normalized because it is matched; the note is human
	// prose and must survive verbatim, case included.
	t.Run("qualified_verdict_keeps_note_verbatim", func(t *testing.T) {
		report, err := ParseReviewReport("- **Git HEAD:** " + newHead + "\n- **Verdict:** APPROVE Minor NITs in Foo.go\n")
		if err != nil {
			t.Fatalf("qualified verdict refused: %v", err)
		}
		if report.Verdict != ReviewApprove {
			t.Errorf("verdict = %q, want %q", report.Verdict, ReviewApprove)
		}
		if report.Note != "Minor NITs in Foo.go" {
			t.Errorf("note = %q, want it preserved verbatim", report.Note)
		}
		// A note must never widen a non-approve token into an approve.
		rejected, err := ParseReviewReport("- **Git HEAD:** " + newHead + "\n- **Verdict:** reject approve later\n")
		if err != nil {
			t.Fatalf("qualified reject refused: %v", err)
		}
		if rejected.Verdict != ReviewReject {
			t.Errorf("verdict = %q, want %q", rejected.Verdict, ReviewReject)
		}
	})

	// R5.2/R7.1: restamping must not touch a single byte outside the HEAD line.
	t.Run("restamp_changes_only_the_head_line", func(t *testing.T) {
		want := strings.Replace(oldReport, oldHead, newHead, 1)
		got, err := RestampReviewReport(oldReport, newHead)
		if err != nil {
			t.Fatalf("restamp failed: %v", err)
		}
		if got != want {
			t.Fatalf("restamp altered bytes outside the HEAD line:\ngot:\n%q\nwant:\n%q", got, want)
		}
	})

	// unresolved-head / stale-head: an unresolvable new HEAD cannot be stamped,
	// and a report whose HEAD is still a placeholder cannot be restamped at all.
	t.Run("unresolved_head_refuses_restamp", func(t *testing.T) {
		if _, err := RestampReviewReport(oldReport, ""); err == nil {
			t.Error("restamped to an empty HEAD")
		}
		if _, err := RestampReviewReport(oldReport, "unknown"); err == nil {
			t.Error("restamped to an unresolved HEAD")
		}
		placeholder := strings.ReplaceAll(humanBody, "{{HEAD}}", "<git HEAD>")
		if _, err := RestampReviewReport(placeholder, newHead); err == nil {
			t.Error("restamped a report whose HEAD was never resolved")
		}
		if h := ReviewReportHead(placeholder); h != "" {
			t.Errorf("ReviewReportHead(placeholder) = %q, want empty", h)
		}
	})
}

func TestReviewContractHardRisksAndRequiredTest(t *testing.T) {
	contract := BuildReviewContract(QualityContract{TaskID: "T1", Required: []EvidenceRequirement{{EvidenceClass: EvidenceTest, CheckID: "unit"}, {EvidenceClass: EvidenceReview, CheckID: "audit"}}}, "head", nil)
	if contract.TaskID != "T1" || contract.SubjectRevision != "head" || len(contract.HardRisks) != 4 {
		t.Fatalf("contract = %+v", contract)
	}
	missingTest := EvaluateQuality(QualityContract{TaskID: "T1", Required: []EvidenceRequirement{{EvidenceClass: EvidenceTest, CheckID: "unit"}}}, nil, FreshnessSubject{Revision: "head"})
	if err := ValidateReviewContract(contract, missingTest); err == nil {
		t.Fatal("review contract bypassed missing required test")
	}
	passed := EvaluateQuality(QualityContract{TaskID: "T1", Required: []EvidenceRequirement{{EvidenceClass: EvidenceTest, CheckID: "unit"}}}, []EvidenceEnvelopeV1{{TaskID: "T1", EvidenceClass: EvidenceTest, CheckID: "unit", Verdict: EvalPass, SubjectRevision: "head"}}, FreshnessSubject{Revision: "head"})
	if err := ValidateReviewContract(contract, passed); err != nil {
		t.Fatalf("valid review contract rejected: %v", err)
	}
}
