package integration

import (
	"reflect"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/orchestration"
)

func TestOrchestrationConformance(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	m := orchestration.MissionV1{ProtocolVersion: orchestration.MissionProtocolVersion, SessionID: "s", MissionID: "s.s1.T1", SpecSlug: "demo", TaskID: "T1", Attempt: 1, Role: "craftsman", AuthorityRef: "approval:tasks", DeclaredFiles: []string{"a.go"}, Acceptance: []string{"R1"}, Verify: "printf ok", ContextRef: "context:T1", ContextDigest: "sha256:c", ConfigDigest: "sha256:g", PaletteDigest: "sha256:p", PolicyDigest: "sha256:y", SubjectHead: "0123456789012345678901234567890123456789", RouteClass: "local", RouteReason: "fixture", Limits: orchestration.MissionLimits{MaxAttempts: 2, TimeoutSeconds: 60}, IssuedAt: now.Add(-time.Second), ExpiresAt: now.Add(time.Hour), Status: orchestration.MissionPending}
	w := orchestration.WorkerV1{WorkerID: "worker-1", Host: "fake", Roles: []string{"craftsman"}}
	e := orchestration.ClaimEcho{MissionID: m.MissionID, TaskID: m.TaskID, Role: m.Role, ContextDigest: m.ContextDigest, ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest, AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead}
	l, err := orchestration.ClaimMission(m, w, e, now, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	l, err = orchestration.RenewLease(l, orchestration.HeartbeatV1{LeaseID: l.LeaseID, MissionID: m.MissionID, WorkerID: w.WorkerID, Attempt: 1, At: now.Add(10 * time.Second)}, time.Minute, 10*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	r := orchestration.WorkerReportV1{MissionID: m.MissionID, LeaseID: l.LeaseID, WorkerID: w.WorkerID, TaskID: m.TaskID, Attempt: 1, Role: m.Role, SubjectHead: m.SubjectHead, VerifyRef: "evidence#T1", Status: "complete"}
	if err := orchestration.ValidateWorkerReport(r, m, l, now.Add(20*time.Second)); err != nil {
		t.Fatal(err)
	}
	revoked, _, err := orchestration.CancelLease(l, "operator", now.Add(30*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	if err := orchestration.ValidateWorkerReport(r, m, revoked, now.Add(40*time.Second)); err == nil {
		t.Fatal("revoked lease accepted stale report")
	}
}

func TestOrchestrationConformanceA2AParity(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	m := orchestration.MissionV1{ProtocolVersion: orchestration.MissionProtocolVersion, SessionID: "s", MissionID: "s.s1.T1", SpecSlug: "demo", TaskID: "T1", Attempt: 1, Role: "craftsman", AuthorityRef: "approval:tasks", DeclaredFiles: []string{"a.go"}, Acceptance: []string{"R6"}, Verify: "printf ok", ContextRef: "context:T1", ContextDigest: "sha256:c", ConfigDigest: "sha256:g", PaletteDigest: "sha256:p", PolicyDigest: "sha256:y", SubjectHead: "0123456789012345678901234567890123456789", RouteClass: "local", RouteReason: "fixture", Limits: orchestration.MissionLimits{MaxAttempts: 2, TimeoutSeconds: 60}, IssuedAt: now, ExpiresAt: now.Add(time.Hour), Status: orchestration.MissionPending}
	raw, err := orchestration.ExportA2A(orchestration.A2AKindMission, m, orchestration.A2ATransport{Adapter: "fake-a2a", MessageID: "delivery-only"})
	if err != nil {
		t.Fatal(err)
	}
	message, err := orchestration.ImportA2A(raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := orchestration.A2ASemanticACP(message)
	if err != nil {
		t.Fatal(err)
	}
	want, err := orchestration.SemanticACP(orchestration.A2AKindMission, m)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("local/A2A semantic streams differ:\ngot  %+v\nwant %+v", got, want)
	}
}
