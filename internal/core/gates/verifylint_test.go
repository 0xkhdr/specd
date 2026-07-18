package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestVerifyLintFlagsFragileShellPatterns pins spec R8.1: `kill %N` and a
// trailing `&` without `$!` capture in a verify cell each yield one
// warning-severity finding at the tasks-phase check — advisory only, never a
// block.
func TestVerifyLintFlagsFragileShellPatterns(t *testing.T) {
	tasks := []core.TaskRow{
		{ID: "T1", Verify: "server & sleep 1; curl localhost; kill %1"},
		{ID: "T2", Verify: "server & PID=$!; curl localhost; kill $PID"},
		{ID: "T3", Verify: "go test ./... &"},
		{ID: "T4", Verify: "go test ./a && go test ./b"},
		{ID: "T5", Verify: "go test ./..."},
	}
	findings := verifyLint(CheckCtx{ApproveTarget: string(core.StatusTasks), Tasks: tasks})
	if HasErrors(findings) {
		t.Fatalf("verify lint must never block: %+v", findings)
	}
	if len(findings) != 2 {
		t.Fatalf("want kill-%%N and trailing-& findings only, got %+v", findings)
	}
	for _, finding := range findings {
		if finding.Severity != Warn {
			t.Fatalf("finding not warning severity: %+v", finding)
		}
	}
	if !strings.Contains(findings[0].Message, "T1") || !strings.Contains(findings[0].Message, "kill %N") {
		t.Fatalf("kill %%N finding wrong: %+v", findings[0])
	}
	if !strings.Contains(findings[1].Message, "T3") || !strings.Contains(findings[1].Message, "$!") {
		t.Fatalf("trailing-& finding wrong: %+v", findings[1])
	}
}

// TestVerifyLintArmingAndCleanCommands pins that clean commands pass silently
// and the lint arms only for plain check and the tasks-phase approval.
func TestVerifyLintArmingAndCleanCommands(t *testing.T) {
	clean := []core.TaskRow{{ID: "T1", Verify: "go test ./... && printf ok"}, {ID: "T2", Verify: ""}}
	if f := verifyLint(CheckCtx{Tasks: clean}); len(f) != 0 {
		t.Fatalf("clean commands flagged: %+v", f)
	}
	fragile := []core.TaskRow{{ID: "T1", Verify: "kill %1"}}
	if f := verifyLint(CheckCtx{ApproveTarget: "", Tasks: fragile}); len(f) != 1 {
		t.Fatalf("plain check not armed: %+v", f)
	}
	if f := verifyLint(CheckCtx{ApproveTarget: "design", Tasks: fragile}); len(f) != 0 {
		t.Fatalf("armed outside tasks-phase check: %+v", f)
	}
}
