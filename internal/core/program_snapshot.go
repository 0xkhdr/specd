package core

import "fmt"

func BuildProgramSnapshot(graph ProgramGraph, active map[string]bool, capacity int) (ProgramSnapshot, error) {
	runtime := make(map[string]ProgramChildRuntime, len(active))
	for slug, isActive := range active {
		runtime[slug] = ProgramChildRuntime{Active: isActive}
	}
	return BuildProgramSnapshotWithRuntime(graph, runtime, capacity)
}

func BuildProgramSnapshotWithRuntime(graph ProgramGraph, runtime map[string]ProgramChildRuntime, capacity int) (ProgramSnapshot, error) {
	if capacity < 1 {
		return ProgramSnapshot{}, fmt.Errorf("program orchestration: capacity must be positive")
	}
	children := make([]ProgramChildSnapshot, 0, len(graph.Specs))
	activeCount := 0
	for _, spec := range graph.Specs {
		childRuntime := runtime[spec.Slug]
		if childRuntime.Active {
			activeCount++
		}
		children = append(children, ProgramChildSnapshot{
			Slug:           spec.Slug,
			Status:         spec.Status,
			Wave:           spec.Wave,
			Depends:        append([]string{}, spec.DependsOn...),
			Complete:       spec.Complete,
			Blocked:        spec.Status == StatusBlocked,
			Active:         childRuntime.Active,
			Escalated:      childRuntime.Escalated,
			ChildSessionID: childRuntime.ChildSessionID,
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
