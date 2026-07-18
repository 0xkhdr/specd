package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestQualityDeclarationGate pins spec R1.1/R1.2: a malformed `evidence` cell
// is a blocker for plain check and for tasks-phase approval, and the finding
// carries the self-documenting parse message (R1.3).
func TestQualityDeclarationGate(t *testing.T) {
	malformed := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./...", Evidence: "vibes/x"}}

	for _, target := range []string{"", string(core.StatusTasks), string(core.StatusExecuting)} {
		findings := qualityDeclaration(CheckCtx{Tasks: malformed, ApproveTarget: target})
		if !HasErrors(findings) {
			t.Fatalf("target %q: malformed declaration not refused", target)
		}
		for _, want := range []string{"T1", "test, output_eval, trajectory_eval, review", "class/check-id", "test/unit-auth"} {
			if !strings.Contains(findings[0].Message, want) {
				t.Errorf("target %q: finding %q missing %q", target, findings[0].Message, want)
			}
		}
	}

	// Requirements/design approvals are not armed: no tasks contract exists yet.
	for _, target := range []string{"requirements", "design"} {
		if f := qualityDeclaration(CheckCtx{Tasks: malformed, ApproveTarget: target}); len(f) != 0 {
			t.Fatalf("target %q: gate armed too early: %+v", target, f)
		}
	}
}

func TestQualityDeclarationGateAcceptsValidAndEmptyCells(t *testing.T) {
	tasks := []core.TaskRow{
		{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./...", Evidence: "test/unit, review/design-review"},
		{ID: "T2", Role: "scout", Files: "b.go", Verify: "printf ok"},
	}
	if f := qualityDeclaration(CheckCtx{Tasks: tasks}); len(f) != 0 {
		t.Fatalf("valid declarations refused: %+v", f)
	}
	if f := qualityDeclaration(CheckCtx{}); len(f) != 0 {
		t.Fatalf("empty CheckCtx produced findings: %+v", f)
	}
}

// TestQualityDeclarationRegistered pins that the gate runs from the core
// registry, so `specd check` and approval inherit it without wiring.
func TestQualityDeclarationRegistered(t *testing.T) {
	registered := false
	for _, name := range CoreRegistry().Names() {
		if name == "quality-declaration" {
			registered = true
		}
	}
	if !registered {
		t.Fatal("quality-declaration gate not registered in CoreRegistry")
	}
	ctx := CheckCtx{Tasks: []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "go test ./...", Evidence: "test"}}}
	found := false
	for _, finding := range CoreRegistry().Run(ctx) {
		if finding.Gate == "quality-declaration" && finding.Severity == Error {
			found = true
		}
	}
	if !found {
		t.Fatal("registry run did not surface quality-declaration blocker")
	}
}
