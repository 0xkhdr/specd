package core

import (
	"reflect"
	"strconv"
	"testing"
)

func TestOrphanDeps(t *testing.T) {
	t.Parallel()
	tasks := []DagTask{
		{ID: "T1", Wave: 1, Depends: []string{"T2"}},
		{ID: "T2", Wave: 1},
	}
	orphans := OrphanDeps(tasks)
	if len(orphans) != 0 {
		t.Errorf("expected no orphans, got %v", orphans)
	}

	tasks2 := []DagTask{{ID: "T1", Wave: 1, Depends: []string{"T99"}}}
	orphans2 := OrphanDeps(tasks2)
	if len(orphans2) != 1 || orphans2[0].Dep != "T99" {
		t.Errorf("expected orphan T99, got %v", orphans2)
	}
}

func TestDetectCycle(t *testing.T) {
	t.Parallel()
	acyclic := []DagTask{
		{ID: "T1", Wave: 1, Depends: []string{}},
		{ID: "T2", Wave: 2, Depends: []string{"T1"}},
	}
	if c := DetectCycle(acyclic); c != nil {
		t.Errorf("expected no cycle, got %v", c)
	}

	cyclic := []DagTask{
		{ID: "T1", Wave: 1, Depends: []string{"T2"}},
		{ID: "T2", Wave: 1, Depends: []string{"T1"}},
	}
	if c := DetectCycle(cyclic); c == nil {
		t.Error("expected cycle, got nil")
	}
}

func TestNextRunnable(t *testing.T) {
	t.Parallel()
	tasks := []DagTask{
		{ID: "T1", Wave: 1, Depends: []string{}, Status: TaskComplete},
		{ID: "T2", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
		{ID: "T3", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
	}
	r := NextRunnable(tasks)
	if r.Kind != NextTask {
		t.Fatalf("expected task, got %s", r.Kind)
	}
	if r.ID != "T2" {
		t.Errorf("expected T2 (ordinal order), got %s", r.ID)
	}
}

func TestAllComplete(t *testing.T) {
	t.Parallel()
	tasks := []DagTask{{ID: "T1", Wave: 1, Status: TaskComplete}}
	r := NextRunnable(tasks)
	if r.Kind != NextAllComplete {
		t.Errorf("expected all-complete, got %s", r.Kind)
	}
}

func TestCriticalPath(t *testing.T) {
	t.Parallel()
	tasks := []DagTask{
		{ID: "T1", Wave: 1, Depends: []string{}},
		{ID: "T2", Wave: 2, Depends: []string{"T1"}},
		{ID: "T3", Wave: 3, Depends: []string{"T2"}},
	}
	cp := CriticalPath(tasks)
	if len(cp) != 3 {
		t.Errorf("expected path of length 3, got %v", cp)
	}
}

func TestCriticalPathCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		tasks []DagTask
		want  int // expected length; -1 means must be nil
	}{
		{"empty", nil, 0},
		{"single", []DagTask{{ID: "T1", Wave: 1}}, 1},
		{
			"linear",
			[]DagTask{
				{ID: "T1", Wave: 1},
				{ID: "T2", Wave: 1, Depends: []string{"T1"}},
				{ID: "T3", Wave: 1, Depends: []string{"T2"}},
			},
			3,
		},
		{
			"diamond",
			[]DagTask{
				{ID: "T1", Wave: 1},
				{ID: "T2", Wave: 1, Depends: []string{"T1"}},
				{ID: "T3", Wave: 1, Depends: []string{"T1"}},
				{ID: "T4", Wave: 1, Depends: []string{"T2", "T3"}},
			},
			3, // T1 -> T2|T3 -> T4
		},
		{
			"disconnected_longest_wins",
			[]DagTask{
				{ID: "T1", Wave: 1},
				{ID: "T2", Wave: 1, Depends: []string{"T1"}},
				{ID: "T8", Wave: 1},
				{ID: "T9", Wave: 1, Depends: []string{"T8"}},
				{ID: "T10", Wave: 1, Depends: []string{"T9"}},
			},
			3, // T8 -> T9 -> T10
		},
		{
			"cyclic_nil",
			[]DagTask{
				{ID: "T1", Wave: 1, Depends: []string{"T2"}},
				{ID: "T2", Wave: 1, Depends: []string{"T1"}},
			},
			-1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cp := CriticalPath(tt.tasks)
			if tt.want == -1 {
				if cp != nil {
					t.Fatalf("expected nil on cyclic input, got %v", cp)
				}
				return
			}
			if len(cp) != tt.want {
				t.Fatalf("expected path length %d, got %d (%v)", tt.want, len(cp), cp)
			}
		})
	}
}

func TestOrdinal(t *testing.T) {
	t.Parallel()
	// Total numeric (not lexicographic) order over valid ids T1..T20.
	prev := ordinal("T1")
	for i := 2; i <= 20; i++ {
		id := "T" + strconv.Itoa(i)
		got := ordinal(id)
		if got != i {
			t.Errorf("ordinal(%q) = %d, want %d", id, got, i)
		}
		if got <= prev {
			t.Errorf("ordinal not monotonic at %s: %d <= %d", id, got, prev)
		}
		prev = got
	}
	// Guards the NextRunnable sort: T10 must outrank T9 numerically.
	if ordinal("T10") <= ordinal("T9") {
		t.Errorf("expected ordinal(T10) > ordinal(T9), got %d vs %d", ordinal("T10"), ordinal("T9"))
	}
	// Id without a digit run sorts last (max int).
	if ordinal("T") != int(^uint(0)>>1) {
		t.Errorf("expected max int for digitless id, got %d", ordinal("T"))
	}
}

// syntheticDAG builds an n-task chain-plus-fan DAG for benchmarking. Each task
// depends on the previous one (a long critical path) and is laid out in waves
// of 10, exercising byID rebuilds, the runnable scan, and the sort.
func syntheticDAG(n int) []DagTask {
	tasks := make([]DagTask, n)
	for i := 0; i < n; i++ {
		var deps []string
		if i > 0 {
			deps = []string{"T" + strconv.Itoa(i)}
		}
		tasks[i] = DagTask{
			ID:      "T" + strconv.Itoa(i+1),
			Wave:    i / 10,
			Depends: deps,
			Status:  TaskPending,
		}
	}
	return tasks
}

func BenchmarkDetectCycle(b *testing.B) {
	tasks := syntheticDAG(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = DetectCycle(tasks)
	}
}

func BenchmarkNextRunnable(b *testing.B) {
	tasks := syntheticDAG(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = NextRunnable(tasks)
	}
}

func TestDagRunnableFrontier(t *testing.T) {
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
	t.Parallel()
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
