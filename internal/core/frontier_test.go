package core

import (
	"strings"
	"testing"
)

func TestFrontierDetect(t *testing.T) {
	t.Parallel()
	det := NewFrontierDetector()

	// T1 (wave 1) runnable; T2 (wave 2) depends on T1.
	s1 := mkState("demo", 1,
		TaskState{ID: "T1", Wave: 1, Status: TaskPending},
		TaskState{ID: "T2", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
	)
	ev, changed := det.Observe(s1)
	if !changed {
		t.Fatal("first observation must report changed")
	}
	if strings.Join(ev.Frontier, ",") != "T1" {
		t.Fatalf("frontier = %v, want [T1]", ev.Frontier)
	}
	if strings.Join(ev.Added, ",") != "T1" {
		t.Errorf("first event Added = %v, want [T1]", ev.Added)
	}

	// Re-observe identical state at a HIGHER revision but unchanged frontier:
	// must NOT report a change (revision advanced, frontier did not).
	s1b := mkState("demo", 5,
		TaskState{ID: "T1", Wave: 1, Status: TaskPending},
		TaskState{ID: "T2", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
	)
	if _, changed := det.Observe(s1b); changed {
		t.Error("unchanged frontier must not emit even when revision advanced")
	}

	// Complete T1 → frontier moves to T2.
	s2 := mkState("demo", 6,
		TaskState{ID: "T1", Wave: 1, Status: TaskComplete},
		TaskState{ID: "T2", Wave: 2, Depends: []string{"T1"}, Status: TaskPending},
	)
	ev, changed = det.Observe(s2)
	if !changed {
		t.Fatal("frontier change (T1 done) must emit")
	}
	if strings.Join(ev.Frontier, ",") != "T2" {
		t.Fatalf("frontier = %v, want [T2]", ev.Frontier)
	}
	if strings.Join(ev.Added, ",") != "T2" || strings.Join(ev.Removed, ",") != "T1" {
		t.Errorf("Added=%v Removed=%v, want Added=[T2] Removed=[T1]", ev.Added, ev.Removed)
	}

	// Per-spec isolation: a different spec is its own first observation.
	other := mkState("other", 1, TaskState{ID: "T1", Wave: 1, Status: TaskPending})
	if _, changed := det.Observe(other); !changed {
		t.Error("new spec must report changed on first observation")
	}
}
