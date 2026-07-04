package cmd

import (
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/orchestration"
)

type brainDispatcher struct {
	dispatched []core.FrontierTask
}

func (dispatcher *brainDispatcher) Dispatch(task core.FrontierTask) error {
	dispatcher.dispatched = append(dispatcher.dispatched, task)
	return nil
}

func runBrainDispatch(frontier []core.FrontierTask, authority orchestration.Authority) (orchestration.Decision, []core.FrontierTask, error) {
	dispatcher := &brainDispatcher{}
	decision, err := orchestration.DispatchFrontier(
		orchestration.Snapshot{Frontier: frontier},
		orchestration.DecisionLimitsForAuthority(authority, orchestration.DecisionLimits{}),
		dispatcher,
	)
	return decision, dispatcher.dispatched, err
}
