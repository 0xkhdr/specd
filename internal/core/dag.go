package core

import (
	"fmt"
	"sort"
)

type TaskDAG struct {
	Tasks []TaskRow
	ByID  map[string]TaskRow
}

func NewTaskDAG(tasks []TaskRow) (TaskDAG, error) {
	dag := TaskDAG{
		Tasks: append([]TaskRow(nil), tasks...),
		ByID:  make(map[string]TaskRow, len(tasks)),
	}
	for _, task := range dag.Tasks {
		if task.ID == "" {
			return TaskDAG{}, fmt.Errorf("task id is required")
		}
		if _, exists := dag.ByID[task.ID]; exists {
			return TaskDAG{}, formatDuplicateTask(task.ID)
		}
		dag.ByID[task.ID] = task
	}
	for _, task := range dag.Tasks {
		for _, dep := range task.DependsOn {
			if _, exists := dag.ByID[dep]; !exists {
				return TaskDAG{}, fmt.Errorf("task %s depends on unknown task %s", task.ID, dep)
			}
		}
	}
	if err := dag.detectCycle(); err != nil {
		return TaskDAG{}, err
	}
	return dag, nil
}

func (dag TaskDAG) TopologicalWaves() ([][]string, error) {
	remaining := make(map[string]TaskRow, len(dag.Tasks))
	for _, task := range dag.Tasks {
		remaining[task.ID] = task
	}
	var waves [][]string
	done := map[string]bool{}
	for len(remaining) > 0 {
		var wave []string
		for _, task := range dag.Tasks {
			if _, ok := remaining[task.ID]; !ok {
				continue
			}
			if depsSatisfied(task, done) {
				wave = append(wave, task.ID)
			}
		}
		if len(wave) == 0 {
			return nil, fmt.Errorf("task graph has a cycle")
		}
		sort.Strings(wave)
		for _, id := range wave {
			delete(remaining, id)
			done[id] = true
		}
		waves = append(waves, wave)
	}
	return waves, nil
}

// RunnableFrontier is the pending-and-ready projection (spec 03 R3.2). It owns
// no eligibility rules of its own: activity and readiness come from
// ProjectTaskStates so dependency truth exists in exactly one place.
func (dag TaskDAG) RunnableFrontier(status map[string]TaskRunStatus) ([]string, error) {
	states, err := ProjectTaskStates(dag.Tasks, status, nil)
	if err != nil {
		return nil, err
	}
	frontier := make([]string, 0, len(states))
	for _, state := range states {
		if state.Runnable() {
			frontier = append(frontier, state.ID)
		}
	}
	sort.Strings(frontier)
	return frontier, nil
}

func (dag TaskDAG) AllBlocked(status map[string]TaskRunStatus) bool {
	incomplete := 0
	for _, task := range dag.Tasks {
		current := status[task.ID]
		if current == TaskComplete {
			continue
		}
		incomplete++
		if current != TaskBlocked {
			return false
		}
	}
	return incomplete > 0
}

func (dag TaskDAG) detectCycle() error {
	visiting := map[string]bool{}
	visited := map[string]bool{}
	var visit func(string) error
	visit = func(id string) error {
		if visiting[id] {
			return fmt.Errorf("task graph has a cycle at %s", id)
		}
		if visited[id] {
			return nil
		}
		visiting[id] = true
		for _, dep := range dag.ByID[id].DependsOn {
			if err := visit(dep); err != nil {
				return err
			}
		}
		visiting[id] = false
		visited[id] = true
		return nil
	}
	for _, task := range dag.Tasks {
		if err := visit(task.ID); err != nil {
			return err
		}
	}
	return nil
}

func depsSatisfied(task TaskRow, done map[string]bool) bool {
	for _, dep := range task.DependsOn {
		if !done[dep] {
			return false
		}
	}
	return true
}
