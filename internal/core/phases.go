package core

import "fmt"

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

func statusIndex(status Status) int {
	for i, known := range statusOrder {
		if status == known {
			return i
		}
	}
	return -1
}
