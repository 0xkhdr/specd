package orchestration

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xkhdr/specd/internal/core"
)

type Route struct {
	Class    string
	Reason   string
	Fallback []string
}

// RouteTask selects provider-neutral capability classes using policy order.
// Provider/model resolution remains outside trusted core.
func RouteTask(task core.TaskRow, policy core.RoutingConfig) (Route, error) {
	// One typed contract (spec 05 R1.1/R1.4): routing does not re-split or
	// re-normalize raw cells, and an unknown capability id or risk tier is
	// refused here against the task id and column rather than silently routed.
	contract, err := core.ParseTaskContract(task)
	if err != nil {
		return Route{}, err
	}
	required := append([]string(nil), contract.Capabilities...)
	if contract.Risk == "high" || contract.Risk == "critical" {
		required = append(required, core.CanonicalTaskCapabilities()...)
	}
	required = uniqueSorted(required)
	var eligible []string
	for _, class := range policy.Fallback {
		if hasCapabilities(policy.ClassCapabilities[class], required) {
			eligible = append(eligible, class)
		}
	}
	if len(eligible) == 0 {
		return Route{}, fmt.Errorf("ROUTE_UNSUPPORTED: task %s requires %s", task.ID, strings.Join(required, ","))
	}
	return Route{Class: eligible[0], Reason: fmt.Sprintf("policy=%s risk=%s complexity=%s capabilities=%s", policy.Version, contract.Risk, contract.Complexity, strings.Join(required, ",")), Fallback: append([]string(nil), eligible[1:]...)}, nil
}

func uniqueSorted(values []string) []string {
	seen := map[string]bool{}
	for _, value := range values {
		if value != "" {
			seen[value] = true
		}
	}
	out := make([]string, 0, len(seen))
	for value := range seen {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func hasCapabilities(actual, required []string) bool {
	have := map[string]bool{}
	for _, capability := range actual {
		have[capability] = true
	}
	for _, capability := range required {
		if !have[capability] {
			return false
		}
	}
	return true
}
