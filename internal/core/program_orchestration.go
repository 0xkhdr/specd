package core

import "fmt"

type ProgramDecisionAction string

const (
	ProgramDecisionStart    ProgramDecisionAction = "start"
	ProgramDecisionWait     ProgramDecisionAction = "wait"
	ProgramDecisionEscalate ProgramDecisionAction = "escalate"
	ProgramDecisionComplete ProgramDecisionAction = "complete"
)

type ProgramChildSnapshot struct {
	Slug     string     `json:"slug"`
	Status   SpecStatus `json:"status"`
	Wave     int        `json:"wave"`
	Depends  []string   `json:"depends"`
	Complete bool       `json:"complete"`
	Blocked  bool       `json:"blocked"`
	Active   bool       `json:"active"`
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

func BuildProgramSnapshot(graph ProgramGraph, active map[string]bool, capacity int) (ProgramSnapshot, error) {
	if capacity < 1 {
		return ProgramSnapshot{}, fmt.Errorf("program orchestration: capacity must be positive")
	}
	children := make([]ProgramChildSnapshot, 0, len(graph.Specs))
	activeCount := 0
	for _, spec := range graph.Specs {
		isActive := active[spec.Slug]
		if isActive {
			activeCount++
		}
		children = append(children, ProgramChildSnapshot{
			Slug:     spec.Slug,
			Status:   spec.Status,
			Wave:     spec.Wave,
			Depends:  append([]string{}, spec.DependsOn...),
			Complete: spec.Complete,
			Blocked:  spec.Status == StatusBlocked,
			Active:   isActive,
		})
	}
	return ProgramSnapshot{
		Version:      OrchestrationModelVersion,
		Children:     children,
		Capacity:     capacity,
		ActiveCount:  activeCount,
		Cycle:        append([]string{}, graph.Cycle...),
		Orphans:      append([]struct{ Spec, Dep string }{}, graph.Orphans...),
		CriticalPath: CriticalPath(graph.Dag),
	}, nil
}

func DecideProgram(snapshot ProgramSnapshot) (ProgramDecision, error) {
	if snapshot.Version != OrchestrationModelVersion {
		return ProgramDecision{}, fmt.Errorf("program orchestration: unsupported snapshot version %d", snapshot.Version)
	}
	decision := ProgramDecision{Version: OrchestrationModelVersion}
	if len(snapshot.Cycle) > 0 {
		decision.Action = ProgramDecisionEscalate
		decision.Reason = "program graph has cycle"
		return decision, nil
	}
	if len(snapshot.Orphans) > 0 {
		decision.Action = ProgramDecisionEscalate
		decision.Reason = "program graph has orphan dependency"
		return decision, nil
	}
	for _, child := range snapshot.Children {
		if child.Blocked {
			decision.Action = ProgramDecisionEscalate
			decision.Reason = "child spec blocked"
			decision.Specs = []string{child.Slug}
			return decision, nil
		}
	}
	if allProgramChildrenComplete(snapshot.Children) {
		decision.Action = ProgramDecisionComplete
		decision.Reason = "all child specs complete"
		return decision, nil
	}
	available := snapshot.Capacity - snapshot.ActiveCount
	if available <= 0 {
		decision.Action = ProgramDecisionWait
		decision.Reason = "program capacity reached"
		return decision, nil
	}
	runnable := programRunnableChildren(snapshot.Children)
	if len(runnable) == 0 {
		decision.Action = ProgramDecisionWait
		decision.Reason = "waiting for dependencies"
		return decision, nil
	}
	if len(runnable) > available {
		runnable = runnable[:available]
	}
	decision.Action = ProgramDecisionStart
	decision.Specs = runnable
	decision.Reason = "frontier ready"
	return decision, nil
}

func allProgramChildrenComplete(children []ProgramChildSnapshot) bool {
	if len(children) == 0 {
		return true
	}
	for _, child := range children {
		if !child.Complete {
			return false
		}
	}
	return true
}

func programRunnableChildren(children []ProgramChildSnapshot) []string {
	bySlug := make(map[string]ProgramChildSnapshot, len(children))
	for _, child := range children {
		bySlug[child.Slug] = child
	}
	out := []string{}
	for _, child := range children {
		if child.Complete || child.Blocked || child.Active {
			continue
		}
		ready := true
		for _, dep := range child.Depends {
			if !bySlug[dep].Complete {
				ready = false
				break
			}
		}
		if ready {
			out = append(out, child.Slug)
		}
	}
	return out
}
