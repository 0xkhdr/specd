package orchestration

import (
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// TestDecideEscalated documents the escalation contract at the decision layer:
// escalated tasks are withheld from the frontier upstream (FrontierExcluding),
// so Decide dispatches only remaining runnable work and never an escalated task
// (spec 06 R2).
func TestDecideEscalated(t *testing.T) {
	limits := DecisionLimits{AllowDispatch: true, MaxRetries: 3}

	t.Run("dispatches_remaining_task_not_escalated_one", func(t *testing.T) {
		snap := Snapshot{Frontier: []core.FrontierTask{{ID: "T2"}}, Now: time.Now()}
		d := Decide(snap, limits)
		if d.Action != ActionDispatch || d.TaskID != "T2" {
			t.Fatalf("decide = %+v, want dispatch T2", d)
		}
	})

	t.Run("waits_when_escalation_empties_the_frontier", func(t *testing.T) {
		d := Decide(Snapshot{Frontier: nil, Now: time.Now()}, limits)
		if d.Action == ActionDispatch {
			t.Fatalf("decide dispatched from an empty frontier: %+v", d)
		}
	})
}
