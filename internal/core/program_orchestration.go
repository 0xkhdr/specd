package core

type ProgramDecisionAction string

const (
	ProgramDecisionStart    ProgramDecisionAction = "start"
	ProgramDecisionWait     ProgramDecisionAction = "wait"
	ProgramDecisionEscalate ProgramDecisionAction = "escalate"
	ProgramDecisionComplete ProgramDecisionAction = "complete"
)

type ProgramChildSnapshot struct {
	Slug           string     `json:"slug"`
	Status         SpecStatus `json:"status"`
	Wave           int        `json:"wave"`
	Depends        []string   `json:"depends"`
	Complete       bool       `json:"complete"`
	Blocked        bool       `json:"blocked"`
	Active         bool       `json:"active"`
	Escalated      bool       `json:"escalated"`
	ChildSessionID string     `json:"childSessionId,omitempty"`
}

type ProgramChildRuntime struct {
	Active         bool
	Escalated      bool
	ChildSessionID string
}

type ProgramSession struct {
	Version         int                        `json:"version"`
	ParentSessionID string                     `json:"parentSessionId"`
	Status          OrchestrationSessionStatus `json:"status"`
	CreatedAt       string                     `json:"createdAt"`
	UpdatedAt       string                     `json:"updatedAt"`
}

type ProgramSnapshot struct {
	Version      int                          `json:"version"`
	Children     []ProgramChildSnapshot       `json:"children"`
	Capacity     int                          `json:"capacity"`
	ActiveCount  int                          `json:"activeCount"`
	Cycle        []string                     `json:"cycle"`
	Orphans      []struct{ Spec, Dep string } `json:"orphans"`
	CriticalPath []string                     `json:"criticalPath"`
}

type ProgramDecision struct {
	Version int                   `json:"version"`
	Action  ProgramDecisionAction `json:"action"`
	Specs   []string              `json:"specs,omitempty"`
	Reason  string                `json:"reason"`
}
