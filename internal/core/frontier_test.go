package core

import "testing"

func TestFrontierAndWaves(t *testing.T) {
	tasks := []TaskRow{
		{ID: "T1", Role: "builder", Verify: "true"},
		{ID: "T2", Role: "builder", DependsOn: []string{"T1"}, Verify: "true"},
		{ID: "T3", Role: "builder", DependsOn: []string{"T1"}, Verify: "true"},
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
