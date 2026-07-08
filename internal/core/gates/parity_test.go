package gates

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestParity (R4.4): the W4 gates contribute nothing to a green spec, so
// check output stays byte-identical to pre-W4 — the new gates only speak when
// they have a violation to report.
func TestParity(t *testing.T) {
	green := CheckCtx{
		StateLoaded:          true,
		ApprovedRequirements: true,
		ApprovedDesign:       true,
		Tasks:                []core.TaskRow{{ID: "T1", Role: "craftsman", Files: "a.go", Verify: "true"}},
		Status:               map[string]core.TaskRunStatus{"T1": core.TaskPending},
		StateTaskStatus:      map[string]core.TaskRunStatus{"T1": core.TaskPending},
		RequirementsDoc:      "- **R1** When a user runs check, the system shall validate.\n",
		RequirementsStub:     "# a different unedited stub\n",
	}
	for _, f := range CoreRegistry().Run(green) {
		switch f.Gate {
		case "ears", "approval", "sync", "design":
			t.Fatalf("W4 gate %q fired on a green spec: %+v", f.Gate, f)
		}
	}
}

func TestByteIdenticalWhenOptInsOff(t *testing.T) {
	ctx := CheckCtx{}
	a := CoreRegistry().Run(ctx)
	b := CoreRegistry().Run(ctx)
	if len(a) != len(b) {
		t.Fatalf("finding count differs: %d != %d", len(a), len(b))
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("finding %d differs: %+v != %+v", i, a[i], b[i])
		}
	}
}
