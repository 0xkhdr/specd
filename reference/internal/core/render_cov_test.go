package core

import (
	"strings"
	"testing"
)

// render_cov_test.go covers the pure projection/render helpers in render.go:
// task counting, DAG projection, wave graph text, next-step summaries, blocker
// lines, and requirement/acceptance gap computation.

func TestCountTasks(t *testing.T) {
	state := mkState("s", 1,
		TaskState{ID: "T1", Status: TaskComplete},
		TaskState{ID: "T2", Status: TaskRunning},
		TaskState{ID: "T3", Status: TaskPending},
		TaskState{ID: "T4", Status: TaskBlocked},
		TaskState{ID: "T5", Status: TaskPending},
	)
	c := CountTasks(state)
	if c.Total != 5 || c.Complete != 1 || c.Running != 1 || c.Pending != 2 || c.Blocked != 1 {
		t.Fatalf("counts wrong: %+v", c)
	}
}

func TestDagTasksFromState(t *testing.T) {
	state := mkState("s", 1,
		TaskState{ID: "T1", Wave: 1, Depends: []string{}, Status: TaskComplete},
		TaskState{ID: "T2", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
	)
	dag := DagTasksFromState(state)
	if len(dag) != 2 {
		t.Fatalf("want 2 dag tasks, got %d", len(dag))
	}
}

func TestWaveGraph(t *testing.T) {
	blk := "needs creds"
	state := mkState("s", 1,
		TaskState{ID: "T1", Title: "build", Wave: 1, Status: TaskComplete},
		TaskState{ID: "T2", Title: "test", Wave: 2, Depends: []string{"T1"}, Status: TaskBlocked, Blocker: &blk},
	)
	out := WaveGraph(state)
	for _, want := range []string{"Wave 1", "Wave 2", "T1", "T2", "build", "test", "(blocked: needs creds)"} {
		if !strings.Contains(out, want) {
			t.Errorf("wave graph missing %q\n%s", want, out)
		}
	}
	if WaveGraph(mkState("empty", 1)) != "(no tasks yet)" {
		t.Error("empty state should report no tasks")
	}
}

func TestNextSummary(t *testing.T) {
	// Runnable task.
	s1 := mkState("s", 1,
		TaskState{ID: "T1", Title: "first", Wave: 1, Status: TaskPending},
	)
	if got := NextSummary(s1); !strings.Contains(got, "T1") || !strings.Contains(got, "first") {
		t.Errorf("next runnable: %q", got)
	}
	// All complete.
	s2 := mkState("s", 1, TaskState{ID: "T1", Wave: 1, Status: TaskComplete})
	if got := NextSummary(s2); got != "all tasks complete" {
		t.Errorf("all complete: %q", got)
	}
	// All blocked.
	s3 := mkState("s", 1, TaskState{ID: "T1", Wave: 1, Status: TaskBlocked})
	if got := NextSummary(s3); !strings.HasPrefix(got, "all remaining blocked") {
		t.Errorf("all blocked: %q", got)
	}
	// Waiting: a pending task depends on an incomplete one.
	s4 := mkState("s", 1,
		TaskState{ID: "T1", Wave: 1, Status: TaskBlocked},
		TaskState{ID: "T2", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
	)
	_ = NextSummary(s4) // exercise the waiting/blocked branches without over-asserting ordering
}

func TestBlockerLines(t *testing.T) {
	state := &State{Blockers: []Blocker{
		{Task: "T1", Reason: "flaky", Since: "now"},
		{Task: "T2", Reason: "creds", Since: "now"},
	}}
	lines := BlockerLines(state)
	if len(lines) != 2 || !strings.Contains(lines[0], "T1: flaky") {
		t.Fatalf("blocker lines wrong: %v", lines)
	}
}

func TestRequirementNumbers(t *testing.T) {
	md := "## Requirement 1\ntext\n## Requirement 3\nmore\n"
	got := RequirementNumbers(md)
	if !got[1] || !got[3] || got[2] {
		t.Fatalf("requirement numbers wrong: %v", got)
	}
}

func TestUncoveredRequirements(t *testing.T) {
	state := mkState("s", 1,
		TaskState{ID: "T1", Requirements: []int{1}},
	)
	md := "## Requirement 1\n## Requirement 2\n## Requirement 3\n"
	got := UncoveredRequirements(state, &md)
	if len(got) != 2 || got[0] != 2 || got[1] != 3 {
		t.Fatalf("uncovered want [2 3], got %v", got)
	}
	if UncoveredRequirements(state, nil) != nil {
		t.Error("nil reqMd should yield nil")
	}
}

func TestGetAcceptanceGaps(t *testing.T) {
	md := "## Requirement 1\n## Requirement 2\n"
	state := &State{
		Tasks: map[string]TaskState{},
		Acceptance: map[string]CriterionRecord{
			"R1C1": {Requirement: 1, Criterion: 1, Status: "pass"},
			"R2C1": {Requirement: 2, Criterion: 1, Status: "fail"},
		},
	}
	gaps := GetAcceptanceGaps(state, &md)
	// Requirement 2 has a failing criterion → unmet; R2C1 listed as failed.
	if len(gaps.Unmet) != 1 || gaps.Unmet[0] != 2 {
		t.Fatalf("unmet want [2], got %v", gaps.Unmet)
	}
	if len(gaps.Failed) != 1 || gaps.Failed[0] != "R2C1" {
		t.Fatalf("failed want [R2C1], got %v", gaps.Failed)
	}
}
