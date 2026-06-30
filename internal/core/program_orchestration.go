package core

// ProgramDecisionAction names the action a program-level decision can take.
type ProgramDecisionAction string

// The ProgramDecisionAction values are the possible outcomes DecideProgram
// can return.
const (
	ProgramDecisionStart    ProgramDecisionAction = "start"
	ProgramDecisionWait     ProgramDecisionAction = "wait"
	ProgramDecisionEscalate ProgramDecisionAction = "escalate"
	ProgramDecisionComplete ProgramDecisionAction = "complete"
)

// ProgramChildSnapshot is one child spec's state as seen by the program
// decision: its status, wave, dependencies, and completion/blocked/active/
// escalated flags, plus its child orchestration session ID if one is running.
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

// ProgramChildRuntime is the live runtime state of one child spec — whether
// it is actively running, escalated, and its child session ID — as observed
// independently of the persisted program graph.
type ProgramChildRuntime struct {
	Active         bool
	Escalated      bool
	ChildSessionID string
}

// ProgramSession is the persisted top-level orchestration session for a
// program: its schema version, parent session ID, lifecycle status, and
// creation/update timestamps.
type ProgramSession struct {
	Version         int                        `json:"version"`
	ParentSessionID string                     `json:"parentSessionId"`
	Status          OrchestrationSessionStatus `json:"status"`
	CreatedAt       string                     `json:"createdAt"`
	UpdatedAt       string                     `json:"updatedAt"`
}

// ProgramSnapshot is the point-in-time view of a program's dependency graph
// that DecideProgram consumes: its children, dispatch capacity and active
// count, any cycle or orphan dependencies, and the critical path.
type ProgramSnapshot struct {
	Version      int                          `json:"version"`
	Children     []ProgramChildSnapshot       `json:"children"`
	Capacity     int                          `json:"capacity"`
	ActiveCount  int                          `json:"activeCount"`
	Cycle        []string                     `json:"cycle"`
	Orphans      []struct{ Spec, Dep string } `json:"orphans"`
	CriticalPath []string                     `json:"criticalPath"`
}

// ProgramDecision is the outcome of DecideProgram: the action to take, the
// specs it applies to (when starting children), and a human-readable reason.
type ProgramDecision struct {
	Version int                   `json:"version"`
	Action  ProgramDecisionAction `json:"action"`
	Specs   []string              `json:"specs,omitempty"`
	Reason  string                `json:"reason"`
}
