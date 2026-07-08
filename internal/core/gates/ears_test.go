package gates

import "testing"

func TestEARSGate(t *testing.T) {
	stub := "# Requirements — demo\n\n- **R1** When <trigger>, the system shall <response>.\n"

	// Unedited scaffold stub → error (byte-compared against the template; the
	// EARS-shaped placeholder must not pass on shape alone).
	if f := earsGate(CheckCtx{RequirementsDoc: stub, RequirementsStub: stub}); !HasErrors(f) {
		t.Fatalf("stub should error, got %+v", f)
	}

	// Edited, EARS-shaped → clean.
	good := "# Requirements — demo\n\n- **R1** When a user runs check, the system shall validate.\n"
	if f := earsGate(CheckCtx{RequirementsDoc: good, RequirementsStub: stub}); len(f) != 0 {
		t.Fatalf("clean EARS should pass, got %+v", f)
	}

	// Non-EARS bullet → warn, never error.
	bad := "# Requirements — demo\n\n- R1 the system does stuff.\n"
	f := earsGate(CheckCtx{RequirementsDoc: bad, RequirementsStub: stub})
	if HasErrors(f) || len(f) != 1 || f[0].Severity != Warn {
		t.Fatalf("non-EARS should warn, got %+v", f)
	}

	// No doc provided → no findings (parity).
	if f := earsGate(CheckCtx{}); len(f) != 0 {
		t.Fatalf("empty ctx should be silent, got %+v", f)
	}
}
