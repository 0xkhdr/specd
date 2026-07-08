package gates

import (
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestApprovalGate(t *testing.T) {
	tasks := []core.TaskRow{{ID: "T1"}}
	progress := map[string]core.TaskRunStatus{"T1": core.TaskComplete}

	// Progress before approval → error.
	if f := approvalGate(CheckCtx{StateLoaded: true, Tasks: tasks, Status: progress}); !HasErrors(f) {
		t.Fatalf("progress before approval should error, got %+v", f)
	}
	// Both approved → clean.
	if f := approvalGate(CheckCtx{StateLoaded: true, ApprovedRequirements: true, ApprovedDesign: true, Tasks: tasks, Status: progress}); len(f) != 0 {
		t.Fatalf("approved should pass, got %+v", f)
	}
	// State not loaded → gate disabled (parity/unit contexts).
	if f := approvalGate(CheckCtx{Tasks: tasks, Status: progress}); len(f) != 0 {
		t.Fatalf("unloaded should skip, got %+v", f)
	}
}

func TestSyncGate(t *testing.T) {
	tasks := []core.TaskRow{{ID: "T1"}}
	marker := map[string]core.TaskRunStatus{"T1": core.TaskComplete}

	// Marker complete, state pending (absent) → disagreement → error.
	if f := syncGate(CheckCtx{StateLoaded: true, Tasks: tasks, Status: marker}); !HasErrors(f) {
		t.Fatalf("marker/state disagreement should error, got %+v", f)
	}
	// Agreement → clean.
	if f := syncGate(CheckCtx{StateLoaded: true, Tasks: tasks, Status: marker, StateTaskStatus: marker}); len(f) != 0 {
		t.Fatalf("agreement should pass, got %+v", f)
	}
	// State not loaded → gate disabled.
	if f := syncGate(CheckCtx{Tasks: tasks, Status: marker}); len(f) != 0 {
		t.Fatalf("unloaded should skip, got %+v", f)
	}
}
