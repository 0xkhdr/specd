package core

import (
	"strconv"
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

func TestCriticalPathCases(t *testing.T) {
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
