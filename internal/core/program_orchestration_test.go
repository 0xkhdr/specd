package core

import (
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"
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

func TestProgramOrchestrationLease(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	cfg, policy := programTestPolicy(t)
	parentIDs := []string{
		strings.Repeat("a", 32),
		strings.Repeat("b", 32),
		strings.Repeat("c", 32),
		strings.Repeat("d", 32),
		strings.Repeat("e", 32),
		strings.Repeat("f", 32),
	}

	var wg sync.WaitGroup
	for _, parentID := range parentIDs {
		wg.Add(1)
		go func(parentID string) {
			defer wg.Done()
			_, _ = StepProgramOrchestration(root, parentID, policy, cfg)
		}(parentID)
	}
	wg.Wait()

	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		t.Fatal(err)
	}
	active := 0
	owners := map[string]bool{}
	now := Clock().UTC()
	for _, lease := range leases {
		if programChildLeaseIsActive(lease, now) {
			active++
			owners[lease.ParentSessionID] = true
		}
	}
	if active != 1 || len(owners) != 1 {
		t.Fatalf("active leases = %d owners = %v, want one active owner", active, owners)
	}
}

func TestProgramOrchestrationCapacity(t *testing.T) {
	root := t.TempDir()
	for _, slug := range []string{"a", "b", "c"} {
		scaffoldSpec(t, root, slug, StatusExecuting)
	}
	cfg, policy := programTestPolicy(t)
	cfg.Program.MaxConcurrentSpecs = 2
	parentID := strings.Repeat("1", 32)

	result, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Action != ProgramDecisionStart || !reflect.DeepEqual(result.Decision.Specs, []string{"a", "b"}) {
		t.Fatalf("decision = %#v, want start [a b]", result.Decision)
	}
	active := 0
	for _, lease := range result.Leases {
		if programChildLeaseIsActive(lease, Clock().UTC()) {
			active++
		}
	}
	if active != 2 || len(result.Started) != 2 {
		t.Fatalf("active=%d started=%d, want 2", active, len(result.Started))
	}

	result, err = StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	active = 0
	for _, lease := range result.Leases {
		if programChildLeaseIsActive(lease, Clock().UTC()) {
			active++
		}
	}
	if active != 2 || len(result.Started) != 0 || result.Decision.Action != ProgramDecisionWait {
		t.Fatalf("second step active=%d started=%d decision=%#v, want capacity wait", active, len(result.Started), result.Decision)
	}
}

func TestProgramOrchestrationFrontier(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusComplete)
	scaffoldSpec(t, root, "b", StatusExecuting)
	scaffoldSpec(t, root, "c", StatusExecuting)
	if err := SaveProgram(root, ProgramManifest{Version: ProgramVersion, DependsOn: map[string][]string{
		"b": {"a"},
		"c": {"b"},
	}}); err != nil {
		t.Fatal(err)
	}
	cfg, policy := programTestPolicy(t)
	cfg.Program.MaxConcurrentSpecs = 1
	parentID := strings.Repeat("2", 32)

	first, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Decision.Specs, []string{"b"}) {
		t.Fatalf("first frontier = %v, want [b]", first.Decision.Specs)
	}
	state, err := LoadState(root, "b")
	if err != nil {
		t.Fatal(err)
	}
	state.Status = StatusComplete
	if err := SaveState(root, "b", state); err != nil {
		t.Fatal(err)
	}

	second, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second.Decision.Specs, []string{"c"}) {
		t.Fatalf("second frontier = %v, want [c]", second.Decision.Specs)
	}
	bySlug := map[string]ProgramChildLease{}
	for _, lease := range second.Leases {
		bySlug[lease.Slug] = lease
	}
	if bySlug["b"].Status != ProgramChildLeaseReleased || !programChildLeaseIsActive(bySlug["c"], Clock().UTC()) {
		t.Fatalf("leases after frontier recompute = %#v", bySlug)
	}
}

func programTestPolicy(t *testing.T) (OrchestrationCfg, OrchestrationPolicy) {
	t.Helper()
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	t.Cleanup(restore)
	cfg := DefaultConfig.Orchestration
	cfg.Enabled = true
	policy, err := NewOrchestrationPolicy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return cfg, policy
}
