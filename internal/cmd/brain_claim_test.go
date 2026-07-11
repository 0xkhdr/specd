package cmd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBrainClaimAndHeartbeat(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	if err := os.WriteFile(filepath.Join(root, ".specd/specs/demo/tasks.md"), []byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"step", "demo"}, map[string]string{"authority": ""}); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"claim", "demo", "demo.s1.T1", "worker-1", "craftsman"}, nil); err != nil {
		t.Fatal(err)
	}
	s := loadBrainSession(t, root)
	if len(s.Leases) != 1 || len(s.PendingMissions) != 0 {
		t.Fatalf("session=%+v", s)
	}
	if err := runBrain(root, []string{"heartbeat", "demo", s.Leases[0].LeaseID, "worker-1"}, nil); err != nil {
		t.Fatal(err)
	}
}

func TestBrainClaimRejectsWrongRole(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	os.WriteFile(filepath.Join(root, ".specd/specs/demo/tasks.md"), []byte("| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n"), 0o644)
	runBrain(root, []string{"start", "demo"}, nil)
	runBrain(root, []string{"step", "demo"}, map[string]string{"authority": ""})
	if err := runBrain(root, []string{"claim", "demo", "demo.s1.T1", "worker-1", "scout"}, nil); err == nil {
		t.Fatal("wrong role accepted")
	}
}
