package core

import "testing"

func TestRollupTelemetry(t *testing.T) {
	state := &State{
		Spec: "demo",
		Tasks: map[string]TaskState{
			"T1": {ID: "T1", Wave: 1, Telemetry: &Telemetry{DurationMs: 1000, VerifyDurationMs: 200, Retries: 1, Tokens: 100, Cost: "0.10"}},
			"T2": {ID: "T2", Wave: 1, Telemetry: &Telemetry{DurationMs: 500, Retries: 2, Tokens: 50, Cost: "0.05"}},
			"T3": {ID: "T3", Wave: 2, Telemetry: &Telemetry{DurationMs: 2000, Tokens: 200}}, // no cost
			"T4": {ID: "T4", Wave: 2},                                                       // no telemetry
		},
	}
	r := RollupTelemetry(state)
	if !r.HasData() {
		t.Fatal("want HasData true")
	}
	if r.DurationMs != 3500 || r.Retries != 3 || r.Tokens != 350 {
		t.Fatalf("spec totals wrong: %+v", r)
	}
	if r.Cost < 0.149 || r.Cost > 0.151 {
		t.Fatalf("spec cost want ~0.15, got %v", r.Cost)
	}
	if len(r.Waves) != 2 {
		t.Fatalf("want 2 waves, got %d", len(r.Waves))
	}
	w1 := r.Waves[0]
	if w1.Wave != 1 || w1.Tasks != 2 || w1.DurationMs != 1500 || w1.Retries != 3 {
		t.Fatalf("wave 1 wrong: %+v", w1)
	}
	w2 := r.Waves[1]
	if w2.Wave != 2 || w2.Tasks != 2 || w2.CostAnnotated {
		t.Fatalf("wave 2 wrong (should have no annotated cost): %+v", w2)
	}

	// Empty spec → no data.
	if RollupTelemetry(&State{Spec: "x", Tasks: map[string]TaskState{}}).HasData() {
		t.Fatal("empty spec should have no telemetry data")
	}
}
