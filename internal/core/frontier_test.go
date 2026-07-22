package core

import (
	"slices"
	"testing"
)

// TestTaskActivityReadiness pins the spec 03 R3 contract: activity and readiness
// are separate, every applicable wait is reported in stable priority order, only
// pending-and-ready runs, and pending blocks parent completion.
func TestTaskActivityReadiness(t *testing.T) {
	tasks := []TaskRow{
		{ID: "T1"},
		{ID: "T2", DependsOn: []string{"T1"}},
		{ID: "T3"},
	}
	state := func(t *testing.T, status map[string]TaskRunStatus, facts map[string]TaskFacts, id string) TaskState {
		t.Helper()
		states, err := ProjectTaskStates(tasks, status, facts)
		if err != nil {
			t.Fatalf("ProjectTaskStates: %v", err)
		}
		for _, s := range states {
			if s.ID == id {
				return s
			}
		}
		t.Fatalf("task %s missing from projection %#v", id, states)
		return TaskState{}
	}

	t.Run("CancelledDependency", func(t *testing.T) {
		facts := map[string]TaskFacts{"T1": {Activity: ActivityCancelled}}
		got := state(t, nil, facts, "T2")
		if got.Readiness != ReadinessWaitingDependency || len(got.Waits) != 1 || got.Waits[0].Code != WaitDependencyTerminal {
			t.Fatalf("cancelled dependency = %#v, want unresolved dependency wait", got)
		}
		if got.Waits[0].Owner == "" || got.Waits[0].Recovery == "" {
			t.Fatalf("wait reason missing owner/recovery: %#v", got.Waits[0])
		}
		facts["T1"] = TaskFacts{Activity: ActivityCancelled, CoverageResolved: true}
		if got := state(t, nil, facts, "T2"); !got.Runnable() {
			t.Fatalf("dispositioned cancelled dependency = %#v, want runnable", got)
		}
	})

	t.Run("MultipleWaits", func(t *testing.T) {
		facts := map[string]TaskFacts{"T2": {Waits: []WaitReason{
			{Code: "schedule_window", Readiness: ReadinessWaitingSchedule, Review: "2026-08-01"},
			{Code: "question_open", Readiness: ReadinessWaitingClarification, Refs: []string{"C1"}},
			{Code: "plan_approval", Readiness: ReadinessWaitingApproval, Refs: []string{"AR1"}},
		}}}
		got := state(t, nil, facts, "T2")
		want := []string{WaitDependencyIncomplete, "plan_approval", "question_open", "schedule_window"}
		codes := make([]string, 0, len(got.Waits))
		for _, wait := range got.Waits {
			codes = append(codes, wait.Code)
		}
		if !slices.Equal(codes, want) {
			t.Fatalf("wait codes = %v, want %v", codes, want)
		}
		if got.Readiness != ReadinessWaitingDependency {
			t.Fatalf("readiness = %q, want highest-priority cause", got.Readiness)
		}
	})

	t.Run("CompletedStale", func(t *testing.T) {
		status := map[string]TaskRunStatus{"T1": TaskComplete, "T2": TaskComplete}
		facts := map[string]TaskFacts{"T1": {Activity: ActivityCancelled}}
		got := state(t, status, facts, "T2")
		if got.Activity != ActivityCompleted {
			t.Fatalf("activity = %q, want completed to stay completed", got.Activity)
		}
		if got.Readiness != ReadinessWaitingDependency || got.Runnable() {
			t.Fatalf("stale completed task = %#v, want reported wait but not runnable", got)
		}
	})

	t.Run("ParentCompletion", func(t *testing.T) {
		status := map[string]TaskRunStatus{"T1": TaskComplete}
		facts := map[string]TaskFacts{"T3": {Waits: []WaitReason{{Code: "plan_approval", Readiness: ReadinessWaitingApproval}}}}
		states, err := ProjectTaskStates(tasks, status, facts)
		if err != nil {
			t.Fatalf("ProjectTaskStates: %v", err)
		}
		if got := PendingCompletionBlockers(states); !slices.Equal(got, []string{"T2", "T3"}) {
			t.Fatalf("blockers = %v, want both pending tasks including the waiting one", got)
		}
		frontier, err := Frontier(tasks, nil)
		if err != nil {
			t.Fatalf("Frontier: %v", err)
		}
		ids := make([]string, 0, len(frontier))
		for _, task := range frontier {
			ids = append(ids, task.ID)
		}
		if !slices.Equal(ids, []string{"T1", "T3"}) {
			t.Fatalf("frontier = %v, want only pending-ready tasks", ids)
		}
		disposed := map[string]TaskFacts{"T2": {Activity: ActivityCancelled}, "T3": {Activity: ActivitySuperseded}}
		states, err = ProjectTaskStates(tasks, status, disposed)
		if err != nil {
			t.Fatalf("ProjectTaskStates: %v", err)
		}
		if got := PendingCompletionBlockers(states); len(got) != 0 {
			t.Fatalf("blockers after terminal disposition = %v, want none", got)
		}
	})
}

func TestFrontierAndWaves(t *testing.T) {
	tasks := []TaskRow{
		{ID: "T1", Role: "craftsman", Verify: "true"},
		{ID: "T2", Role: "craftsman", DependsOn: []string{"T1"}, Verify: "true"},
		{ID: "T3", Role: "craftsman", DependsOn: []string{"T1"}, Verify: "true"},
	}
	frontier, err := Frontier(tasks, map[string]TaskRunStatus{})
	if err != nil {
		t.Fatalf("Frontier: %v", err)
	}
	if len(frontier) != 1 || frontier[0].ID != "T1" {
		t.Fatalf("frontier = %#v, want T1", frontier)
	}
	waves, err := ProjectWaves(tasks)
	if err != nil {
		t.Fatalf("ProjectWaves: %v", err)
	}
	if len(waves) != 2 || len(waves[1].Tasks) != 2 {
		t.Fatalf("waves = %#v, want two waves with T2/T3 in second", waves)
	}
}
