package orchestration

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSessionPendingMissionDistinctFromLease(t *testing.T) {
	s := Session{PendingMissions: []MissionV1{validMission()}}
	if len(s.PendingMissions) != 1 || len(s.Leases) != 0 {
		t.Fatalf("pending mission created lease: %+v", s)
	}
}

func TestLeaseValidateV1(t *testing.T) {
	m := validMission()
	l, err := ClaimMission(m, WorkerV1{WorkerID: "worker-1", Host: "local", Roles: []string{m.Role}}, ClaimEcho{MissionID: m.MissionID, TaskID: m.TaskID, Role: m.Role, ContextDigest: m.ContextDigest, ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest, AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead}, m.IssuedAt, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if err := ValidateLease(l); err != nil {
		t.Fatal(err)
	}
	l.WorkerID = "brain"
	if err := ValidateLease(l); err == nil {
		t.Fatal("controller accepted as worker")
	}
}

// TestControllerApprovalHandoffPreservesSession pins R4.3: halting at an
// approval gate marks the session and nothing else. Everything the run already
// earned — its identity, step counter, leases, and missions — survives the halt
// and the resume that clears it.
func TestControllerApprovalHandoffPreservesSession(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".specd", "specs", "demo", "session.json")
	mission := validMission()
	running := Session{
		ID:              "demo",
		State:           SessionRunning,
		Step:            3,
		PendingMissions: []MissionV1{mission},
		Leases:          []Lease{{TaskID: mission.TaskID, MissionID: mission.MissionID, WorkerID: "worker-1", ExpiresAt: time.Now().Add(time.Hour)}},
	}
	if err := SaveSessionCAS(root, path, 0, running); err != nil {
		t.Fatal(err)
	}

	halted, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if halted.WaitingApproval != "" {
		t.Fatalf("a fresh session claims to be waiting on %q", halted.WaitingApproval)
	}
	halted.WaitingApproval = "tasks"
	if err := SaveSessionCAS(root, path, halted.Revision, halted); err != nil {
		t.Fatal(err)
	}

	reloaded, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.WaitingApproval != "tasks" {
		t.Fatalf("waiting gate = %q, want it persisted", reloaded.WaitingApproval)
	}
	if reloaded.ID != running.ID || reloaded.Step != running.Step ||
		len(reloaded.PendingMissions) != 1 || reloaded.PendingMissions[0].MissionID != mission.MissionID ||
		len(reloaded.Leases) != 1 || reloaded.Status() != SessionRunning {
		t.Fatalf("halt discarded progress: %+v", reloaded)
	}

	// Resuming clears the marker and leaves the same work in place.
	reloaded.WaitingApproval = ""
	if err := SaveSessionCAS(root, path, reloaded.Revision, reloaded); err != nil {
		t.Fatal(err)
	}
	resumed, err := LoadSession(path)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.WaitingApproval != "" || resumed.Step != running.Step || len(resumed.PendingMissions) != 1 || len(resumed.Leases) != 1 {
		t.Fatalf("resume did not restore a clean running session: %+v", resumed)
	}
}
