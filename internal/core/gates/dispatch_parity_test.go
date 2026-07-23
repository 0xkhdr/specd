package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestDispatchParityRefusesUnknownKind pins spec R1.1/R1.2: an unknown kind is
// refused at every readiness phase and the finding carries the row id, the
// rejected value, and the accepted vocabulary verbatim from ParseTaskContract.
func TestDispatchParityRefusesUnknownKind(t *testing.T) {
	bad := []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Kind: "epic"}}

	for _, target := range []string{"", string(core.StatusRequirements), string(core.StatusDesign), string(core.StatusTasks), string(core.StatusExecuting), string(core.StatusVerifying), string(core.StatusComplete)} {
		findings := dispatchParity(CheckCtx{Tasks: bad, ApproveTarget: target})
		if !HasErrors(findings) {
			t.Fatalf("target %q: unknown kind not refused", target)
		}
		for _, want := range []string{"T1", `"epic"`, "kind", "chore, deferred, docs, feature, fix, refactor, spike, test"} {
			if !strings.Contains(findings[0].Message, want) {
				t.Errorf("target %q: finding %q missing %q", target, findings[0].Message, want)
			}
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
	findings := dispatchParity(CheckCtx{Tasks: tasks})
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
	tasks := []core.TaskRow{
		{ID: "T1", Role: "craftsman", Files: "a.go", Kind: "feature", Risk: "high", Capabilities: []string{"context"}},
		{ID: "T2", Role: "scout", Files: "b.go", Kind: "docs"},
	}
	if f := dispatchParity(CheckCtx{Tasks: tasks}); len(f) != 0 {
		t.Fatalf("conforming plan refused: %+v", f)
	}
	if f := dispatchParity(CheckCtx{}); len(f) != 0 {
		t.Fatalf("empty CheckCtx produced findings: %+v", f)
	}
	badButUnarmed := []core.TaskRow{{ID: "T1", Files: "a.go", Kind: "epic"}}
	if f := dispatchParity(CheckCtx{Tasks: badButUnarmed, ApproveTarget: "not-a-phase"}); len(f) != 0 {
		t.Fatalf("unarmed target produced findings: %+v", f)
	}
}
