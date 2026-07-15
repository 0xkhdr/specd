package core

import "testing"

func TestPhaseRatchet(t *testing.T) {
	if phase, err := AdvanceStatus(StatusRequirements, StatusDesign); err != nil || phase != PhaseAnalyze {
		t.Fatalf("advance to design = %q, %v; want %q, nil", phase, err, PhaseAnalyze)
	}
	// W0 characterization: current ratchet accepts idempotent and skipped
	// targets. W1 flips these assertions to the exact-successor contract.
	if phase, err := AdvanceStatus(StatusDesign, StatusDesign); err != nil || phase != PhaseAnalyze {
		t.Fatalf("same-status advance = %q, %v; want %q, nil", phase, err, PhaseAnalyze)
	}
	if phase, err := AdvanceStatus(StatusRequirements, StatusExecuting); err != nil || phase != PhaseExecute {
		t.Fatalf("skipped advance = %q, %v; want %q, nil", phase, err, PhaseExecute)
	}
	if _, err := AdvanceStatus(StatusExecuting, StatusTasks); err == nil {
		t.Fatal("backward advance succeeded")
	}
	if _, err := AdvanceStatus(StatusRequirements, Status("unknown")); err == nil {
		t.Fatal("unknown status advance succeeded")
	}
}
