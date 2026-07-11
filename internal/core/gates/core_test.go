package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestRolesGate(t *testing.T) {
	if f := roles(CheckCtx{Tasks: []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."}}}); HasErrors(f) {
		t.Fatalf("known role should pass, got %+v", f)
	}
	f := roles(CheckCtx{Tasks: []core.TaskRow{{ID: "T1", Role: "wizard", Files: "a.go", Verify: "go test ./..."}}})
	if !HasErrors(f) {
		t.Fatal("unknown role should error")
	}
	if !strings.Contains(f[0].Message, "wizard") || !strings.Contains(f[0].Message, "T1") {
		t.Fatalf("finding should name task and role, got %+v", f)
	}
	if f := roles(CheckCtx{Tasks: []core.TaskRow{{ID: "T1", Files: "a.go"}}}); !HasErrors(f) {
		t.Fatal("empty role should error")
	}
}

func TestVerifyGate(t *testing.T) {
	trivial := core.DefaultTrivialVerify
	// Write task (craftsman) with a trivial verify → rejected (spec 01 R4.2).
	f := verifyCommands(CheckCtx{
		Tasks:         []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "printf ok"}},
		TrivialVerify: trivial,
	})
	if !HasErrors(f) {
		t.Fatal("craftsman with trivial verify should error")
	}
	if !strings.Contains(f[0].Message, "T1") {
		t.Fatalf("finding should name the task, got %+v", f)
	}
	// Read-only task (scout) may retain a trivial verify.
	if f := verifyCommands(CheckCtx{
		Tasks:         []core.TaskRow{{ID: "T1", Role: "scout", Files: "a.go", Verify: "printf ok"}},
		TrivialVerify: trivial,
	}); HasErrors(f) {
		t.Fatalf("scout with trivial verify should pass, got %+v", f)
	}
	// Write task with a real verify passes.
	if f := verifyCommands(CheckCtx{
		Tasks:         []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."}},
		TrivialVerify: trivial,
	}); HasErrors(f) {
		t.Fatalf("craftsman with real verify should pass, got %+v", f)
	}
	// Missing verify is still required regardless of trivial policy.
	if f := verifyCommands(CheckCtx{Tasks: []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go"}}}); !HasErrors(f) {
		t.Fatal("empty verify should error")
	}
}

func TestCoreGates(t *testing.T) {
	tasks := []core.TaskRow{
		{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./..."},
		{ID: "T2", Role: "craftsman", Files: "b.go", DependsOn: []string{"T1"}, Verify: "go test ./..."},
	}
	ctx := CheckCtx{
		Tasks:    tasks,
		Status:   map[string]core.TaskRunStatus{"T1": core.TaskComplete},
		Evidence: map[string]core.EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 0, GitHead: "abc"}},
	}
	if findings := CoreRegistry().Run(ctx); HasErrors(findings) {
		t.Fatalf("valid tasks produced errors: %#v", findings)
	}

	ctx.Evidence = map[string]core.EvidenceRecord{}
	findings := CoreRegistry().Run(ctx)
	if !HasErrors(findings) {
		t.Fatalf("missing evidence should fail")
	}
}
