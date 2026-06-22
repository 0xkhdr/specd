package core

import "testing"

func TestEvaluateCostBrakeWarnAndHalt(t *testing.T) {
	tests := []struct {
		name  string
		cost  float64
		limit float64
		want  CostBrakeLevel
	}{
		{"disabled", 999, 0, CostBrakeNone},
		{"under_soft", 7.99, 10, CostBrakeNone},
		{"soft_at_80_percent", 8.0, 10, CostBrakeWarn},
		{"between_soft_and_hard", 9.99, 10, CostBrakeWarn},
		{"hard_at_100_percent", 10.0, 10, CostBrakeHalt},
		{"hard_over_limit", 11.0, 10, CostBrakeHalt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EvaluateCostBrake(tt.cost, tt.limit); got != tt.want {
				t.Fatalf("EvaluateCostBrake(%v, %v) = %s, want %s", tt.cost, tt.limit, got, tt.want)
			}
		})
	}
}

func TestCostBrakeEventsAndNoDispatchAfterHalt(t *testing.T) {
	policy := validOrchestrationPolicy()
	policy.HostReportedCostLimitUSD = 10
	snapshot := validOrchestrationSnapshot()
	snapshot.Runnable = []OrchestrationTaskSnapshot{{ID: "T2", Wave: 1, Status: TaskPending, Attempt: 1, Role: "builder", Depends: []string{}}}

	var events []DriverEvent
	observer := func(ev DriverEvent) { events = append(events, ev) }

	snapshot.AccumulatedCostUSD = 8
	warnDecision, err := DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if warnDecision.Action != OrchestrationDispatch {
		t.Fatalf("warn decision = %s, want dispatch", warnDecision.Action)
	}
	emitCostBrakeEvent(observer, snapshot.SessionID, snapshot, policy, warnDecision)

	snapshot.AccumulatedCostUSD = 10
	haltDecision, err := DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if haltDecision.Action != OrchestrationEscalate || haltDecision.TaskID != "" {
		t.Fatalf("halt decision = %#v, want escalate with no task dispatch", haltDecision)
	}
	emitCostBrakeEvent(observer, snapshot.SessionID, snapshot, policy, haltDecision)

	if len(events) != 2 || events[0].Event != "cost_warn" || events[1].Event != "cost_halt" {
		t.Fatalf("events = %#v, want cost_warn then cost_halt", events)
	}
}
