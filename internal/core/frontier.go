package core

import "sort"

// Readiness answers whether a task may start. It is projected, never a second
// copy of dependency truth (spec 03 R3.5): dependency waits are derived from the
// DAG plus task activity, and only manual approval, clarification, schedule,
// pause, and block facts persist (TaskFacts).
type Readiness string

const (
	ReadinessReady                Readiness = "ready"
	ReadinessWaitingDependency    Readiness = "waiting_dependency"
	ReadinessWaitingApproval      Readiness = "waiting_approval"
	ReadinessWaitingClarification Readiness = "waiting_clarification"
	ReadinessWaitingSchedule      Readiness = "waiting_schedule"
)

// readinessPriority is the stable order every applicable wait cause is reported
// in (R3.3). The lowest-priority cause present also names the task's readiness.
var readinessPriority = map[Readiness]int{
	ReadinessWaitingDependency:    0,
	ReadinessWaitingApproval:      1,
	ReadinessWaitingClarification: 2,
	ReadinessWaitingSchedule:      3,
}

// Wait reason codes. Derived dependency codes are owned by this file; manual
// codes are carried verbatim from the persisted fact.
const (
	WaitDependencyIncomplete = "dependency_incomplete"
	WaitDependencyTerminal   = "dependency_terminal_unresolved"
)

// WaitReason is one applicable cause a task is not ready, with the stable code,
// references, owner, recovery, and review/expiry a caller needs to act (R3.3).
type WaitReason struct {
	Code      string    `json:"code"`
	Readiness Readiness `json:"readiness"`
	Refs      []string  `json:"refs,omitempty"`
	Owner     string    `json:"owner,omitempty"`
	Recovery  string    `json:"recovery,omitempty"`
	Review    string    `json:"review,omitempty"`
}

// TaskFacts are the non-derived facts about one task. Activity carries the
// states the legacy marker cannot express; Waits are the manual approval,
// clarification, and schedule records; CoverageResolved is the explicit
// disposition that a cancelled or superseded task's acceptance is covered
// elsewhere, without which descendants stay unresolved.
// Attempt is the task's current attempt number, set by a reopen (spec 04 R3.1).
// Zero means the first attempt, so a task that was never reopened projects
// exactly as it did before attempts existed.
type TaskFacts struct {
	Activity         TaskActivity `json:"activity,omitempty"`
	Waits            []WaitReason `json:"waits,omitempty"`
	CoverageResolved bool         `json:"coverage_resolved,omitempty"`
	Attempt          int          `json:"attempt,omitempty"`
}

// TaskState is the projected activity plus readiness pair for one task (R3.1).
type TaskState struct {
	ID        string       `json:"id"`
	Activity  TaskActivity `json:"activity"`
	Readiness Readiness    `json:"readiness"`
	Waits     []WaitReason `json:"waits,omitempty"`
	// Attempt is omitted on the first attempt: only reopened work carries one.
	Attempt int `json:"attempt,omitempty"`
}

// Runnable reports whether the task may be picked up: accepted, unattempted,
// undisposed, and with no applicable wait (R3.2).
func (s TaskState) Runnable() bool {
	return s.Activity == ActivityPending && s.Readiness == ReadinessReady
}

// ProjectTaskStates derives activity and readiness for every task, in task-file
// order. Pure: no disk, no clock. facts may be nil.
func ProjectTaskStates(tasks []TaskRow, status map[string]TaskRunStatus, facts map[string]TaskFacts) ([]TaskState, error) {
	dag, err := NewTaskDAG(tasks)
	if err != nil {
		return nil, err
	}
	activity := make(map[string]TaskActivity, len(dag.Tasks))
	for _, task := range dag.Tasks {
		activity[task.ID] = taskActivity(task, status, facts)
	}
	states := make([]TaskState, 0, len(dag.Tasks))
	for _, task := range dag.Tasks {
		waits := append(dependencyWaits(task, activity, facts), facts[task.ID].Waits...)
		sort.SliceStable(waits, func(i, j int) bool {
			return readinessPriority[waits[i].Readiness] < readinessPriority[waits[j].Readiness]
		})
		state := TaskState{ID: task.ID, Activity: activity[task.ID], Readiness: ReadinessReady, Waits: waits, Attempt: facts[task.ID].Attempt}
		if len(waits) > 0 {
			state.Readiness = waits[0].Readiness
		}
		states = append(states, state)
	}
	return states, nil
}

// taskActivity resolves the activity view: a persisted fact wins, otherwise the
// run status, otherwise the tasks.md marker.
func taskActivity(task TaskRow, status map[string]TaskRunStatus, facts map[string]TaskFacts) TaskActivity {
	if fact := facts[task.ID].Activity; fact != "" {
		return fact
	}
	current := status[task.ID]
	if current == "" {
		current = statusFromMarker(task.Marker)
	}
	return ActivityFromStatus(current)
}

// dependencyWaits derives the dependency causes for one task. An incomplete
// dependency and a terminally disposed one without coverage are separate causes
// so both stay visible when both apply.
func dependencyWaits(task TaskRow, activity map[string]TaskActivity, facts map[string]TaskFacts) []WaitReason {
	var incomplete, unresolved []string
	for _, dep := range task.DependsOn {
		switch activity[dep] {
		case ActivityCompleted:
		case ActivityCancelled, ActivitySuperseded:
			if !facts[dep].CoverageResolved {
				unresolved = append(unresolved, dep)
			}
		default:
			incomplete = append(incomplete, dep)
		}
	}
	var waits []WaitReason
	if len(incomplete) > 0 {
		waits = append(waits, WaitReason{
			Code: WaitDependencyIncomplete, Readiness: ReadinessWaitingDependency, Refs: incomplete,
			Owner: "craftsman", Recovery: "complete the listed dependency tasks",
		})
	}
	if len(unresolved) > 0 {
		waits = append(waits, WaitReason{
			Code: WaitDependencyTerminal, Readiness: ReadinessWaitingDependency, Refs: unresolved,
			Owner: "human", Recovery: "record an acceptance coverage disposition for the listed tasks",
		})
	}
	return waits
}

// PendingCompletionBlockers lists the tasks that still block parent completion:
// every task left pending, ready or not, until it completes or takes an accepted
// terminal disposition (R3.4). Sorted for stable reporting.
func PendingCompletionBlockers(states []TaskState) []string {
	blockers := make([]string, 0, len(states))
	for _, state := range states {
		if state.Activity == ActivityPending {
			blockers = append(blockers, state.ID)
		}
	}
	sort.Strings(blockers)
	return blockers
}

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
