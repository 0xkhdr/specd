package orchestration

import "github.com/0xkhdr/specd/internal/core"

type Dispatcher interface {
	Dispatch(task core.FrontierTask) error
}

func DispatchFrontier(snapshot Snapshot, limits DecisionLimits, dispatcher Dispatcher) (Decision, error) {
	decision := Decide(snapshot, limits)
	if decision.Action != ActionDispatch {
		return decision, nil
	}
	for _, task := range snapshot.Frontier {
		if task.ID == decision.TaskID {
			return decision, dispatcher.Dispatch(task)
		}
	}
	return Decision{Action: ActionWait, Reason: "selected task not found"}, nil
}
