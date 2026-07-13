package adapter

import (
	"reflect"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/orchestration"
)

func TestA2AMissionRoundTripPreservesAuthorityScopeEvidence(t *testing.T) {
	mission := orchestration.MissionV1{
		ProtocolVersion: orchestration.MissionProtocolVersion,
		SessionID:       "session-1", MissionID: "mission-1", SpecSlug: "demo", TaskID: "T13",
		Attempt: 1, Role: "craftsman", AuthorityRef: "authority:sha256", DeclaredFiles: []string{"b.go", "a.go"},
		Acceptance: []string{"R10.1"}, Verify: "go test ./...", ContextRef: "context.json", ContextDigest: "ctx",
		ConfigDigest: "config", PaletteDigest: "palette", PolicyDigest: "policy", SubjectHead: "0123456789abcdef",
		RouteClass: "local", RouteReason: "test", Limits: orchestration.MissionLimits{MaxAttempts: 1, TimeoutSeconds: 30},
		IssuedAt: time.Date(2026, 7, 13, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 7, 13, 0, 0, 30, 0, time.UTC), Status: orchestration.MissionPending,
	}
	req, err := MissionRequest(mission, "request-1", "correlation-1", "a2a")
	if err != nil {
		t.Fatal(err)
	}
	got, err := MissionFromRequest(req)
	if err != nil {
		t.Fatal(err)
	}
	orchestration.CanonicalizeMission(&mission)
	if !reflect.DeepEqual(got, mission) {
		t.Fatalf("mission changed:\nwant %+v\n got %+v", mission, got)
	}
	if req.AuthorityRef != mission.AuthorityRef || req.Subject.SpecSlug != mission.SpecSlug || req.Subject.TaskID != mission.TaskID || req.Subject.GitHead != mission.SubjectHead {
		t.Fatalf("envelope pins lost: %+v", req)
	}
}
