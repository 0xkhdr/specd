package gates

import "testing"

func TestDesignGateContract(t *testing.T) {
	reqs := "### R1 — Title\n\n- R1.1: When x, the system shall y.\n"
	base := "# Design\n\n## Modules\nm runs gates.\n"

	// Unknown requirement reference is always refused (spec 01 R2.2).
	badRef := base + "\n- references: R9\n"
	if f := designGate(CheckCtx{ApproveTarget: "design", DesignDoc: badRef, RequirementsDoc: reqs}); !HasErrors(f) {
		t.Fatalf("unknown ref should refuse, got %+v", f)
	}

	// Known reference under the default profile approves (R7.1 compatibility).
	okDefault := base + "\n- references: R1\n"
	if f := designGate(CheckCtx{ApproveTarget: "design", DesignDoc: okDefault, RequirementsDoc: reqs}); len(f) != 0 {
		t.Fatalf("known ref default profile should pass, got %+v", f)
	}

	// Production profile demands the full decision contract (spec 01 R2.1).
	if f := designGate(CheckCtx{ApproveTarget: "design", DesignDoc: okDefault, RequirementsDoc: reqs, DesignContractRequired: true}); !HasErrors(f) {
		t.Fatalf("production profile incomplete contract should refuse, got %+v", f)
	}

	// A gate that is not the design transition stays disabled.
	if f := designGate(CheckCtx{ApproveTarget: "", DesignDoc: badRef, RequirementsDoc: reqs}); len(f) != 0 {
		t.Fatalf("non-design target should skip, got %+v", f)
	}
}
