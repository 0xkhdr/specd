package orchestration

import (
	"reflect"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestRoutingDeterministicEligibleClass(t *testing.T) {
	policy := core.RoutingConfig{
		Version: "1", Classes: []string{"basic", "reviewed", "sandboxed"}, DefaultClass: "basic",
		Fallback: []string{"sandboxed", "reviewed", "basic"},
		ClassCapabilities: map[string][]string{
			"basic": {"context"}, "reviewed": {"context", "review"},
			"sandboxed": {"context", "eval", "review", "sandbox"},
		},
	}
	task := core.TaskRow{ID: "T1", Role: "craftsman", Risk: "high", Complexity: "complex", Capabilities: []string{"context", "sandbox", "review"}}
	first, err := RouteTask(task, policy)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RouteTask(task, policy)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) || first.Class != "sandboxed" || len(first.Fallback) != 0 {
		t.Fatalf("route = %#v second = %#v", first, second)
	}
}

func TestRoutingFallbackAndRefusal(t *testing.T) {
	policy := core.RoutingConfig{Version: "1", Classes: []string{"basic", "reviewed"}, DefaultClass: "basic", Fallback: []string{"reviewed", "basic"}, ClassCapabilities: map[string][]string{"basic": {"context"}, "reviewed": {"context", "review", "eval", "sandbox"}}}
	route, err := RouteTask(core.TaskRow{ID: "T1", Risk: "low", Capabilities: []string{"context"}}, policy)
	if err != nil {
		t.Fatal(err)
	}
	if route.Class != "reviewed" || !reflect.DeepEqual(route.Fallback, []string{"basic"}) {
		t.Fatalf("route = %#v", route)
	}
	if _, err := RouteTask(core.TaskRow{ID: "T2", Risk: "high", Capabilities: []string{"context"}}, core.RoutingConfig{Version: "1", Classes: []string{"basic"}, DefaultClass: "basic", Fallback: []string{"basic"}, ClassCapabilities: map[string][]string{"basic": {"context"}}}); err == nil {
		t.Fatal("high-risk task accepted class without review/eval/sandbox")
	}
}

// TestTaskContractConformance pins routing to the one typed task contract (spec
// 05 R1.1/R1.2/R1.4): routing reads canonical capability ids, refuses unknown
// ones against the task id and column instead of routing them, and the shipped
// tasks scaffold example routes under the default policy.
func TestTaskContractConformance(t *testing.T) {
	defaultPolicy := core.DefaultConfig.Routing

	t.Run("scaffold_example_routes_under_default_policy", func(t *testing.T) {
		route, err := RouteTask(core.TasksScaffoldExampleRow(), defaultPolicy)
		if err != nil {
			t.Fatalf("scaffold example does not route: %v", err)
		}
		if route.Class != defaultPolicy.DefaultClass {
			t.Fatalf("route = %#v", route)
		}
	})

	t.Run("unknown_capability_is_refused_not_routed", func(t *testing.T) {
		_, err := RouteTask(core.TaskRow{ID: "T3", Capabilities: []string{"telepathy"}}, defaultPolicy)
		if err == nil {
			t.Fatal("unknown capability routed")
		}
		for _, want := range []string{"TASK_FIELD_UNKNOWN", "task T3", "column capabilities"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error missing %q: %v", want, err)
			}
		}
	})

	t.Run("high_risk_escalation_uses_the_canonical_registry", func(t *testing.T) {
		// The escalation set routing demands for high risk must be exactly the
		// canonical task-capability vocabulary; otherwise a legal task row could
		// name a capability no class can ever satisfy.
		route, err := RouteTask(core.TaskRow{ID: "T4", Risk: "HIGH"}, defaultPolicy)
		if err != nil {
			t.Fatalf("high-risk task did not route under the default policy: %v", err)
		}
		want := "capabilities=" + strings.Join(core.CanonicalTaskCapabilities(), ",")
		if !strings.Contains(route.Reason, want) {
			t.Fatalf("route reason %q does not carry %q", route.Reason, want)
		}
	})
}
