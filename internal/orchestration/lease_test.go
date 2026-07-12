package orchestration

import (
	"strings"
	"testing"
	"time"
)

func TestDispatchEnvelopeStaleClaimAndReportRejected(t *testing.T) {
	m := validMission()
	if _, err := PinDispatchEnvelope("/repo", &m); err != nil {
		t.Fatal(err)
	}
	now := m.IssuedAt.Add(time.Minute)
	echo := ClaimEcho{MissionID: m.MissionID, TaskID: m.TaskID, Role: m.Role, ContextDigest: m.ContextDigest, ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest, AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead, DispatchDigest: "changed"}
	if _, err := ClaimMission(m, WorkerV1{WorkerID: "w1", Host: "host", Roles: []string{m.Role}}, echo, now, time.Minute); err == nil || !strings.Contains(err.Error(), "PIN") {
		t.Fatalf("stale claim accepted: %v", err)
	}
	echo.DispatchDigest = m.DispatchDigest
	lease, err := ClaimMission(m, WorkerV1{WorkerID: "w1", Host: "host", Roles: []string{m.Role}}, echo, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	report := WorkerReportV1{MissionID: m.MissionID, LeaseID: lease.LeaseID, WorkerID: lease.WorkerID, TaskID: m.TaskID, Attempt: m.Attempt, Role: m.Role, SubjectHead: m.SubjectHead, VerifyRef: "verify:1", Status: "complete", DispatchDigest: "changed"}
	if err := ValidateWorkerReport(report, m, lease, now.Add(time.Second)); err == nil || !strings.Contains(err.Error(), "IDENTITY") {
		t.Fatalf("stale report accepted: %v", err)
	}
}

func TestDispatchEnvelopeDoesNotAuthorizeCompletion(t *testing.T) {
	m := validMission()
	if _, err := PinDispatchEnvelope("/repo", &m); err != nil {
		t.Fatal(err)
	}
	if m.Status != MissionPending {
		t.Fatalf("envelope changed mission status: %s", m.Status)
	}
}
