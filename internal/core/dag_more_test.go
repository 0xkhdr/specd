package core

import (
	"reflect"
	"testing"
)

func ids(tasks []DagTask) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}

func TestDagRunnableFrontier(t *testing.T) {
	tasks := []DagTask{
		{ID: "T1", Wave: 1, Status: TaskComplete},
		{ID: "T3", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
		{ID: "T2", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
		{ID: "T4", Wave: 2, Depends: []string{"T1"}, Status: TaskRunning},  // excluded: running
		{ID: "T5", Wave: 2, Depends: []string{"T1"}, Status: TaskBlocked},  // excluded: blocked
		{ID: "T6", Wave: 3, Depends: []string{"T99"}, Status: TaskPending}, // excluded: dep incomplete
	}
	got := ids(RunnableFrontier(tasks))
	// Only T2,T3 runnable; ordered by wave then ordinal (T2 before T3).
	if !reflect.DeepEqual(got, []string{"T2", "T3"}) {
		t.Errorf("frontier = %v, want [T2 T3]", got)
	}
}

func TestDagGroupWaves(t *testing.T) {
	tasks := []DagTask{
		{ID: "T3", Wave: 2},
		{ID: "T1", Wave: 1},
		{ID: "T2", Wave: 1},
		{ID: "T10", Wave: 2},
	}
	rows := GroupWaves(tasks)
	if len(rows) != 2 {
		t.Fatalf("got %d wave rows, want 2", len(rows))
	}
	if rows[0].Wave != 1 || rows[1].Wave != 2 {
		t.Errorf("wave rows out of order: %d, %d", rows[0].Wave, rows[1].Wave)
	}
	if got := ids(rows[0].Tasks); !reflect.DeepEqual(got, []string{"T1", "T2"}) {
		t.Errorf("wave 1 = %v, want [T1 T2]", got)
	}
	// Numeric ordinal order within a wave: T3 before T10.
	if got := ids(rows[1].Tasks); !reflect.DeepEqual(got, []string{"T3", "T10"}) {
		t.Errorf("wave 2 = %v, want [T3 T10]", got)
	}
}

func TestDagWaveViolations(t *testing.T) {
	tasks := []DagTask{
		{ID: "T1", Wave: 1},
		{ID: "T2", Wave: 2, Depends: []string{"T1"}}, // fine: dep in earlier wave
		{ID: "T3", Wave: 1, Depends: []string{"T4"}}, // violation: dep T4 in later wave
		{ID: "T4", Wave: 2},
	}
	v := WaveViolations(tasks)
	if len(v) != 1 {
		t.Fatalf("got %d violations, want 1: %v", len(v), v)
	}
	if v[0].Task != "T3" || v[0].Dep != "T4" {
		t.Errorf("violation = %v, want {T3 T4}", v[0])
	}
}

func TestDetectCyclePath(t *testing.T) {
	t.Run("self_loop", func(t *testing.T) {
		c := DetectCycle([]DagTask{{ID: "T1", Depends: []string{"T1"}}})
		if !reflect.DeepEqual(c, []string{"T1", "T1"}) {
			t.Errorf("self-loop path = %v, want [T1 T1]", c)
		}
	})

	t.Run("multi_node_with_acyclic_tail", func(t *testing.T) {
		// T0 -> T1 -> T2 -> T3 -> T1 (cycle on T1..T3); T0 is an acyclic tail.
		tasks := []DagTask{
			{ID: "T0", Depends: []string{"T1"}},
			{ID: "T1", Depends: []string{"T2"}},
			{ID: "T2", Depends: []string{"T3"}},
			{ID: "T3", Depends: []string{"T1"}},
		}
		c := DetectCycle(tasks)
		if len(c) == 0 {
			t.Fatal("expected a cycle path, got nil")
		}
		// Path is closed: first == last, and the acyclic head T0 is not part of it.
		if c[0] != c[len(c)-1] {
			t.Errorf("cycle path not closed: %v", c)
		}
		for _, id := range c {
			if id == "T0" {
				t.Errorf("acyclic tail T0 leaked into cycle path: %v", c)
			}
		}
		// The closed cycle visits T1,T2,T3 exactly once before closing.
		if !reflect.DeepEqual(c, []string{"T1", "T2", "T3", "T1"}) {
			t.Errorf("cycle path = %v, want [T1 T2 T3 T1]", c)
		}
	})
}

func TestCriticalPathInvariant(t *testing.T) {
	// Documented invariant: CriticalPath returns nil iff DetectCycle is non-nil.
	cases := [][]DagTask{
		nil,
		{{ID: "T1"}},
		{{ID: "T1"}, {ID: "T2", Depends: []string{"T1"}}},
		{{ID: "T1", Depends: []string{"T2"}}, {ID: "T2", Depends: []string{"T1"}}}, // cyclic
		{{ID: "T1", Depends: []string{"T1"}}},                                      // self-loop
	}
	for i, tasks := range cases {
		hasCycle := DetectCycle(tasks) != nil
		cpNil := CriticalPath(tasks) == nil
		// For empty input both are nil/empty; treat empty path as "not a cycle" case.
		if len(tasks) == 0 {
			continue
		}
		if hasCycle != cpNil {
			t.Errorf("case %d: DetectCycle!=nil (%v) must match CriticalPath==nil (%v)", i, hasCycle, cpNil)
		}
	}
}
