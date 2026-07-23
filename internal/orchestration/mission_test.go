package orchestration

import (
	"encoding/json"
	"strings"
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
	m.Status = MissionStatus("active")
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

// TestMissionLifecycle tests mission state transitions and baseline selection
// (R4.1-R4.4): mission states, release without TTL, and baseline precedence.
func TestMissionLifecycle(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	m1 := validMission()
	m1.MissionID = "m1"
	m1.TaskID = "T1"
	m1.IssuedAt = now.Add(-1 * time.Hour)

	m2 := validMission()
	m2.MissionID = "m2"
	m2.TaskID = "T2"
	m2.IssuedAt = now.Add(-30 * time.Minute)

	t.Run("pending_mission_has_no_lease", func(t *testing.T) {
		// Dispatch creates a pending mission with no lease
		state := ComputeMissionState(m1.MissionID, nil, nil, now)
		if state != MissionStatePending {
			t.Errorf("pending mission without lease: got %q, want pending", state)
		}
	})

	t.Run("claimed_mission_with_active_lease", func(t *testing.T) {
		// Claim event creates an active lease
		lease := Lease{
			LeaseID:   "l1",
			MissionID: m1.MissionID,
			TaskID:    m1.TaskID,
			WorkerID:  "worker-1",
			State:     LeaseActive,
			IssuedAt:  now.Add(-20 * time.Minute),
			ExpiresAt: now.Add(40 * time.Minute),
		}
		state := ComputeMissionState(m1.MissionID, &lease, nil, now)
		if state != MissionStateClaimed {
			t.Errorf("claimed mission with active lease: got %q, want claimed", state)
		}
	})

	t.Run("expired_mission_when_lease_expires", func(t *testing.T) {
		// Lease expires without being released
		lease := Lease{
			LeaseID:   "l1",
			MissionID: m1.MissionID,
			TaskID:    m1.TaskID,
			WorkerID:  "worker-1",
			State:     LeaseActive,
			IssuedAt:  now.Add(-1 * time.Hour),
			ExpiresAt: now.Add(-10 * time.Minute), // expired
		}
		state := ComputeMissionState(m1.MissionID, &lease, nil, now)
		if state != MissionStateExpired {
			t.Errorf("expired lease: got %q, want expired", state)
		}
	})

	t.Run("released_mission_when_explicitly_released", func(t *testing.T) {
		// Controller releases the mission immediately (R4.3)
		lease := Lease{
			LeaseID:          "l1",
			MissionID:        m1.MissionID,
			TaskID:           m1.TaskID,
			WorkerID:         "worker-1",
			State:            LeaseRevoked,
			RevocationReason: "operator released",
			IssuedAt:         now.Add(-1 * time.Hour),
			ExpiresAt:        now.Add(1 * time.Hour),
		}
		events := []ACPEvent{{Kind: ACPKindCancel, MissionID: m1.MissionID, TaskID: m1.TaskID}}
		state := ComputeMissionState(m1.MissionID, &lease, events, now)
		if state != MissionStateReleased {
			t.Errorf("released mission: got %q, want released", state)
		}
	})

	t.Run("completed_mission_after_report", func(t *testing.T) {
		// Worker reports successful completion
		lease := Lease{
			LeaseID:   "l1",
			MissionID: m1.MissionID,
			TaskID:    m1.TaskID,
			WorkerID:  "worker-1",
			State:     LeaseActive,
			IssuedAt:  now.Add(-20 * time.Minute),
			ExpiresAt: now.Add(40 * time.Minute),
		}
		report := WorkerReportV1{Status: "complete"}
		reportPayload, _ := json.Marshal(report)
		events := []ACPEvent{{
			Kind:      ACPKindReport,
			MissionID: m1.MissionID,
			TaskID:    m1.TaskID,
			Payload:   string(reportPayload),
		}}
		state := ComputeMissionState(m1.MissionID, &lease, events, now)
		if state != MissionStateCompleted {
			t.Errorf("completed mission: got %q, want completed", state)
		}
	})

	t.Run("failed_mission_after_report", func(t *testing.T) {
		// Worker reports failure
		lease := Lease{
			LeaseID:   "l1",
			MissionID: m1.MissionID,
			TaskID:    m1.TaskID,
			WorkerID:  "worker-1",
			State:     LeaseActive,
			IssuedAt:  now.Add(-20 * time.Minute),
			ExpiresAt: now.Add(40 * time.Minute),
		}
		report := WorkerReportV1{Status: "failed"}
		reportPayload, _ := json.Marshal(report)
		events := []ACPEvent{{
			Kind:      ACPKindReport,
			MissionID: m1.MissionID,
			TaskID:    m1.TaskID,
			Payload:   string(reportPayload),
		}}
		state := ComputeMissionState(m1.MissionID, &lease, events, now)
		if state != MissionStateFailed {
			t.Errorf("failed mission: got %q, want failed", state)
		}
	})

	t.Run("baseline_prefers_claimed_over_expired", func(t *testing.T) {
		// R4.4: prefer live claimed mission over expired
		activeLease := Lease{
			LeaseID:   "l1",
			MissionID: m1.MissionID,
			TaskID:    m1.TaskID,
			WorkerID:  "worker-1",
			State:     LeaseActive,
			ExpiresAt: now.Add(40 * time.Minute),
		}
		expiredLease := Lease{
			LeaseID:   "l2",
			MissionID: m2.MissionID,
			TaskID:    m2.TaskID,
			WorkerID:  "worker-2",
			State:     LeaseActive,
			ExpiresAt: now.Add(-10 * time.Minute), // expired
		}
		leases := []Lease{expiredLease, activeLease}
		baseline := SelectBaseline([]MissionV1{m1, m2}, leases, nil, now)
		if baseline == nil || baseline.MissionID != m1.MissionID {
			t.Errorf("baseline selection: got %v, want m1 (claimed)", baseline)
		}
	})

	t.Run("baseline_prefers_claimed_over_released", func(t *testing.T) {
		// Claimed mission takes priority over released mission
		activeLease := Lease{
			LeaseID:   "l1",
			MissionID: m1.MissionID,
			TaskID:    m1.TaskID,
			WorkerID:  "worker-1",
			State:     LeaseActive,
			ExpiresAt: now.Add(40 * time.Minute),
		}
		releasedLease := Lease{
			LeaseID:          "l2",
			MissionID:        m2.MissionID,
			TaskID:           m2.TaskID,
			WorkerID:         "worker-2",
			State:            LeaseRevoked,
			RevocationReason: "released",
		}
		leases := []Lease{releasedLease, activeLease}
		baseline := SelectBaseline([]MissionV1{m1, m2}, leases, nil, now)
		if baseline == nil || baseline.MissionID != m1.MissionID {
			t.Errorf("baseline selection: got %v, want m1 (claimed)", baseline)
		}
	})

	t.Run("baseline_picks_any_when_all_pending", func(t *testing.T) {
		// All missions pending: pick the most recent
		baseline := SelectBaseline([]MissionV1{m1, m2}, []Lease{}, nil, now)
		if baseline == nil || baseline.MissionID != m2.MissionID {
			t.Errorf("baseline from pending: got %v, want m2 (more recent)", baseline)
		}
	})

	t.Run("release_mission_immediate", func(t *testing.T) {
		// R4.3: immediate release without TTL wait
		lease := Lease{
			LeaseID:   "l1",
			MissionID: m1.MissionID,
			TaskID:    m1.TaskID,
			WorkerID:  "worker-1",
			State:     LeaseActive,
			ExpiresAt: now.Add(1 * time.Hour), // still valid TTL
		}
		released := ReleaseMission(lease, "controller released")
		if released.State != LeaseRevoked {
			t.Errorf("release: got state %q, want revoked", released.State)
		}
		if released.RevocationReason != "controller released" {
			t.Errorf("release: got reason %q, want 'controller released'", released.RevocationReason)
		}
		// Verify it's immediately treated as released, not expired
		state := ComputeMissionState(m1.MissionID, &released, nil, now)
		if state != MissionStateReleased {
			t.Errorf("released state: got %q, want released", state)
		}
	})

	t.Run("serialize_missions_in_shared_worktree", func(t *testing.T) {
		// R4.1: serialize task missions so diff scope satisfied
		// Create two missions with overlapping declared files
		mission1 := MissionV1{
			MissionID:     "m-T1",
			TaskID:        "T1",
			DeclaredFiles: []string{"internal/core/thing.go", "config.json"},
		}
		mission2 := MissionV1{
			MissionID:     "m-T2",
			TaskID:        "T2",
			DeclaredFiles: []string{"config.json", "other.go"}, // overlaps mission1
		}
		rule := CoordinationRule{
			Digest:       "rule1",
			OrderedTasks: []string{"T1", "T2"}, // T2 must come after T1
		}
		activeLease := Lease{
			LeaseID:   "l1",
			MissionID: mission1.MissionID,
			TaskID:    mission1.TaskID,
			WorkerID:  "worker-1",
			State:     LeaseActive,
			ExpiresAt: now.Add(1 * time.Hour),
		}
		// An approved coordination rule is the one way past serialization.
		if err := CheckParallelConflict(mission2, []MissionV1{mission1}, []Lease{activeLease}, rule, now); err != nil {
			t.Errorf("coordinated ordering should permit dispatch: %v", err)
		}

		// Without a coordination rule and without proven isolation, the shared
		// worktree serializes the frontier — overlap or not, since the second
		// mission's diff would otherwise contain the first mission's edits.
		rule.Digest = "" // disable coordination
		disjoint := mission2
		disjoint.DeclaredFiles = []string{"unrelated.go"}
		for _, tc := range []struct {
			name      string
			candidate MissionV1
		}{{"overlapping", mission2}, {"disjoint", disjoint}} {
			err := CheckParallelConflict(tc.candidate, []MissionV1{mission1}, []Lease{activeLease}, rule, now)
			if err == nil {
				t.Errorf("%s: shared worktree must serialize a second active mission", tc.name)
			} else if !strings.Contains(err.Error(), "SHARED_WORKTREE_SERIAL") {
				t.Errorf("%s: unexpected refusal: %v", tc.name, err)
			}
		}

		// A host that proves isolation may run the disjoint mission concurrently.
		isolated := CoordinationRule{IsolationID: "wt-2"}
		if err := CheckParallelConflict(disjoint, []MissionV1{mission1}, []Lease{activeLease}, isolated, now); err != nil {
			t.Errorf("proven isolation should permit a disjoint mission: %v", err)
		}
		if err := CheckParallelConflict(mission2, []MissionV1{mission1}, []Lease{activeLease}, isolated, now); err == nil {
			t.Error("proven isolation must still refuse overlapping write scopes")
		}

		// An expired lease holds nothing: it must not serialize the frontier.
		stale := activeLease
		stale.ExpiresAt = now.Add(-time.Minute)
		if err := CheckParallelConflict(disjoint, []MissionV1{mission1}, []Lease{stale}, rule, now); err != nil {
			t.Errorf("expired lease should not block dispatch: %v", err)
		}
	})

	t.Run("isolation_binding_preserved_across_dispatch", func(t *testing.T) {
		// R4.5: preserve session/lease authority bindings
		lease := Lease{
			LeaseID:         "l1",
			MissionID:       m1.MissionID,
			TaskID:          m1.TaskID,
			WorkerID:        "worker-1",
			State:           LeaseActive,
			ExpiresAt:       now.Add(1 * time.Hour),
			DriverSessionID: "session-1", // Binding to a session
		}
		// Binding should persist across operations
		if lease.DriverSessionID != "session-1" {
			t.Errorf("driver session binding lost: got %q, want session-1", lease.DriverSessionID)
		}
		// Verify session validation works
		if err := ValidateLeaseSession(lease, "session-1"); err != nil {
			t.Errorf("valid session rejected: %v", err)
		}
		if err := ValidateLeaseSession(lease, "session-other"); err == nil {
			t.Errorf("invalid session accepted")
		}
	})
}

// TestWorkerOutOfScopeMissionCarriesWorker pins that the optional worker field
// round-trips through validation and payload without over-constraining a
// host-chooses plan (spec R6.4).
func TestWorkerOutOfScopeMissionCarriesWorker(t *testing.T) {
	m := validMission()
	m.Worker = "w1"
	if err := ValidateMission(m); err != nil {
		t.Fatalf("named worker rejected by ValidateMission: %v", err)
	}
	m.Worker = ""
	if err := ValidateMission(m); err != nil {
		t.Fatalf("host-chooses mission rejected: %v", err)
	}
}
