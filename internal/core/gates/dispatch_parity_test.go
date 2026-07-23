package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestDispatchParityRefusesUnknownKind pins spec R1.1/R1.2: at the tasks
// approval an unknown kind is refused and the finding carries the row id, the
// rejected value, and the accepted vocabulary verbatim from ParseTaskContract.
// The gate is armed ONLY at the tasks target, so plain check (target "") — which
// specComplete runs — does not retroactively reject a legacy plan.
func TestDispatchParityRefusesUnknownKind(t *testing.T) {
	bad := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Kind: "epic"}}

	if !dispatchParityArmed(string(core.StatusTasks)) {
		t.Fatal("dispatch-parity must arm at the tasks approval target")
	}
	if dispatchParityArmed("") {
		t.Fatal("dispatch-parity must not arm at plain check (target \"\")")
	}
	if f := dispatchParity(CheckCtx{Tasks: bad, ApproveTarget: ""}); len(f) != 0 {
		t.Fatalf("dispatch-parity fired at target \"\": %+v", f)
	}

	findings := dispatchParity(CheckCtx{Tasks: bad, ApproveTarget: string(core.StatusTasks)})
	if !HasErrors(findings) {
		t.Fatal("unknown kind not refused at tasks target")
	}
	for _, want := range []string{"T1", `"epic"`, "kind", "chore, deferred, docs, feature, fix, refactor, spike, test"} {
		if !strings.Contains(findings[0].Message, want) {
			t.Errorf("finding %q missing %q", findings[0].Message, want)
		}
	}
}

// TestDispatchParityReportsEveryBadRow pins spec R1.3: a plan mixing conforming
// and nonconforming rows reports all bad rows, not just the first.
func TestDispatchParityReportsEveryBadRow(t *testing.T) {
	tasks := []core.TaskRow{
		{ID: "T1", Role: "craftsman", Files: "a.go", Kind: "epic"},
		{ID: "T2", Role: "craftsman", Files: "b.go", Kind: "feature"},
		{ID: "T3", Role: "craftsman", Files: "c.go", Kind: "story"},
	}
	findings := dispatchParity(CheckCtx{Tasks: tasks, ApproveTarget: string(core.StatusTasks)})
	if len(findings) != 2 {
		t.Fatalf("want 2 findings for 2 bad rows, got %d: %+v", len(findings), findings)
	}
	if !strings.Contains(findings[0].Message, "T1") || !strings.Contains(findings[1].Message, "T3") {
		t.Errorf("findings did not name every bad row in order: %+v", findings)
	}
}

// TestDispatchParityAcceptsConformingAndEmpty pins that a fully conforming plan
// and an empty CheckCtx both yield zero findings, and that an unarmed target is
// a no-op.
func TestDispatchParityAcceptsConformingAndEmpty(t *testing.T) {
	arm := string(core.StatusTasks)
	tasks := []core.TaskRow{
		{ID: "T1", Role: "craftsman", Files: "a.go", Kind: "feature", Risk: "high", Capabilities: []string{"context"}},
		{ID: "T2", Role: "scout", Files: "b.go", Kind: "docs"},
	}
	if f := dispatchParity(CheckCtx{Tasks: tasks, ApproveTarget: arm}); len(f) != 0 {
		t.Fatalf("conforming plan refused: %+v", f)
	}
	if f := dispatchParity(CheckCtx{ApproveTarget: arm}); len(f) != 0 {
		t.Fatalf("empty CheckCtx produced findings: %+v", f)
	}
	badButUnarmed := []core.TaskRow{{ID: "T1", Files: "a.go", Kind: "epic"}}
	if f := dispatchParity(CheckCtx{Tasks: badButUnarmed, ApproveTarget: "not-a-phase"}); len(f) != 0 {
		t.Fatalf("unarmed target produced findings: %+v", f)
	}
}
