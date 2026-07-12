package core

import (
	"strings"
	"testing"
)

func TestAnalyzeCoverageFindsDesignAndCriterionGaps(t *testing.T) {
	requirements, err := ParseRequirements([]byte("### R1 — first\n\n- R1.1: When x, the system shall y.\n- R1.2: When z, the system shall q.\n\n### R2 — second\n\n- R2.1: When a, the system shall b.\n"))
	if err != nil {
		t.Fatal(err)
	}
	design := ParseDesign([]byte("- references: R1\n"))
	tasks := []TaskRow{{ID: "T1", Refs: []string{"R1", "R1.1"}, Kind: "implementation"}}
	findings := AnalyzeCoverage(requirements, design, tasks)
	joined := coverageMessages(findings)
	for _, want := range []string{"R1.2 lacks task coverage", "R2 lacks design coverage", "R2 lacks task coverage", "R2.1 lacks task coverage"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in findings: %+v", want, findings)
		}
	}
}

func TestAnalyzeCoverageAcceptsExplicitDeferredDisposition(t *testing.T) {
	requirements, err := ParseRequirements([]byte("### R1 — deferred\n\n- R1.1: When x, the system shall y.\n"))
	if err != nil {
		t.Fatal(err)
	}
	design := ParseDesign([]byte("- references: R1\n"))
	tasks := []TaskRow{{ID: "T1", Refs: []string{"R1"}, Kind: "deferred"}}
	if findings := AnalyzeCoverage(requirements, design, tasks); len(findings) != 0 {
		t.Fatalf("explicit deferred disposition rejected: %+v", findings)
	}
}

func coverageMessages(findings []CoverageFinding) string {
	parts := make([]string, 0, len(findings))
	for _, finding := range findings {
		parts = append(parts, finding.Message)
	}
	return strings.Join(parts, "\n")
}
