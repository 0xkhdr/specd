package gates

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestEvidencePolicyGateRefusesProductionBoundaryGap(t *testing.T) {
	ctx := CheckCtx{
		ApproveTarget:    string(core.StatusExecuting),
		ProductionPolicy: true,
		DesignDoc:        "- boundaries: external payment adapter\n",
		Tasks:            []core.TaskRow{{Evidence: "unit", Checks: "error path"}},
	}
	if findings := evidencePolicyGate(ctx); !HasErrors(findings) || !containsMsg(findings, "integration evidence") {
		t.Fatalf("production boundary gap passed: %+v", findings)
	}
}
