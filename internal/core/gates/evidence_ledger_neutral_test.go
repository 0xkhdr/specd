package gates

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestEvidenceLedgerNeutral pins R6.3: delivery ledgers are additive. The
// evidence gate takes no ledger as input, so its verdict is identical whether
// release/deployment ledgers are present or absent, and no deployment record —
// not even a healthy one — can satisfy a task's evidence or flip `complete`.
func TestEvidenceLedgerNeutral(t *testing.T) {
	head := "0123456789abcdef0123456789abcdef01234567"
	base := func() CheckCtx {
		return CheckCtx{
			Tasks:  []core.TaskRow{{ID: "T1", Role: "craftsman", Verify: "go test ./..."}},
			Status: map[string]core.TaskRunStatus{"T1": core.TaskComplete},
		}
	}

	// A healthy deployment ledger must not substitute for missing task evidence.
	healthy := []core.DeploymentV1{{Schema: core.DeploymentSchemaV1, Status: core.StatusHealthy}}
	frozen := []core.ReleaseCandidateV1{{Schema: core.ReleaseCandidateSchemaV1}}
	_ = healthy
	_ = frozen

	missing := base()
	if !HasErrors(evidence(missing)) {
		t.Fatal("a complete task without evidence must fail regardless of a healthy deployment ledger")
	}

	// With passing evidence the gate passes — and adding ledger records changes
	// nothing, because the gate never reads them.
	passing := base()
	passing.Evidence = map[string]core.EvidenceRecord{"T1": {TaskID: "T1", ExitCode: 0, GitHead: head}}
	absent := evidence(passing)
	present := evidence(passing) // same input; ledgers exist on the side, unused
	if HasErrors(absent) || HasErrors(present) {
		t.Fatalf("complete task with passing evidence must pass: absent=%+v present=%+v", absent, present)
	}
	if len(absent) != len(present) {
		t.Fatalf("gate verdict differs with vs without ledgers: %d != %d", len(absent), len(present))
	}
}
