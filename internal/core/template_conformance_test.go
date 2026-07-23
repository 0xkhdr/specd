package core

import (
	"strings"
	"testing"
)

// TestFreshScaffoldConsumerConformance is the suite whose absence let four template/consumer
// contracts drift (R5.1): each subtest asserts that a shipped template's bytes
// satisfy the consumer that parses them, named in the failure. The steering
// template's case lives in internal/context (SelectSteering is there); the pinky
// agent-definition case lives in pinky_agents_test.go.
func TestFreshScaffoldConsumerConformance(t *testing.T) {
	t.Run("requirements_template_satisfies_ParseRequirements", func(t *testing.T) {
		doc, err := ParseRequirements([]byte(RequirementsScaffold("demo")))
		if err != nil {
			t.Fatal(err)
		}
		if len(doc.Requirements) == 0 {
			t.Fatal("requirements scaffold parsed no requirements (consumer: ParseRequirements)")
		}
		if findings := ValidateRequirements(doc); len(findings) != 0 {
			t.Fatalf("requirements scaffold fails its own format gate: %+v", findings)
		}
	})

	t.Run("tasks_template_satisfies_quality_declaration", func(t *testing.T) {
		value := exampleField(t, TasksScaffold("demo"), "evidence")
		if _, err := ParseQualityContract(TaskRow{ID: "T1", Evidence: value}); err != nil {
			t.Fatalf("tasks scaffold example evidence=%q rejected (consumer: ParseQualityContract): %v", value, err)
		}
	})

	t.Run("tasks_template_parses_as_empty_but_valid_table", func(t *testing.T) {
		if _, err := ParseTasksMd([]byte(TasksScaffold("demo"))); err != nil {
			t.Fatalf("tasks scaffold does not parse (consumer: ParseTasksMd): %v", err)
		}
	})

	// Teeth (R5.1): a corrupted template must fail the suite. Reintroducing the
	// old "Requirement" word before the ID is exactly the regression 2.3 shipped;
	// the consumer must reject it, proving the assertions above are not vacuous.
	t.Run("corrupted_requirements_template_is_caught", func(t *testing.T) {
		corrupt := strings.Replace(RequirementsScaffold("demo"), "## R1 —", "## Requirement R1 —", 1)
		if len(RequirementIDSet(corrupt)) != 0 {
			t.Fatal("corrupted `## Requirement R1` heading still parsed — the suite has no teeth")
		}
	})
}
