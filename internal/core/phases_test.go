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
	for _, target := range []Status{StatusComplete, ""} {
		_, err := AdvanceStatus(StatusComplete, target)
		if err == nil || !strings.Contains(err.Error(), "no lifecycle successor") {
			t.Errorf("terminal advance to %q error = %v; want missing-successor refusal", target, err)
		}
	}
}

// TestStageConditionMigrationValidatorOwnsCombinations pins spec 03 R2.2: one
// canonical validator decides every stage/condition pair, and R2.3: the legacy
// status is the deterministic projection of that pair.
func TestStageConditionMigrationValidatorOwnsCombinations(t *testing.T) {
	for _, tc := range []struct {
		name string
		sc   StageCondition
		want bool
	}{
		{name: "executing_active", sc: StageCondition{Stage: StageExecuting, Condition: ConditionActive}, want: true},
		{name: "complete_complete", sc: StageCondition{Stage: StageComplete, Condition: ConditionComplete}, want: true},
		{name: "tasks_cancelled", sc: StageCondition{Stage: StageTasks, Condition: ConditionCancelled}, want: true},
		{name: "waiting_approval_with_request", sc: StageCondition{Stage: StageDesign, Condition: ConditionWaitingApproval, CurrentRequest: "req-1"}, want: true},
		{name: "complete_paused", sc: StageCondition{Stage: StageComplete, Condition: ConditionPaused}},
		{name: "executing_complete", sc: StageCondition{Stage: StageExecuting, Condition: ConditionComplete}},
		{name: "waiting_approval_without_request", sc: StageCondition{Stage: StageDesign, Condition: ConditionWaitingApproval}},
		{name: "unknown_stage", sc: StageCondition{Stage: "blocked", Condition: ConditionBlocked}},
		{name: "unknown_condition", sc: StageCondition{Stage: StageTasks, Condition: "stuck"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateStageCondition(tc.sc)
			if tc.want != (err == nil) {
				t.Fatalf("ValidateStageCondition(%+v) = %v, want valid = %v", tc.sc, err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		sc   StageCondition
		want Status
	}{
		{sc: StageCondition{Stage: StageTasks, Condition: ConditionActive}, want: StatusTasks},
		{sc: StageCondition{Stage: StageExecuting, Condition: ConditionPaused}, want: StatusExecuting},
		{sc: StageCondition{Stage: StageExecuting, Condition: ConditionBlocked}, want: StatusBlocked},
		{sc: StageCondition{Stage: StageTasks, Condition: ConditionCancelled}, want: StatusBlocked},
		{sc: StageCondition{Stage: StageComplete, Condition: ConditionComplete}, want: StatusComplete},
	} {
		if got := ProjectStatus(tc.sc); got != tc.want {
			t.Fatalf("ProjectStatus(%+v) = %q, want %q", tc.sc, got, tc.want)
		}
	}
}
