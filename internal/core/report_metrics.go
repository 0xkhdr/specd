package core

import (
	"fmt"
	"sort"
	"strings"
)

func RenderMetrics(model ReportModel) string {
	var b strings.Builder
	fmt.Fprintf(&b, "specd_tasks_total{spec=%q} %d\n", model.Slug, model.Total)
	fmt.Fprintf(&b, "specd_tasks_complete{spec=%q} %d\n", model.Slug, model.Complete)
	fmt.Fprintf(&b, "specd_tasks_running{spec=%q} %d\n", model.Slug, model.Running)
	fmt.Fprintf(&b, "specd_tasks_blocked{spec=%q} %d\n", model.Slug, model.Blocked)
	fmt.Fprintf(&b, "specd_tasks_pending{spec=%q} %d\n", model.Slug, model.Pending)
	return b.String()
}

// WorkflowMetrics is the local, derived count of workflow activity. It is
// projected at READ time from workflow events, ledgers, and current state — no
// aggregate file is ever written and no second metrics store exists. Every
// count is an aggregate identity (transition names, totals); no source content
// or secret enters it.
type WorkflowMetrics struct {
	TransitionAttempts int            `json:"transition_attempts"`
	StaleDescendants   int            `json:"stale_descendants"`
	DeprecatedUse      int            `json:"deprecated_use"`
	ByTransition       map[string]int `json:"by_transition"`
}

// DeriveWorkflowMetrics computes workflow metrics at read time from already-loaded
// events and the compatibility diagnostics. It is pure: given the same inputs it
// returns the same counts, and it writes nothing. Transitions are counted by
// their governed identity, which covers refusals, waits, retries, reopen cycles,
// delegated approvals, and zero-progress halts without exposing any source.
func DeriveWorkflowMetrics(events []WorkflowEventV1, diagnostics []CompatDiagnostic) WorkflowMetrics {
	m := WorkflowMetrics{ByTransition: map[string]int{}}
	m.TransitionAttempts = len(events)
	for _, e := range events {
		m.ByTransition[e.Transition]++
	}
	m.StaleDescendants = len(StaleDescendants(events))
	for _, d := range diagnostics {
		if d.Active {
			m.DeprecatedUse++
		}
	}
	return m
}

// RenderWorkflowMetrics emits the derived metrics as a stable, textfile-collector
// exposition. Transition rows are sorted so the output is byte-stable.
func RenderWorkflowMetrics(slug string, m WorkflowMetrics) string {
	var b strings.Builder
	fmt.Fprintf(&b, "specd_workflow_transition_attempts{spec=%q} %d\n", slug, m.TransitionAttempts)
	fmt.Fprintf(&b, "specd_workflow_stale_descendants{spec=%q} %d\n", slug, m.StaleDescendants)
	fmt.Fprintf(&b, "specd_workflow_deprecated_use{spec=%q} %d\n", slug, m.DeprecatedUse)
	names := make([]string, 0, len(m.ByTransition))
	for name := range m.ByTransition {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		fmt.Fprintf(&b, "specd_workflow_transition{spec=%q,transition=%q} %d\n", slug, name, m.ByTransition[name])
	}
	return b.String()
}
