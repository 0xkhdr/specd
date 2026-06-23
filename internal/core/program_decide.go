package core

import "fmt"

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
		if child.Escalated {
			decision.Action = ProgramDecisionEscalate
			decision.Reason = "child spec escalated"
			decision.Specs = []string{child.Slug}
			return decision, nil
		}
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
