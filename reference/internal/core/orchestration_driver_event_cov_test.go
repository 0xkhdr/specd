package core

import "testing"

// orchestration_driver_event_cov_test.go covers the pure observer emitters
// emitDriverEvent and emitCostBrakeEvent.

func TestEmitDriverEvent(t *testing.T) {
	// nil observer is a no-op.
	emitDriverEvent(nil, "x", "s", DriverDispatch{})

	var got []DriverEvent
	obs := func(e DriverEvent) { got = append(got, e) }

	// Mission carries explicit worker/task IDs.
	emitDriverEvent(obs, "dispatch", "sess", DriverDispatch{
		Mission: PinkyMission{WorkerID: "w1", TaskID: "T1"},
	})
	// No mission IDs → fall back to decision-derived IDs.
	emitDriverEvent(obs, "retry", "sess", DriverDispatch{
		Decision: OrchestrationDecision{TaskID: "T2", Attempt: 1},
	})

	if len(got) != 2 {
		t.Fatalf("want 2 events, got %d", len(got))
	}
	if got[0].Worker != "w1" || got[0].Task != "T1" {
		t.Errorf("explicit IDs not threaded: %#v", got[0])
	}
	if got[1].Task != "T2" || got[1].Worker == "" {
		t.Errorf("decision fallback IDs missing: %#v", got[1])
	}
}

func TestEmitCostBrakeEvent(t *testing.T) {
	// nil observer is a no-op.
	emitCostBrakeEvent(nil, "s", OrchestrationSnapshot{}, OrchestrationPolicy{}, OrchestrationDecision{})

	var got []DriverEvent
	obs := func(e DriverEvent) { got = append(got, e) }
	policy := OrchestrationPolicy{HostReportedCostLimitUSD: 10}

	// Warn band (≥80% of the limit, below it).
	warnSnap := OrchestrationSnapshot{AccumulatedCostUSD: 9}
	emitCostBrakeEvent(obs, "s", warnSnap, policy, OrchestrationDecision{})

	// Halt band with an escalate decision → cost_halt.
	haltSnap := OrchestrationSnapshot{AccumulatedCostUSD: 50}
	emitCostBrakeEvent(obs, "s", haltSnap, policy, OrchestrationDecision{Action: OrchestrationEscalate})

	if len(got) == 0 {
		t.Fatal("expected cost-brake events")
	}
	sawWarn, sawHalt := false, false
	for _, e := range got {
		if e.Event == "cost_warn" {
			sawWarn = true
		}
		if e.Event == "cost_halt" {
			sawHalt = true
		}
	}
	if !sawWarn || !sawHalt {
		t.Errorf("warn=%v halt=%v, want both", sawWarn, sawHalt)
	}
}
