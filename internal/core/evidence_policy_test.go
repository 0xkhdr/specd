package core

import (
	"strings"
	"testing"
)

func TestBoundaryEvidenceFindingsProductionRequiresBothEvidenceKinds(t *testing.T) {
	design := ParseDesign([]byte("- boundaries: external payment adapter\n"))
	tasks := []TaskRow{{Evidence: "test/unit", Checks: "error path"}}
	findings := BoundaryEvidenceFindings(design, tasks, true)
	if len(findings) != 1 || !strings.Contains(findings[0].Message, "lacks integration evidence") {
		t.Fatalf("findings = %+v", findings)
	}
	// The canonical class/check-id spelling (spec 05 R1.1) satisfies the boundary,
	// whichever delimiter is used.
	for _, evidence := range []string{"test/integration-payments", "test/unit;test/integration-payments"} {
		if got := BoundaryEvidenceFindings(design, []TaskRow{{Evidence: evidence, Checks: "error-path"}}, true); len(got) != 0 {
			t.Fatalf("canonical evidence %q rejected: %+v", evidence, got)
		}
	}
}

func TestBoundaryEvidenceFindingsDefaultAndLocalPlansAreInert(t *testing.T) {
	design := ParseDesign([]byte("- boundaries: external payment adapter\n"))
	if got := BoundaryEvidenceFindings(design, nil, false); len(got) != 0 {
		t.Fatalf("default profile enforced production policy: %+v", got)
	}
	local := ParseDesign([]byte("- boundaries: local in-memory store\n"))
	if got := BoundaryEvidenceFindings(local, nil, true); len(got) != 0 {
		t.Fatalf("local boundary enforced external policy: %+v", got)
	}
}

// TestSharedEvidenceParserClosesDoubleBind pins spec R7.1: the boundary gate and
// the quality-declaration gate read one parser, so a cell either satisfies both
// or is rejected by both — never required by one and rejected by the other. A
// bare `integration` token (which ParseQualityContract refuses) therefore no
// longer secretly satisfies the boundary.
func TestSharedEvidenceParserClosesDoubleBind(t *testing.T) {
	design := ParseDesign([]byte("- boundaries: external payment adapter\n"))

	// A canonical cell satisfies BOTH gates.
	both := TaskRow{Evidence: "test/integration-payments", Checks: "error-path"}
	if _, err := ParseQualityContract(both); err != nil {
		t.Fatalf("quality-declaration rejected a cell the boundary requires: %v", err)
	}
	if got := BoundaryEvidenceFindings(design, []TaskRow{both}, true); len(got) != 0 {
		t.Fatalf("boundary rejected a cell quality-declaration accepts: %+v", got)
	}

	// A bare `integration` token is rejected by quality-declaration, so it must
	// NOT earn integration credit at the boundary (double-bind closed).
	bare := TaskRow{Evidence: "integration", Checks: "error-path"}
	if _, err := ParseQualityContract(bare); err == nil {
		t.Fatal("bare `integration` should be rejected by ParseQualityContract")
	}
	got := BoundaryEvidenceFindings(design, []TaskRow{bare}, true)
	if len(got) != 1 || !strings.Contains(got[0].Message, "lacks integration evidence") {
		t.Fatalf("bare token must not satisfy the boundary: %+v", got)
	}
}

// TestSharedEvidenceParserAcceptedForms pins spec R7.2: an integration-equivalent
// evidence class and an integration check id each satisfy an external boundary.
func TestSharedEvidenceParserAcceptedForms(t *testing.T) {
	design := ParseDesign([]byte("- boundaries: external payment adapter\n"))
	for _, evidence := range []string{"trajectory_eval/e2e-payments", "test/integration-payments"} {
		task := TaskRow{Evidence: evidence, Checks: "error-path"}
		if got := BoundaryEvidenceFindings(design, []TaskRow{task}, true); len(got) != 0 {
			t.Fatalf("accepted form %q rejected: %+v", evidence, got)
		}
	}
	// A non-integration class + non-integration check id does not satisfy it.
	task := TaskRow{Evidence: "test/unit", Checks: "error-path"}
	if got := BoundaryEvidenceFindings(design, []TaskRow{task}, true); len(got) != 1 {
		t.Fatalf("non-integration evidence unexpectedly satisfied boundary: %+v", got)
	}
}

// TestSharedEvidenceParserRefusalNamesBoundaryAndRemedy pins spec R7.3: a
// boundary refusal names the boundary it inspected and the artifact that would
// satisfy it.
func TestSharedEvidenceParserRefusalNamesBoundaryAndRemedy(t *testing.T) {
	design := ParseDesign([]byte("- boundaries: external payment adapter\n"))
	got := BoundaryEvidenceFindings(design, []TaskRow{{Evidence: "test/unit", Checks: "happy only"}}, true)
	if len(got) != 2 {
		t.Fatalf("expected integration + error-path findings: %+v", got)
	}
	for _, want := range []string{"boundaries:external", "trajectory_eval", "integration"} {
		if !strings.Contains(got[0].Message, want) {
			t.Fatalf("integration refusal %q missing %q", got[0].Message, want)
		}
	}
	if !strings.Contains(got[1].Message, "boundaries:external") {
		t.Fatalf("error-path refusal must name the boundary: %q", got[1].Message)
	}
}
