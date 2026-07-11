package orchestration

import (
	"strings"
	"testing"
	"time"
)

func TestWorkerClaimValidatesCapabilityAndPins(t *testing.T) {
	m := validMission()
	w := WorkerV1{WorkerID: "worker-1", Host: "local", Roles: []string{"craftsman"}, Capabilities: []string{"edit", "verify"}}
	l, err := ClaimMission(m, w, ClaimEcho{MissionID: m.MissionID, TaskID: m.TaskID, Role: m.Role, ContextDigest: m.ContextDigest, ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest, AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead}, m.IssuedAt, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if l.WorkerID != w.WorkerID || l.State != LeaseActive || l.LeaseID == "" {
		t.Fatalf("lease = %+v", l)
	}
	w.Roles = []string{"scout"}
	if _, err := ClaimMission(m, w, ClaimEcho{MissionID: m.MissionID}, m.IssuedAt, time.Minute); err == nil || !strings.Contains(err.Error(), "ROLE") {
		t.Fatalf("role mismatch err = %v", err)
	}
}

func TestWorkerClaimConflict(t *testing.T) {
	m := validMission()
	active := Lease{LeaseID: "l", MissionID: m.MissionID, TaskID: m.TaskID, Attempt: 1, WorkerID: "other", IssuedAt: m.IssuedAt, ExpiresAt: m.ExpiresAt, PolicyDigest: m.PolicyDigest, State: LeaseActive}
	if err := CheckClaimConflict([]Lease{active}, m, m.IssuedAt); err == nil {
		t.Fatal("conflicting live claim accepted")
	}
}
