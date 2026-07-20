package core

import "testing"

// TestScaffoldRequirementsParse pins the requirements-template → ParseRequirements
// contract (R2.1, R5.1): the shipped scaffold must yield a non-empty requirement
// ID set through the real consumer, and its criteria must be EARS-shaped so the
// filled-in scaffold passes its own gate.
func TestScaffoldRequirementsParse(t *testing.T) {
	stub := RequirementsScaffold("demo")

	ids := RequirementIDSet(stub)
	if len(ids) == 0 {
		t.Fatalf("scaffolded requirements.md yields an empty ID set:\n%s", stub)
	}
	if !ids["R1"] || !ids["R1.1"] {
		t.Fatalf("expected R1 and R1.1 in ID set, got %v", ids)
	}

	doc, err := ParseRequirements([]byte(stub))
	if err != nil {
		t.Fatal(err)
	}
	if findings := ValidateRequirements(doc); len(findings) != 0 {
		t.Fatalf("scaffold produced requirement findings: %+v", findings)
	}

	// Guard the two shapes the old template got wrong (plan 2.3): a heading that
	// keeps a "Requirement" word before the ID never parses, and a criterion
	// written without the trailing colon never parses.
	if len(RequirementIDSet("## Requirement R1 — x\n")) != 0 {
		t.Fatal("`## Requirement R1` must not parse as a requirement heading")
	}
	bad := ParseRequirementsMust(t, "## R1 — x\n\n- **R1.1** When a, the system shall b.\n")
	if len(bad.Requirements) == 1 && len(bad.Requirements[0].Criteria) != 0 {
		t.Fatal("`- **R1.1**` without a colon must not parse as a criterion")
	}
}

func ParseRequirementsMust(t *testing.T, raw string) RequirementsDoc {
	t.Helper()
	doc, err := ParseRequirements([]byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
