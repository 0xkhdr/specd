package core

import "testing"

func TestBoundaryEvidenceFindingsProductionRequiresBothEvidenceKinds(t *testing.T) {
	design := ParseDesign([]byte("- boundaries: external payment adapter\n"))
	tasks := []TaskRow{{Evidence: "unit", Checks: "error path"}}
	findings := BoundaryEvidenceFindings(design, tasks, true)
	if len(findings) != 1 || findings[0].Message != "external boundary lacks integration evidence planning" {
		t.Fatalf("findings = %+v", findings)
	}
	if got := BoundaryEvidenceFindings(design, []TaskRow{{Evidence: "integration", Checks: "error path"}}, true); len(got) != 0 {
		t.Fatalf("complete boundary plan rejected: %+v", got)
	}
	// The canonical class/check-id spelling (spec 05 R1.1) must satisfy the same
	// planning intent as the bare legacy token, whichever delimiter is used.
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
