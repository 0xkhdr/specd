package core

import (
	"os"
	"testing"
)

func TestWave4RejectSecretBearingOrchestration(t *testing.T) {
	clean := []byte(`{"enabled":true,"transport":{"kind":"file"}}`)
	if err := rejectSecretBearingOrchestration(clean); err != nil {
		t.Fatalf("clean orchestration rejected: %v", err)
	}

}

func TestWave4FindSecretBearingKey(t *testing.T) {
	if err := findSecretBearingKey(map[string]any{
		"transport": map[string]any{"kind": "file"},
	}, "orchestration"); err != nil {
		t.Fatalf("clean map rejected: %v", err)
	}
	if err := findSecretBearingKey(map[string]any{
		"token": "secret",
	}, "orchestration"); err == nil {
		t.Fatal("expected token key rejection")
	}
}

func TestWave4ReconcileDropsStaleBlockers(t *testing.T) {
	state := &State{
		Status: StatusTasks,
		Tasks: map[string]TaskState{
			"T1": {Status: TaskPending},
			"T2": {Status: TaskBlocked},
		},
		Blockers: []Blocker{
			{Task: "T1", Reason: "keep"},
			{Task: "T2", Reason: "drop"},
		},
	}
	Reconcile(state, ParsedTasks{Title: "Tasks", Tasks: []ParsedTask{
		{ID: "T1", Title: "kept", Wave: 1},
		{ID: "T3", Title: "new", Wave: 2},
	}})
	if _, ok := state.Tasks["T2"]; ok {
		t.Fatal("stale task T2 was not dropped")
	}
	if _, ok := state.Tasks["T3"]; !ok {
		t.Fatal("new task T3 was not added")
	}
	if len(state.Blockers) != 1 || state.Blockers[0].Task != "T1" {
		t.Fatalf("blockers = %#v", state.Blockers)
	}
}

func TestWave4ListAndRequireSpecs(t *testing.T) {
	root := t.TempDir()
	for _, slug := range []string{"beta", "alpha"} {
		if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := SaveState(root, slug, &State{Status: StatusTasks, Tasks: map[string]TaskState{}}); err != nil {
			t.Fatal(err)
		}
	}
	got := ListSpecs(root)
	if len(got) != 2 || got[0] != "alpha" || got[1] != "beta" {
		t.Fatalf("ListSpecs = %#v", got)
	}
	if err := RequireSpec(root, "alpha"); err != nil {
		t.Fatalf("RequireSpec(alpha): %v", err)
	}
	if err := RequireSpec(root, "missing"); err == nil {
		t.Fatal("expected missing spec error")
	}
}
