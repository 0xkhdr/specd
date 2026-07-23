package gates

import (
	"os"
	"path/filepath"
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

// TestVerifyRunSelectorCouplingGate pins spec R2.1/R2.2: a `-run` selector
// naming a test must declare a *_test.go file (R2.1) and must match a `func
// Test...` in the declared files (R2.2); a selector with a matching declared
// test passes.
func TestVerifyRunSelectorCouplingGate(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a_test.go"), []byte("package a\n\nfunc TestFoo(t *T) {}\nfunc TestBar(t *T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	arm := string(core.StatusTasks)

	// R2.1: selector but no declared test file.
	noTest := []core.TaskRow{{ID: "T1", Files: "a.go", Verify: "go test ./x -run TestFoo"}}
	f := verifyLint(CheckCtx{ApproveTarget: arm, Root: root, Tasks: noTest})
	if !HasErrors(f) {
		t.Fatalf("R2.1: selector without declared test file not refused: %+v", f)
	}
	if !strings.Contains(f[0].Message, "T1") || !strings.Contains(f[0].Message, "TestFoo") {
		t.Fatalf("R2.1 finding missing row id or selector: %+v", f[0])
	}

	// R2.2: selector matches no declared Test func.
	missing := []core.TaskRow{{ID: "T2", Files: "a_test.go", Verify: "go test . -run TestNope"}}
	f = verifyLint(CheckCtx{ApproveTarget: arm, Root: root, Tasks: missing})
	if !HasErrors(f) {
		t.Fatalf("R2.2: selector matching no declared test not reported: %+v", f)
	}
	if !strings.Contains(f[0].Message, "T2") || !strings.Contains(f[0].Message, "TestNope") {
		t.Fatalf("R2.2 finding missing row id or selector: %+v", f[0])
	}

	// Valid: selector matches a declared Test func → no findings.
	valid := []core.TaskRow{{ID: "T3", Files: "a_test.go", Verify: "go test . -run TestFoo -count=2"}}
	if f := verifyLint(CheckCtx{ApproveTarget: arm, Root: root, Tasks: valid}); len(f) != 0 {
		t.Fatalf("valid selector refused: %+v", f)
	}
}
