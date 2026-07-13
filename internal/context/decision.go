package context

import (
	"sort"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

// activeDecisionItems projects accepted, unexpired, non-superseded decisions
// into routine context. History retains all records separately.
func activeDecisionItems(records []core.DecisionV1, now time.Time) []Item {
	superseded := map[string]bool{}
	for _, r := range records {
		if r.Supersedes != "" {
			superseded[r.Supersedes] = true
		}
	}
	var items []Item
	for _, r := range records {
		if !superseded[r.ID] && r.ActiveAt(now) {
			items = append(items, Item{Kind: "decision", TaskID: r.ID, Required: false, Reason: "active governance decision"})
		}
	}
	sort.Slice(items, func(i, j int) bool { return items[i].TaskID < items[j].TaskID })
	return items
}

func loadActiveDecisionItems(root, slug string, now time.Time) ([]Item, error) {
	records, err := core.LoadDecisions(core.DecisionPath(root, slug))
	if err != nil {
		return nil, err
	}
	return activeDecisionItems(records, now), nil
}
