package core

import (
	"testing"
	"time"
)

// chainTasks builds n tasks where task i depends on task i-1 — the worst case
// for wave projection and a realistic linear frontier scan.
func chainTasks(n int) []TaskRow {
	tasks := make([]TaskRow, n)
	for i := 0; i < n; i++ {
		id := "T" + itoaBench(i)
		var deps []string
		if i > 0 {
			deps = []string{"T" + itoaBench(i-1)}
		}
		tasks[i] = TaskRow{ID: id, Marker: "⬜", Role: "craftsman", DependsOn: deps, Verify: "printf ok"}
	}
	return tasks
}

func itoaBench(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func BenchmarkFrontier(b *testing.B) {
	for _, n := range []int{100, 500, 2000} {
		tasks := chainTasks(n)
		b.Run("n="+itoaBench(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := Frontier(tasks, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

func BenchmarkProjectWaves(b *testing.B) {
	for _, n := range []int{100, 500, 2000} {
		tasks := chainTasks(n)
		b.Run("n="+itoaBench(n), func(b *testing.B) {
			b.ReportAllocs()
			for i := 0; i < b.N; i++ {
				if _, err := ProjectWaves(tasks); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// TestFrontierScalesSubQuadratically pins the perf invariant (SPEC-03 T-03-02):
// the runnable-frontier recompute must not blow up quadratically with task
// count. Quadrupling the input should quadruple work for the linear frontier
// scan (~4x), never square it (~16x). The threshold of 9x rejects quadratic
// growth while tolerating the wall-clock noise of a shared CI runner.
func TestFrontierScalesSubQuadratically(t *testing.T) {
	if testing.Short() {
		t.Skip("timing-based scaling check skipped in -short")
	}
	bench := func(n int) time.Duration {
		tasks := chainTasks(n)
		r := testing.Benchmark(func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				if _, err := Frontier(tasks, nil); err != nil {
					b.Fatal(err)
				}
			}
		})
		return time.Duration(r.NsPerOp())
	}
	// Best-of-two per size damps shared-runner and coverage-instrumentation
	// noise: a scheduling stall inflates one sample, rarely both.
	best := func(n int) time.Duration {
		a, b := bench(n), bench(n)
		if b < a {
			return b
		}
		return a
	}
	small := best(500)
	large := best(2000) // 4x the tasks
	if small <= 0 {
		t.Skip("benchmark produced non-positive baseline; timing unavailable")
	}
	ratio := float64(large) / float64(small)
	if ratio > 9 {
		t.Fatalf("frontier scaling looks quadratic: 4x tasks => %.1fx time (small=%v large=%v)", ratio, small, large)
	}
	t.Logf("frontier 4x-input time ratio = %.2fx (small=%v large=%v)", ratio, small, large)
}
