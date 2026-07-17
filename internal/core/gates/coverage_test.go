package gates

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func containsMsg(findings []Finding, want string) bool {
	for _, finding := range findings {
		if strings.Contains(finding.Message, want) {
			return true
		}
	}
	return false
}

func TestCoverageGateRefusesExecutionWithCriterionGap(t *testing.T) {
	ctx := CheckCtx{
		ApproveTarget:   string(core.StatusExecuting),
		RequirementsDoc: "### R1 — demo\n\n- R1.1: When x, the system shall y.\n- R1.2: When z, the system shall q.\n",
		DesignDoc:       "- references: R1\n",
		Tasks:           []core.TaskRow{{ID: "T1", Refs: []string{"R1", "R1.1"}, Kind: "implementation"}},
	}
	if findings := coverageGate(ctx); !HasErrors(findings) || !containsMsg(findings, "R1.2") {
		t.Fatalf("coverage gap passed: %+v", findings)
	}
}

func TestCoverageGateKeepsMinimalTasksCompatible(t *testing.T) {
	ctx := CheckCtx{
		ApproveTarget:   string(core.StatusExecuting),
		RequirementsDoc: "### R1 — demo\n\n- R1.1: When x, the system shall y.\n",
		Tasks:           []core.TaskRow{{ID: "T1"}},
	}
	if findings := coverageGate(ctx); len(findings) != 0 {
		t.Fatalf("minimal task table should remain compatible: %+v", findings)
	}
}
