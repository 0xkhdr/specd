package core

type FrontierTask struct {
	ID       string `json:"id"`
	Role     string `json:"role,omitempty"`
	Verify   string `json:"verify,omitempty"`
	Terminal string `json:"terminal,omitempty"`
}

type Wave struct {
	Index int      `json:"index"`
	Tasks []string `json:"tasks"`
}

func Frontier(tasks []TaskRow, status map[string]TaskRunStatus) ([]FrontierTask, error) {
	return FrontierExcluding(tasks, status, nil)
}

// FrontierExcluding is Frontier with an escalation filter: any task id present
// in escalated is dropped from the runnable frontier so neither `status` nor the
// Brain will pick it up until a human clears it with an override (spec 06 R2).
// A nil escalated set is exactly Frontier.
func FrontierExcluding(tasks []TaskRow, status map[string]TaskRunStatus, escalated map[string]bool) ([]FrontierTask, error) {
	dag, err := NewTaskDAG(tasks)
	if err != nil {
		return nil, err
	}
	ids, err := dag.RunnableFrontier(status)
	if err != nil {
		return nil, err
	}
	out := make([]FrontierTask, 0, len(ids))
	for _, id := range ids {
		if escalated[id] {
			continue
		}
		task := dag.ByID[id]
		out = append(out, FrontierTask{
			ID:       task.ID,
			Role:     task.Role,
			Verify:   task.Verify,
			Terminal: string(status[task.ID]),
		})
	}
	return out, nil
}

func ProjectWaves(tasks []TaskRow) ([]Wave, error) {
	dag, err := NewTaskDAG(tasks)
	if err != nil {
		return nil, err
	}
	groups, err := dag.TopologicalWaves()
	if err != nil {
		return nil, err
	}
	waves := make([]Wave, 0, len(groups))
	for i, group := range groups {
		waves = append(waves, Wave{Index: i + 1, Tasks: group})
	}
	return waves, nil
}
