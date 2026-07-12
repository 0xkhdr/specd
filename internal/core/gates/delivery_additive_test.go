package gates

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestEvidenceAdditive(t *testing.T) {
	ctx := CheckCtx{
		Tasks:  []core.TaskRow{{ID: "T1", Role: "craftsman", Verify: "go test ./..."}},
		Status: map[string]core.TaskRunStatus{"T1": core.TaskComplete},
	}
	deploymentLedger := []core.DeploymentV1{{Schema: core.DeploymentSchemaV1, Status: core.StatusHealthy}}
	if len(deploymentLedger) != 1 {
		t.Fatal("delivery fixture setup failed")
	}
	if findings := evidence(ctx); !HasErrors(findings) {
		t.Fatalf("delivery record satisfied missing task evidence: %+v", findings)
	}
}
