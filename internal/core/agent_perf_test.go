package core

import (
	"strconv"
	"testing"
)

// stateWithTasks builds an executing-spec State with an n-task chain DAG (waves
// of 10) for determinism goldens and render/frontier benchmarks. T1 is left
// complete so a frontier exists at T2.
func stateWithTasks(n int) *State {
	tasks := make(map[string]TaskState, n)
	for i := 0; i < n; i++ {
		id := "T" + strconv.Itoa(i+1)
		var deps []string
		if i > 0 {
			deps = []string{"T" + strconv.Itoa(i)}
		}
		st := TaskPending
		if i == 0 {
			st = TaskComplete
		}
		tasks[id] = TaskState{
			ID:           id,
			Title:        "task " + id,
			Wave:         i / 10,
			Depends:      deps,
			Requirements: []int{1},
			Status:       st,
		}
	}
	return &State{
		SchemaVersion: SchemaVersion,
		Spec:          "perf-sample",
		Title:         "Perf sample",
		Status:        StatusExecuting,
		Phase:         PhaseExecute,
		Gate:          GateNone,
		Tasks:         tasks,
	}
}

// TestDeterminismRenders asserts the read/render hot paths produce byte-identical
// output when called twice on unchanged state. R2.2.
func TestDeterminismRenders(t *testing.T) {
	s := stateWithTasks(50)

	t.Run("WaveGraph", func(t *testing.T) {
		if a, b := WaveGraph(s), WaveGraph(s); a != b {
			t.Errorf("WaveGraph nondeterministic:\n--- a ---\n%s\n--- b ---\n%s", a, b)
		}
	})
	t.Run("NextSummary", func(t *testing.T) {
		if a, b := NextSummary(s), NextSummary(s); a != b {
			t.Errorf("NextSummary nondeterministic: %q vs %q", a, b)
		}
	})
	t.Run("Frontier", func(t *testing.T) {
		a, b := FrontierOf(s), FrontierOf(s)
		if len(a) != len(b) {
			t.Fatalf("FrontierOf len %d vs %d", len(a), len(b))
		}
		for i := range a {
			if a[i] != b[i] {
				t.Errorf("FrontierOf[%d] = %q vs %q (order nondeterministic)", i, a[i], b[i])
			}
		}
	})
}

func BenchmarkWaveGraph(b *testing.B) {
	s := stateWithTasks(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = WaveGraph(s)
	}
}

func BenchmarkFrontierOf(b *testing.B) {
	s := stateWithTasks(200)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = FrontierOf(s)
	}
}
