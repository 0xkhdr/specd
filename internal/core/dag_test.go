package core

import (
	"testing"
)

func TestOrphanDeps(t *testing.T) {
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
	tasks := []DagTask{{ID: "T1", Wave: 1, Status: TaskComplete}}
	r := NextRunnable(tasks)
	if r.Kind != NextAllComplete {
		t.Errorf("expected all-complete, got %s", r.Kind)
	}
}

func TestCriticalPath(t *testing.T) {
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
