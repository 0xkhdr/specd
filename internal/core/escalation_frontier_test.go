package core

import "testing"

func TestFrontierEscalated(t *testing.T) {
	tasks := []TaskRow{
		{ID: "T1", Role: "craftsman", Verify: "go test", Files: "a.go"},
		{ID: "T2", Role: "craftsman", Verify: "go test", Files: "b.go", DependsOn: []string{}},
	}
	status := map[string]TaskRunStatus{}

	t.Run("escalated_task_excluded_from_frontier", func(t *testing.T) {
		front, err := FrontierExcluding(tasks, status, map[string]bool{"T1": true})
		if err != nil {
			t.Fatal(err)
		}
		for _, f := range front {
			if f.ID == "T1" {
				t.Fatalf("escalated task T1 present in frontier: %+v", front)
			}
		}
		if len(front) != 1 || front[0].ID != "T2" {
			t.Fatalf("expected only T2 runnable, got %+v", front)
		}
	})

	t.Run("nil_set_is_plain_frontier", func(t *testing.T) {
		a, _ := Frontier(tasks, status)
		b, _ := FrontierExcluding(tasks, status, nil)
		if len(a) != len(b) {
			t.Fatalf("nil exclusion diverged from Frontier: %d vs %d", len(a), len(b))
		}
	})
}

func TestVerifyEscalated(t *testing.T) {
	// N consecutive failing verifies escalate; an override resets the counter but
	// must NOT manufacture passing evidence — completion still requires a real
	// exit-0 record pinned to a commit (the no-bypass invariant).
	evidence := []EvidenceRecord{
		ev("T1", 1, "2026-01-01T00:00:01Z"),
		ev("T1", 1, "2026-01-01T00:00:02Z"),
		ev("T1", 1, "2026-01-01T00:00:03Z"),
	}
	if !IsEscalated(ConsecutiveVerifyFails(evidence, nil, "T1"), 3) {
		t.Fatal("expected T1 escalated after 3 fails")
	}

	overrides := []OverrideRecord{{TaskID: "T1", Reason: "manual", Timestamp: "2026-01-01T00:00:04Z"}}
	if IsEscalated(ConsecutiveVerifyFails(evidence, overrides, "T1"), 3) {
		t.Fatal("override should clear escalation")
	}

	// Even cleared, there is still no passing evidence, so CompleteTask refuses.
	records := map[string]EvidenceRecord{"T1": ev("T1", 1, "2026-01-01T00:00:03Z")}
	if _, err := CompleteTask([]byte("| T1 | craftsman | a.go | | go test | done |\n"), "T1", records); err == nil {
		t.Fatal("override must not bypass the passing-evidence requirement")
	}
}
