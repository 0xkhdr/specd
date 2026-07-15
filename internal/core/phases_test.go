package core

import (
	"strings"
	"testing"
)

func TestPhaseRatchet(t *testing.T) {
	if phase, err := AdvanceStatus(StatusRequirements, StatusDesign); err != nil || phase != PhaseAnalyze {
		t.Fatalf("advance to design = %q, %v; want %q, nil", phase, err, PhaseAnalyze)
	}
	if _, err := AdvanceStatus(StatusDesign, StatusDesign); err == nil {
		t.Fatal("same-status advance succeeded")
	}
	if _, err := AdvanceStatus(StatusRequirements, StatusExecuting); err == nil {
		t.Fatal("skipped advance succeeded")
	}
	if _, err := AdvanceStatus(StatusExecuting, StatusTasks); err == nil {
		t.Fatal("backward advance succeeded")
	}
	if _, err := AdvanceStatus(StatusRequirements, Status("unknown")); err == nil {
		t.Fatal("unknown status advance succeeded")
	}
}

func TestAdvanceStatusExactSuccessorMatrix(t *testing.T) {
	statuses := []Status{
		StatusRequirements,
		StatusDesign,
		StatusTasks,
		StatusExecuting,
		StatusVerifying,
		StatusComplete,
		StatusBlocked,
		Status("unknown"),
	}
	wantNext := map[Status]Status{
		StatusRequirements: StatusDesign,
		StatusDesign:       StatusTasks,
		StatusTasks:        StatusExecuting,
		StatusExecuting:    StatusVerifying,
		StatusVerifying:    StatusComplete,
	}

	for _, from := range statuses {
		for _, to := range statuses {
			phase, err := AdvanceStatus(from, to)
			if wantNext[from] == to {
				if err != nil || phase != PhaseForStatus(to) {
					t.Errorf("AdvanceStatus(%q, %q) = %q, %v; want %q, nil", from, to, phase, err, PhaseForStatus(to))
				}
				continue
			}
			if err == nil {
				t.Errorf("AdvanceStatus(%q, %q) succeeded; want exact-successor refusal", from, to)
			}
		}
	}
}

func TestAdvanceStatusTerminalRefusalNamesMissingSuccessor(t *testing.T) {
	_, err := AdvanceStatus(StatusComplete, StatusComplete)
	if err == nil || !strings.Contains(err.Error(), "no lifecycle successor") {
		t.Fatalf("terminal advance error = %v; want missing-successor refusal", err)
	}
}
