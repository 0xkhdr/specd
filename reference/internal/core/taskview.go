package core

// TaskView is the merged "doc overrides state" projection of a single task: the
// authoritative tasks.md view (doc) layered over the persisted task state, with
// fields absent from the doc falling back to state. It is the shared source of
// truth for the dispatch and next renderers, which previously each open-coded
// this merge.
type TaskView struct {
	ID           string
	Title        string
	Role         string
	Wave         int
	Meta         map[string]string
	Depends      []string
	Requirements []int
	FromDoc      bool // true when the task is present in tasks.md (doc)
}

// ResolveTaskView merges the doc view of task id over its persisted state.
// Title/Wave/Meta come from the doc when present; Role is overridden by the
// doc's role meta only when non-empty; Depends/Requirements always come from
// state (the doc does not carry resolved dependency state).
func ResolveTaskView(doc ParsedTasks, state *State, id string) TaskView {
	ts := state.Tasks[id]
	v := TaskView{
		ID:           id,
		Title:        ts.Title,
		Role:         ts.Role,
		Wave:         ts.Wave,
		Meta:         map[string]string{},
		Depends:      ts.Depends,
		Requirements: ts.Requirements,
	}
	if t := FindTask(doc, id); t != nil {
		v.FromDoc = true
		v.Title = t.Title
		v.Wave = t.Wave
		v.Meta = t.Meta
		if r := t.Meta["role"]; r != "" {
			v.Role = r
		}
	}
	return v
}
