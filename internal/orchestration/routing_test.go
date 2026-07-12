package orchestration

import (
	"reflect"
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
