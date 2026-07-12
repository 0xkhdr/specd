package cmd

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/orchestration"
)

func TestBrainCancelRevokesLeaseAndRecordsEvent(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", brainEnabledConfig)
	sessionPath := brainSessionPath(root)
	lease := orchestration.Lease{LeaseID: "lease-1", MissionID: "m1", TaskID: "T1", Attempt: 1, WorkerID: "w1", State: orchestration.LeaseActive, ExpiresAt: time.Now().Add(time.Hour)}
	if err := orchestration.SaveSessionCAS(root, sessionPath, 0, orchestration.Session{ID: "demo", Leases: []orchestration.Lease{lease}}); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"cancel", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	s := loadBrainSession(t, root)
	if len(s.Leases) != 1 || s.Leases[0].State != orchestration.LeaseRevoked {
		t.Fatalf("revoked lease not retained: %+v", s.Leases)
	}
	events, err := orchestration.ReadACP(filepath.Join(root, ".specd", "specs", "demo", "acp.jsonl"))
	if err != nil || len(events) != 1 || events[0].Kind != orchestration.ACPKindCancel {
		t.Fatalf("cancel event missing: events=%+v err=%v", events, err)
	}
}
