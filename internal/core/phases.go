package core

import (
	"errors"
	"fmt"
)

type Status string

const (
	StatusRequirements Status = "requirements"
	StatusDesign       Status = "design"
	StatusTasks        Status = "tasks"
	StatusExecuting    Status = "executing"
	StatusVerifying    Status = "verifying"
	StatusComplete     Status = "complete"
	StatusBlocked      Status = "blocked"
)

type Phase string

const (
	PhasePerceive Phase = "perceive"
	PhaseAnalyze  Phase = "analyze"
	PhasePlan     Phase = "plan"
	PhaseExecute  Phase = "execute"
	PhaseVerify   Phase = "verify"
	PhaseReflect  Phase = "reflect"

	// PhaseAny is the sentinel a command declares when it is valid in every
	// lifecycle phase. It is never a real state phase (ValidPhase rejects it);
	// it exists only so command metadata declares "unrestricted" explicitly
	// rather than defaulting silently to it (spec 03 R6).
	PhaseAny Phase = "any"
)

var statusOrder = []Status{
	StatusRequirements,
	StatusDesign,
	StatusTasks,
	StatusExecuting,
	StatusVerifying,
	StatusComplete,
}

var phaseForStatus = map[Status]Phase{
	StatusRequirements: PhasePerceive,
	StatusDesign:       PhaseAnalyze,
	StatusTasks:        PhasePlan,
	StatusExecuting:    PhaseExecute,
	StatusVerifying:    PhaseVerify,
	StatusComplete:     PhaseReflect,
	StatusBlocked:      PhaseReflect,
}

func PhaseForStatus(status Status) Phase {
	return phaseForStatus[status]
}

func ValidStatus(status Status) bool {
	_, ok := phaseForStatus[status]
	return ok
}

func ValidPhase(phase Phase) bool {
	for _, known := range phaseForStatus {
		if phase == known {
			return true
		}
	}
	return false
}

func CanAdvanceStatus(from, to Status) bool {
	fromIndex := statusIndex(from)
	toIndex := statusIndex(to)
	return fromIndex >= 0 && toIndex == fromIndex+1
}

func AdvanceStatus(current, next Status) (Phase, error) {
	if !ValidStatus(current) {
		return "", fmt.Errorf("invalid current status %q", current)
	}
	currentIndex := statusIndex(current)
	if currentIndex < 0 || currentIndex+1 >= len(statusOrder) {
		return "", fmt.Errorf("status %q has no lifecycle successor", current)
	}
	if !ValidStatus(next) {
		return "", fmt.Errorf("invalid target status %q", next)
	}
	if !CanAdvanceStatus(current, next) {
		return "", fmt.Errorf("lifecycle approval from %q requires exact successor %q, got %q", current, statusOrder[currentIndex+1], next)
	}
	return PhaseForStatus(next), nil
}

// Stage is where a spec sits in its lifecycle; Condition is what it is doing
// there. Schema 2 stores the pair independently (spec 03 R2.1) so a blocked or
// paused spec no longer loses the stage it was blocked in.
type Stage string

const (
	StageRequirements Stage = "requirements"
	StageDesign       Stage = "design"
	StageTasks        Stage = "tasks"
	StageExecuting    Stage = "executing"
	StageVerifying    Stage = "verifying"
	StageComplete     Stage = "complete"
)

type Condition string

const (
	ConditionActive               Condition = "active"
	ConditionWaitingApproval      Condition = "waiting_approval"
	ConditionWaitingClarification Condition = "waiting_clarification"
	ConditionPaused               Condition = "paused"
	ConditionBlocked              Condition = "blocked"
	ConditionCancelled            Condition = "cancelled"
	ConditionComplete             Condition = "complete"
)

// StageCondition is the canonical lifecycle pair plus the identity a condition
// may require. ValidateStageCondition is the single owner of the legal
// combinations (spec 03 R2.2): proposal and load both route through it, and no
// second table of combinations may exist.
type StageCondition struct {
	Stage          Stage
	Condition      Condition
	CurrentRequest string
}

var validStages = map[Stage]bool{
	StageRequirements: true, StageDesign: true, StageTasks: true,
	StageExecuting: true, StageVerifying: true, StageComplete: true,
}

var validConditions = map[Condition]bool{
	ConditionActive: true, ConditionWaitingApproval: true, ConditionWaitingClarification: true,
	ConditionPaused: true, ConditionBlocked: true, ConditionCancelled: true, ConditionComplete: true,
}

func ValidStage(stage Stage) bool { return validStages[stage] }

func ValidCondition(condition Condition) bool { return validConditions[condition] }

func ValidateStageCondition(sc StageCondition) error {
	if !ValidStage(sc.Stage) {
		return fmt.Errorf("invalid spec stage %q", sc.Stage)
	}
	if !ValidCondition(sc.Condition) {
		return fmt.Errorf("invalid spec condition %q", sc.Condition)
	}
	// A finished lifecycle and a finished condition imply each other; only
	// cancellation may end a spec at any other stage. This is what makes
	// complete plus paused (or executing plus complete) unrepresentable.
	if sc.Condition != ConditionCancelled && (sc.Stage == StageComplete) != (sc.Condition == ConditionComplete) {
		return fmt.Errorf("invalid spec stage %q with condition %q", sc.Stage, sc.Condition)
	}
	if sc.Condition == ConditionWaitingApproval && sc.CurrentRequest == "" {
		return errors.New("condition waiting_approval requires a current approval request")
	}
	return nil
}

// ProjectStatus is the deterministic compatibility projection of the canonical
// pair onto the legacy status field (spec 03 R2.3). Legacy readers keep reading
// status; nothing mutates it as an independent fact.
func ProjectStatus(sc StageCondition) Status {
	switch sc.Condition {
	case ConditionBlocked, ConditionCancelled:
		return StatusBlocked
	default:
		return Status(sc.Stage)
	}
}

func statusIndex(status Status) int {
	for i, known := range statusOrder {
		if status == known {
			return i
		}
	}
	return -1
}
