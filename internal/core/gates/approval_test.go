package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestCoverageRefusalNamesRefsColumnAndRemedies pins spec R5.1: the coverage
// refusal states matching is done against the tasks.md `refs` column, lists
// every uncovered id, and names both remedies (add the id to `refs`, or mark
// the task `kind: deferred`). Matching semantics stay unchanged.
func TestCoverageRefusalNamesRefsColumnAndRemedies(t *testing.T) {
	tasks := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./...", Refs: []string{"R1.1"}}}
	ctx := CheckCtx{
		ApproveTarget: string(core.StatusExecuting),
		Tasks:         tasks,
		CoverageGaps:  []string{"R2", "R2.1"},
	}
	findings := coverageGate(ctx)
	if !HasErrors(findings) {
		t.Fatalf("coverage gaps not refused: %+v", findings)
	}
	message := findings[0].Message
	for _, want := range []string{"tasks.md `refs` column", "R2", "R2.1", "add each id to an implementing task's `refs` column", "`kind: deferred`"} {
		if !strings.Contains(message, want) {
			t.Errorf("refusal %q missing %q", message, want)
		}
	}
}

// TestCoverageGateArmingPerTarget pins spec R5.2: the tasks-phase approval
// runs the same coverage analysis at warning severity (approval proceeds, the
// gap is reported early), the executing transition keeps its blocking error
// severity, and every other target — or an untraced/gapless table — stays
// silent.
func TestCoverageGateArmingPerTarget(t *testing.T) {
	traced := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./...", Refs: []string{"R1.1"}}}

	tasksFindings := coverageGate(CheckCtx{ApproveTarget: string(core.StatusTasks), Tasks: traced, CoverageGaps: []string{"R2"}})
	if len(tasksFindings) != 1 || tasksFindings[0].Severity != Warn || !strings.Contains(tasksFindings[0].Message, "R2") {
		t.Fatalf("tasks-phase coverage advisory wrong: %+v", tasksFindings)
	}
	if HasErrors(tasksFindings) {
		t.Fatalf("tasks-phase coverage advisory must not block approval: %+v", tasksFindings)
	}
	execFindings := coverageGate(CheckCtx{ApproveTarget: string(core.StatusExecuting), Tasks: traced, CoverageGaps: []string{"R2"}})
	if !HasErrors(execFindings) {
		t.Fatalf("executing transition no longer blocks on coverage gaps: %+v", execFindings)
	}
	if f := coverageGate(CheckCtx{ApproveTarget: "design", Tasks: traced, CoverageGaps: []string{"R2"}}); len(f) != 0 {
		t.Fatalf("gate armed outside tasks/executing: %+v", f)
	}
	untraced := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."}}
	if f := coverageGate(CheckCtx{ApproveTarget: string(core.StatusExecuting), Tasks: untraced, CoverageGaps: []string{"R2"}}); len(f) != 0 {
		t.Fatalf("gate armed without task trace: %+v", f)
	}
	if f := coverageGate(CheckCtx{ApproveTarget: string(core.StatusExecuting), Tasks: traced}); len(f) != 0 {
		t.Fatalf("gate fired without gaps: %+v", f)
	}
}
