package orchestration

import (
	"testing"
	"time"
)

func validMission() MissionV1 {
	return MissionV1{ProtocolVersion: MissionProtocolVersion, SessionID: "s1", MissionID: "m1", SpecSlug: "demo", TaskID: "T1", Attempt: 1, Role: "craftsman", AuthorityRef: "approval:tasks", DeclaredFiles: []string{"b.go", "a.go"}, Acceptance: []string{"R2", "R1"}, Verify: "go test ./...", ContextRef: "ctx:r1", ContextDigest: "ctx", ConfigDigest: "cfg", PaletteDigest: "pal", PolicyDigest: "pol", SubjectHead: "abc", RouteClass: "local", RouteReason: "default", Limits: MissionLimits{MaxAttempts: 2, TimeoutSeconds: 60}, IssuedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), ExpiresAt: time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC), Status: MissionPending}
}

func TestMissionCanonicalDigest(t *testing.T) {
	a, b := validMission(), validMission()
	b.DeclaredFiles[0], b.DeclaredFiles[1] = b.DeclaredFiles[1], b.DeclaredFiles[0]
	b.Acceptance[0], b.Acceptance[1] = b.Acceptance[1], b.Acceptance[0]
	if MissionDigest(a) != MissionDigest(b) {
		t.Fatal("mission digest depends on array order")
	}
}

func TestMissionValidateFailsClosed(t *testing.T) {
	m := validMission()
	if err := ValidateMission(m); err != nil {
		t.Fatal(err)
	}
	m.ProtocolVersion = "2"
	if err := ValidateMission(m); err == nil {
		t.Fatal("unknown version accepted")
	}
	m = validMission()
	m.ContextDigest = ""
	if err := ValidateMission(m); err == nil {
		t.Fatal("missing pin accepted")
	}
	m = validMission()
	m.Status = MissionActive
	if err := ValidateMission(m); err == nil {
		t.Fatal("controller minted active mission")
	}
}

func TestMissionDispatchPinsRejectDrift(t *testing.T) {
	m := validMission()
	p := DispatchPins{TaskID: m.TaskID, Role: m.Role, DeclaredFiles: append([]string(nil), m.DeclaredFiles...), Acceptance: append([]string(nil), m.Acceptance...), Verify: m.Verify, ContextDigest: m.ContextDigest, ConfigDigest: m.ConfigDigest, PaletteDigest: m.PaletteDigest, AuthorityRef: m.AuthorityRef, SubjectHead: m.SubjectHead}
	if err := ValidateMissionPins(m, p); err != nil {
		t.Fatal(err)
	}
	p.ContextDigest = "changed"
	if err := ValidateMissionPins(m, p); err == nil {
		t.Fatal("stale context pin accepted")
	}
}
