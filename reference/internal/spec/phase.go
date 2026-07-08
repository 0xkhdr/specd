package spec

// Phase is the PDCA-style execution phase a spec status maps to.
type Phase string

const (
	PhasePerceive Phase = "perceive"
	PhaseAnalyze  Phase = "analyze"
	PhasePlan     Phase = "plan"
	PhaseExecute  Phase = "execute"
	PhaseVerify   Phase = "verify"
	PhaseReflect  Phase = "reflect"
)

// PhaseForStatus maps a spec status to its execution phase.
func PhaseForStatus(status SpecStatus) Phase {
	switch status {
	case StatusRequirements:
		return PhaseAnalyze
	case StatusDesign, StatusTasks:
		return PhasePlan
	case StatusExecuting, StatusBlocked:
		return PhaseExecute
	case StatusVerifying:
		return PhaseVerify
	case StatusComplete:
		return PhaseReflect
	}
	return PhaseAnalyze
}
