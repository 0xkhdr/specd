package gates

import (
	"strings"
	"testing"
)

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

func TestEARSGateStructuredFindings(t *testing.T) {
	// A structured doc with a duplicate criterion and a non-EARS clause must
	// produce addressable Error findings naming the offending IDs (R1.2).
	doc := "### R1 — Group\n\n- R1.1: When a user acts, system shall respond.\n- R1.1: this one is malformed.\n"
	f := earsGate(CheckCtx{RequirementsDoc: doc})
	if !HasErrors(f) {
		t.Fatalf("structured defects should error, got %+v", f)
	}
	var sawDup bool
	for _, finding := range f {
		if strings.Contains(finding.Message, "R1.1") {
			sawDup = true
		}
	}
	if !sawDup {
		t.Fatalf("findings should name the offending id R1.1, got %+v", f)
	}

	// A valid structured doc passes clean.
	good := "### R1 — Group\n\n- R1.1: When a user acts, the system shall respond.\n"
	if f := earsGate(CheckCtx{RequirementsDoc: good}); len(f) != 0 {
		t.Fatalf("valid structured doc should pass, got %+v", f)
	}
}
