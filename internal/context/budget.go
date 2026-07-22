package context

import (
	"fmt"

	"github.com/0xkhdr/specd/internal/core"
)

// CountsAgainstBudget reports whether a lane's tokens are part of the context
// cost (R2.5). Only required and loaded lanes count: a prospective output is an
// authorized path with no content, so it costs nothing and — being weightless
// authority rather than sheddable knowledge — is never a shedding candidate.
func CountsAgainstBudget(item MachineItem) bool {
	return item.Lane != LaneProspectiveOutput
}

// SelectableForContext reports whether a task should have context built for it
// as part of a sweep over the plan (R2.5). A terminal task is done: its context
// is no longer an active cost and must not fail a whole-plan budget check.
// Selecting a terminal task *explicitly* — to reopen or revalidate it — stays
// legal; this predicate only governs the sweep.
func SelectableForContext(status core.TaskRunStatus) bool {
	switch core.ActivityFromStatus(status) {
	case core.ActivityCompleted, core.ActivityCancelled, core.ActivitySuperseded:
		return false
	default:
		return true
	}
}

func ManifestBudget(manifest Manifest) int {
	total := 0
	total += EstimateNoLLM(manifest.Slug)
	total += EstimateNoLLM(manifest.TaskID)
	total += EstimateNoLLM(manifest.Mode)
	for _, item := range manifest.Items {
		total += EstimateNoLLM(item.Kind)
		total += EstimateNoLLM(item.Path)
		total += EstimateNoLLM(item.TaskID)
		total += item.EstimatedTokens
	}
	return total
}

func CheckBudget(manifest Manifest, maxTokens int) error {
	if maxTokens <= 0 {
		return nil
	}
	used := ManifestBudget(manifest)
	if used > maxTokens {
		return fmt.Errorf("context budget exceeded: %d > %d", used, maxTokens)
	}
	return nil
}
