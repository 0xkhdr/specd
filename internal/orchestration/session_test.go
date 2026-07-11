package orchestration

import (
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
