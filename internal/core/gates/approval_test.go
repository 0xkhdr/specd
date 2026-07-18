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

// TestCoverageGateArmingUnchanged pins that the wording change altered no
// semantics: the gate still arms only at the executing transition and only
// when the tasks carry a trace, and stays silent without gaps.
func TestCoverageGateArmingUnchanged(t *testing.T) {
	traced := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./...", Refs: []string{"R1.1"}}}

	if f := coverageGate(CheckCtx{ApproveTarget: string(core.StatusTasks), Tasks: traced, CoverageGaps: []string{"R2"}}); len(f) != 0 {
		t.Fatalf("gate armed outside executing transition: %+v", f)
	}
	untraced := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."}}
	if f := coverageGate(CheckCtx{ApproveTarget: string(core.StatusExecuting), Tasks: untraced, CoverageGaps: []string{"R2"}}); len(f) != 0 {
		t.Fatalf("gate armed without task trace: %+v", f)
	}
	if f := coverageGate(CheckCtx{ApproveTarget: string(core.StatusExecuting), Tasks: traced}); len(f) != 0 {
		t.Fatalf("gate fired without gaps: %+v", f)
	}
}
