package core

import "fmt"

// BuildProgramSnapshot builds a ProgramSnapshot from a ProgramGraph and a
// simple active/inactive map keyed by spec slug. It is a convenience wrapper
// over BuildProgramSnapshotWithRuntime for callers with no richer per-child
// runtime data (e.g. escalation or child session ID) to report.
func BuildProgramSnapshot(graph ProgramGraph, active map[string]bool, capacity int) (ProgramSnapshot, error) {
	runtime := make(map[string]ProgramChildRuntime, len(active))
	for slug, isActive := range active {
		runtime[slug] = ProgramChildRuntime{Active: isActive}
	}
	return BuildProgramSnapshotWithRuntime(graph, runtime, capacity)
}

// BuildProgramSnapshotWithRuntime assembles a ProgramSnapshot of the
// cross-spec program: one ProgramChildSnapshot per spec in graph (merging in
// per-child runtime state such as Active/Escalated/ChildSessionID), the count
// of currently active children, the worker capacity, any dependency cycle or
// orphaned dependencies, and the computed critical path. capacity must be
// positive.
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
