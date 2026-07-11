package orchestration

import (
	"testing"
	"time"
)

// TestBrakeDormantWhenMaxCostUnset characterizes the W0 gap W4 closes: the cost
// brake only fires when MaxCost is a positive limit, so a run with no configured
// limit (MaxCost==0) never halts however high the accrued cost climbs. W4 makes
// an unset cost limit an explicit, honest state rather than a silently disabled
// brake.
func TestBrakeDormantWhenMaxCostUnset(t *testing.T) {
	snap := Snapshot{Cost: 1 << 30, Now: time.Now()}
	if d := EvaluateBrakes(snap, DecisionLimits{MaxCost: 0}); d.Action != "" {
		t.Fatalf("W0 gap closed early: unset cost limit already halts: %+v", d)
	}
	// A positive limit still trips, proving the brake works when armed.
	if d := EvaluateBrakes(snap, DecisionLimits{MaxCost: 10}); d.Action != ActionHalt {
		t.Fatalf("armed cost brake failed to halt: %+v", d)
	}
}
