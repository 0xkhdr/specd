package core

import (
	"reflect"
	"testing"
)

// T3 / R1 regression closure. These tests lock the DAG/frontier contract:
// cycle detection refuses a frontier, incomplete deps exclude dependents, and
// frontier/wave order is deterministic regardless of input slice order (the
// solver builds an internal map, so input permutations must not leak through).

// permutations returns every ordering of the input tasks (n! — keep n small).
func permutations(tasks []DagTask) [][]DagTask {
	if len(tasks) <= 1 {
		return [][]DagTask{append([]DagTask(nil), tasks...)}
	}
	var out [][]DagTask
	for i := range tasks {
		rest := make([]DagTask, 0, len(tasks)-1)
		rest = append(rest, tasks[:i]...)
		rest = append(rest, tasks[i+1:]...)
		for _, p := range permutations(rest) {
			out = append(out, append([]DagTask{tasks[i]}, p...))
		}
	}
	return out
}

// R1.1 + R1.3: frontier and wave grouping are invariant under input order.
// Iterating the same logical DAG in every possible slice order must yield the
// identical sorted frontier and identical wave rows.
func TestFrontierDeterministicAcrossPermutations(t *testing.T) {
	base := []DagTask{
		{ID: "T1", Wave: 1, Status: TaskComplete},
		{ID: "T2", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
		{ID: "T3", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
		{ID: "T10", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
	}
	wantFrontier := []string{"T2", "T3", "T10"} // wave then numeric ordinal
	wantWave2 := []string{"T2", "T3", "T10"}

	for _, p := range permutations(base) {
		if got := ids(RunnableFrontier(p)); !reflect.DeepEqual(got, wantFrontier) {
			t.Fatalf("frontier = %v, want %v for input order %v", got, wantFrontier, ids(p))
		}
		rows := GroupWaves(p)
		if len(rows) != 2 || rows[0].Wave != 1 || rows[1].Wave != 2 {
			t.Fatalf("wave rows = %+v, want waves [1 2] for input %v", rows, ids(p))
		}
		if got := ids(rows[1].Tasks); !reflect.DeepEqual(got, wantWave2) {
			t.Fatalf("wave 2 = %v, want %v for input %v", got, wantWave2, ids(p))
		}
	}
}

// R1.4: a dependent is excluded from the frontier unless every dependency is
// complete. Pending, running, blocked, and missing deps all exclude.
func TestFrontierIncompleteDepExclusion(t *testing.T) {
	cases := []struct {
		name    string
		depStat TaskStatus
		depID   string // dependency referenced by the gated task
		want    bool   // gated task expected in frontier
	}{
		{"dep_complete_included", TaskComplete, "T1", true},
		{"dep_pending_excluded", TaskPending, "T1", false},
		{"dep_running_excluded", TaskRunning, "T1", false},
		{"dep_blocked_excluded", TaskBlocked, "T1", false},
		{"dep_missing_excluded", TaskComplete, "T99", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tasks := []DagTask{
				{ID: "T1", Wave: 1, Status: tc.depStat},
				{ID: "T2", Wave: 2, Depends: []string{tc.depID}, Status: TaskPending},
			}
			front := ids(RunnableFrontier(tasks))
			has := false
			for _, id := range front {
				if id == "T2" {
					has = true
				}
			}
			if has != tc.want {
				t.Errorf("frontier %v: T2 present=%v, want %v", front, has, tc.want)
			}
		})
	}
}

// R1.2: a cyclic graph is reported as a cycle, and the engine contract refuses
// to emit a frontier in that case (callers gate on DetectCycle before trusting
// RunnableFrontier). CriticalPath shares the same refusal.
func TestFrontierCycleRefused(t *testing.T) {
	cyclic := []DagTask{
		{ID: "T1", Wave: 1, Depends: []string{"T2"}, Status: TaskPending},
		{ID: "T2", Wave: 1, Depends: []string{"T1"}, Status: TaskPending},
	}
	cyc := DetectCycle(cyclic)
	if cyc == nil {
		t.Fatal("expected cycle to be reported")
	}
	if cyc[0] != cyc[len(cyc)-1] {
		t.Errorf("reported cycle path not closed: %v", cyc)
	}
	// Engine refusal contract: with a cycle present, no frontier may be trusted.
	if CriticalPath(cyclic) != nil {
		t.Error("CriticalPath must refuse (nil) on cyclic input")
	}
}
