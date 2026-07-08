package core

import (
	"fmt"
	"testing"
)

// genWaveTasks builds a synthetic task graph of n tasks arranged into waves of
// up to waveSize tasks each, where every task outside wave 1 depends on 1-2
// tasks from the immediately preceding wave. This mirrors the fan-in/parallel
// shape seen in real specs (several runnable tasks per wave) rather than a
// degenerate linear chain (one dep each, see syntheticDAG in dag_test.go) or a
// fully disconnected graph. Tasks are returned in ID order T1..Tn, and IDs only
// ever depend on lower-numbered IDs, so completing tasks in ID order is a valid
// topological run.
func genWaveTasks(n int) []TaskState {
	const waveSize = 5
	tasks := make([]TaskState, 0, n)
	wave := 1
	var prevWave, curWave []string
	for i := 1; i <= n; i++ {
		id := fmt.Sprintf("T%d", i)
		var deps []string
		if len(prevWave) > 0 {
			deps = append(deps, prevWave[(i-1)%len(prevWave)])
			if len(prevWave) > 1 {
				deps = append(deps, prevWave[i%len(prevWave)])
			}
		}
		tasks = append(tasks, TaskState{ID: id, Wave: wave, Depends: deps, Status: TaskPending})
		curWave = append(curWave, id)
		if len(curWave) == waveSize || i == n {
			prevWave, curWave = curWave, nil
			wave++
		}
	}
	return tasks
}

func cloneTaskStates(in []TaskState) []TaskState {
	out := make([]TaskState, len(in))
	copy(out, in)
	return out
}

func taskStateMap(tasks []TaskState) map[string]TaskState {
	m := make(map[string]TaskState, len(tasks))
	for _, t := range tasks {
		m[t.ID] = t
	}
	return m
}

// benchmarkFullRun simulates one full orchestration run over n tasks: an
// initial Observe() (first sight of the spec) followed by one Observe() per
// task completion, in dependency order — mirroring how `specd watch` polls
// state while Brain/Pinky complete tasks one at a time. This is the call
// path named in spec.md Requirement 1: every Observe() following a
// completion currently re-derives the frontier with a full O(V+E)
// RunnableFrontier rescan, for O(V*(V+E)) cumulative cost over the run.
func benchmarkFullRun(b *testing.B, n int) {
	b.Helper()
	b.ReportAllocs()
	base := genWaveTasks(n)
	for i := 0; i < b.N; i++ {
		det := NewFrontierDetector()
		tasks := cloneTaskStates(base)
		det.Observe(mkState("bench", 1, tasks...))
		for j := range tasks {
			tasks[j].Status = TaskComplete
			det.Observe(mkState("bench", j+2, tasks...))
		}
	}
}

func BenchmarkRunnableFrontier20(b *testing.B)  { benchmarkFullRun(b, 20) }
func BenchmarkRunnableFrontier100(b *testing.B) { benchmarkFullRun(b, 100) }
func BenchmarkRunnableFrontier500(b *testing.B) { benchmarkFullRun(b, 500) }
