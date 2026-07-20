package integration

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// The policy lives in internal/core; this package re-exports it as the
// host-facing surface. The behavioural table is in core's own test. What must
// hold here is that the re-export is the same policy and not a second copy that
// could drift — a wrapper returning a different answer is the failure this
// catches.
func TestHostContractReExportMatchesCorePolicy(t *testing.T) {
	for _, sandbox := range []bool{true, false} {
		contract := ReferenceHostContract()
		contract.Sandbox = sandbox

		viaIntegration := EvaluateHostContract(contract)
		viaCore := core.EvaluateHostContract(contract)

		if viaIntegration.Assurance != viaCore.Assurance {
			t.Fatalf("sandbox=%v: integration says %q, core says %q", sandbox, viaIntegration.Assurance, viaCore.Assurance)
		}
		if viaIntegration.Governed != viaCore.Governed {
			t.Fatalf("sandbox=%v: governed disagrees", sandbox)
		}
		if len(viaIntegration.Unmet) != len(viaCore.Unmet) {
			t.Fatalf("sandbox=%v: unmet sets differ: %v vs %v", sandbox, viaIntegration.Unmet, viaCore.Unmet)
		}
	}
	if len(HumanOnlyOperations()) != len(core.HumanOnlyOperations()) {
		t.Fatal("human-only set differs between the re-export and core")
	}
}
