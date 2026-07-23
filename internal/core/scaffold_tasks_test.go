package core

import (
	"strings"
	"testing"
)

// TestScaffoldTasksQualityDeclaration pins the tasks-template → quality-declaration
// contract (R2.2, R5.1): the `evidence=` value the scaffold teaches must parse
// through the real ParseQualityContract, and a bare class (no check-id) must
// still be rejected so the guard has teeth.
func TestFreshScaffoldConsumerConformanceTasks(t *testing.T) {
	value := exampleField(t, TasksScaffold("demo"), "evidence")
	if _, err := ParseQualityContract(TaskRow{ID: "T1", Evidence: value}); err != nil {
		t.Fatalf("scaffold example evidence=%q rejected by quality-declaration gate: %v", value, err)
	}

	if _, err := ParseQualityContract(TaskRow{ID: "T1", Evidence: "tests"}); err == nil {
		t.Fatal("a bare class without check-id must still be rejected")
	}
}

// exampleField pulls `key=<value>` out of the scaffold's example-values comment,
// stopping at the terminating `;` or `.` so the test pins the actual shipped
// token rather than a copy.
func exampleField(t *testing.T, doc, key string) string {
	t.Helper()
	idx := strings.Index(doc, key+"=")
	if idx < 0 {
		t.Fatalf("scaffold has no %s= example", key)
	}
	rest := doc[idx+len(key)+1:]
	rest = strings.SplitN(rest, ";", 2)[0]
	rest = strings.SplitN(rest, ".", 2)[0]
	return strings.TrimSpace(rest)
}
