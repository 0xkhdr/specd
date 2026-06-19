package core

import (
	"reflect"
	"testing"
)

func TestProgramOrchestrationDecide(t *testing.T) {
	t.Run("frontier_order_wave_then_slug_and_capacity", func(t *testing.T) {
		root := t.TempDir()
		scaffoldSpec(t, root, "a", StatusComplete)
		scaffoldSpec(t, root, "b", StatusExecuting)
		scaffoldSpec(t, root, "c", StatusExecuting)
		scaffoldSpec(t, root, "d", StatusExecuting)
		if err := SaveProgram(root, ProgramManifest{
			Version: ProgramVersion,
			DependsOn: map[string][]string{
				"b": {"a"},
				"c": {"a"},
				"d": {"b"},
			},
		}); err != nil {
			t.Fatal(err)
		}
		graph, err := BuildProgram(root, nil)
		if err != nil {
			t.Fatal(err)
		}
		snapshot, err := BuildProgramSnapshot(graph, map[string]bool{"b": true}, 2)
		if err != nil {
			t.Fatal(err)
		}
		decision, err := DecideProgram(snapshot)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Action != ProgramDecisionStart || !reflect.DeepEqual(decision.Specs, []string{"c"}) {
			t.Fatalf("decision = %#v, want start [c]", decision)
		}
	})

	t.Run("cycle_or_orphan_escalates", func(t *testing.T) {
		cycle := ProgramSnapshot{Version: OrchestrationModelVersion, Capacity: 1, Cycle: []string{"a", "b", "a"}}
		decision, err := DecideProgram(cycle)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Action != ProgramDecisionEscalate {
			t.Fatalf("cycle decision = %#v, want escalate", decision)
		}
		orphan := ProgramSnapshot{
			Version:  OrchestrationModelVersion,
			Capacity: 1,
			Orphans:  []struct{ Spec, Dep string }{{Spec: "b", Dep: "missing"}},
		}
		decision, err = DecideProgram(orphan)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Action != ProgramDecisionEscalate {
			t.Fatalf("orphan decision = %#v, want escalate", decision)
		}
	})

	t.Run("complete_and_capacity_wait", func(t *testing.T) {
		complete := ProgramSnapshot{
			Version:  OrchestrationModelVersion,
			Capacity: 1,
			Children: []ProgramChildSnapshot{{Slug: "a", Status: StatusComplete, Complete: true}},
		}
		decision, err := DecideProgram(complete)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Action != ProgramDecisionComplete {
			t.Fatalf("complete decision = %#v, want complete", decision)
		}
		wait := ProgramSnapshot{
			Version:     OrchestrationModelVersion,
			Capacity:    1,
			ActiveCount: 1,
			Children:    []ProgramChildSnapshot{{Slug: "a", Status: StatusExecuting}},
		}
		decision, err = DecideProgram(wait)
		if err != nil {
			t.Fatal(err)
		}
		if decision.Action != ProgramDecisionWait {
			t.Fatalf("wait decision = %#v, want wait", decision)
		}
	})
}
