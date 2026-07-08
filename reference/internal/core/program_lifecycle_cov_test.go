package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// program_lifecycle_cov_test.go drives the program-orchestration lease and
// session lifecycle error/edge branches that the happy-path tests in
// program_orchestration_test.go never reach: lease release, escalation guards,
// resume, sense, active-session lookup, and the session-status guards.

func TestReleaseProgramChildLease(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	cfg, _ := programTestPolicy(t)
	parentID := strings.Repeat("a", 32)

	// No lease yet → error.
	if _, err := ReleaseProgramChildLease(root, parentID, "a"); err == nil {
		t.Fatal("release without lease should error")
	}

	lease, err := AcquireProgramChildLease(root, parentID, "a", cfg)
	if err != nil {
		t.Fatal(err)
	}

	// Wrong parent → ownership error.
	other := strings.Repeat("b", 32)
	if _, err := ReleaseProgramChildLease(root, other, "a"); err == nil {
		t.Fatal("release by non-owner should error")
	}

	released, err := ReleaseProgramChildLease(root, parentID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if released.Status != ProgramChildLeaseReleased || released.ChildSessionID != lease.ChildSessionID {
		t.Fatalf("released lease = %#v", released)
	}

	// Idempotent: releasing an already-released lease returns it unchanged.
	again, err := ReleaseProgramChildLease(root, parentID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if again.Status != ProgramChildLeaseReleased {
		t.Fatalf("second release = %#v, want released", again)
	}

	// Bad parent ID fails validation up front.
	if _, err := ReleaseProgramChildLease(root, "not-hex", "a"); err == nil {
		t.Fatal("release with invalid parent ID should error")
	}
}

func TestMarkProgramChildLeaseEscalatedGuards(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	cfg, _ := programTestPolicy(t)
	parentID := strings.Repeat("c", 32)

	// No lease → error.
	if _, err := markProgramChildLeaseEscalated(root, parentID, "a"); err == nil {
		t.Fatal("escalate without lease should error")
	}

	if _, err := AcquireProgramChildLease(root, parentID, "a", cfg); err != nil {
		t.Fatal(err)
	}

	// Wrong parent → ownership error.
	if _, err := markProgramChildLeaseEscalated(root, strings.Repeat("d", 32), "a"); err == nil {
		t.Fatal("escalate by non-owner should error")
	}

	first, err := markProgramChildLeaseEscalated(root, parentID, "a")
	if err != nil {
		t.Fatal(err)
	}
	if first.Status != ProgramChildLeaseEscalated {
		t.Fatalf("status = %s, want escalated", first.Status)
	}
	// Idempotent.
	if _, err := markProgramChildLeaseEscalated(root, parentID, "a"); err != nil {
		t.Fatal(err)
	}

	// Released lease cannot be escalated.
	if _, err := ReleaseProgramChildLease(root, parentID, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := markProgramChildLeaseEscalated(root, parentID, "a"); err == nil {
		t.Fatal("escalate of released lease should error")
	}
}

func TestEnsureProgramChildSessionMismatch(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	scaffoldSpec(t, root, "b", StatusExecuting)
	cfg, policy := programTestPolicy(t)
	parentID := strings.Repeat("e", 32)

	lease, err := AcquireProgramChildLease(root, parentID, "a", cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := ensureProgramChildSession(root, lease, policy); err != nil {
		t.Fatal(err)
	}
	// Re-running is a no-op once the session exists for the right spec.
	if err := ensureProgramChildSession(root, lease, policy); err != nil {
		t.Fatal(err)
	}

	// A lease that points the existing child session at the wrong slug must be
	// rejected — the session belongs to "a", not "b".
	mismatch := lease
	mismatch.Slug = "b"
	if err := ensureProgramChildSession(root, mismatch, policy); err == nil {
		t.Fatal("session/slug mismatch should error")
	}
}

func TestResumeProgramOrchestration(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	cfg, policy := programTestPolicy(t)
	parentID := strings.Repeat("f", 32)

	first, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Started) != 1 {
		t.Fatalf("started=%d, want 1", len(first.Started))
	}
	if _, err := PauseProgramOrchestration(root, parentID); err != nil {
		t.Fatal(err)
	}

	resumed, err := ResumeProgramOrchestration(root, parentID)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != OrchestrationSessionRunning {
		t.Fatalf("resumed status = %s, want running", resumed.Status)
	}
	// The paused child must be resumed (running again) by control propagation.
	child, err := LoadOrchestrationSession(root, first.Started[0].ChildSessionID)
	if err != nil {
		t.Fatal(err)
	}
	if child.Status != OrchestrationSessionRunning {
		t.Fatalf("child status = %s, want running after resume", child.Status)
	}
}

func TestUpdateProgramSessionGuards(t *testing.T) {
	root := t.TempDir()
	parentID := strings.Repeat("1", 32)

	// Invalid / failed target status is rejected.
	if _, err := updateProgramSession(root, parentID, OrchestrationSessionFailed); err == nil {
		t.Fatal("updating to failed should error")
	}
	if _, err := updateProgramSession(root, parentID, OrchestrationSessionStatus("bogus")); err == nil {
		t.Fatal("invalid status should error")
	}

	// Drive the session to complete, then refuse further updates.
	if _, err := markProgramSessionStatus(root, parentID, OrchestrationSessionComplete); err != nil {
		t.Fatal(err)
	}
	if _, err := updateProgramSession(root, parentID, OrchestrationSessionPaused); err == nil {
		t.Fatal("updating a complete session should error")
	}

	// markProgramSessionStatus also validates its input.
	if _, err := markProgramSessionStatus(root, parentID, OrchestrationSessionStatus("bogus")); err == nil {
		t.Fatal("invalid mark status should error")
	}
}

func TestSenseProgramOrchestration(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	scaffoldSpec(t, root, "b", StatusComplete)
	cfg, policy := programTestPolicy(t)
	parentID := strings.Repeat("2", 32)

	// Unknown session → not found.
	if _, err := SenseProgramOrchestration(root, parentID, cfg); err == nil {
		t.Fatal("sense of unknown session should error")
	}

	if _, err := StepProgramOrchestration(root, parentID, policy, cfg); err != nil {
		t.Fatal(err)
	}
	report, err := SenseProgramOrchestration(root, parentID, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if report.Counts.Total == 0 {
		t.Fatalf("report counts empty: %+v", report.Counts)
	}
	if report.Session.ParentSessionID != parentID {
		t.Fatalf("session not threaded: %+v", report.Session)
	}

	// Bad parent ID is rejected up front.
	if _, err := SenseProgramOrchestration(root, "bad", cfg); err == nil {
		t.Fatal("sense with invalid ID should error")
	}
}

func TestActiveOrchestrationSessionForSpec(t *testing.T) {
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	cfg, policy := programTestPolicy(t)
	parentID := strings.Repeat("3", 32)

	// No sessions dir yet → nil, nil.
	got, err := ActiveOrchestrationSessionForSpec(root, "a")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Fatalf("expected no active session, got %#v", got)
	}

	first, err := StepProgramOrchestration(root, parentID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Started) != 1 {
		t.Fatalf("started=%d, want 1", len(first.Started))
	}

	got, err = ActiveOrchestrationSessionForSpec(root, "a")
	if err != nil {
		t.Fatal(err)
	}
	if got == nil || got.Spec != "a" {
		t.Fatalf("active session for a = %#v", got)
	}

	// A spec with no session returns nil.
	none, err := ActiveOrchestrationSessionForSpec(root, "nonexistent")
	if err != nil {
		t.Fatal(err)
	}
	if none != nil {
		t.Fatalf("expected nil for spec with no session, got %#v", none)
	}
}

func TestValidateProgramSessionErrors(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	valid := ProgramSession{
		Version:         OrchestrationModelVersion,
		ParentSessionID: strings.Repeat("a", 32),
		Status:          OrchestrationSessionRunning,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := validateProgramSession(valid); err != nil {
		t.Fatalf("valid session rejected: %v", err)
	}

	cases := map[string]func(s *ProgramSession){
		"bad version":     func(s *ProgramSession) { s.Version = 0 },
		"bad parent":      func(s *ProgramSession) { s.ParentSessionID = "nope" },
		"bad status":      func(s *ProgramSession) { s.Status = "bogus" },
		"bad created":     func(s *ProgramSession) { s.CreatedAt = "not-a-time" },
		"bad updated":     func(s *ProgramSession) { s.UpdatedAt = "not-a-time" },
		"updated<created": func(s *ProgramSession) { s.UpdatedAt = "2020-01-01T00:00:00Z" },
	}
	for name, mutate := range cases {
		s := valid
		mutate(&s)
		if err := validateProgramSession(s); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestValidateProgramChildLeaseErrors(t *testing.T) {
	now := time.Date(2026, 6, 19, 12, 0, 0, 0, time.UTC)
	acquired := now.Format(time.RFC3339Nano)
	until := now.Add(time.Hour).Format(time.RFC3339Nano)
	valid := ProgramChildLease{
		Version:         OrchestrationModelVersion,
		ParentSessionID: strings.Repeat("a", 32),
		ChildSessionID:  strings.Repeat("b", 32),
		Slug:            "a",
		Status:          ProgramChildLeaseActive,
		AcquiredAt:      acquired,
		LeaseUntil:      until,
	}
	if err := validateProgramChildLease(valid); err != nil {
		t.Fatalf("valid lease rejected: %v", err)
	}

	cases := map[string]func(l *ProgramChildLease){
		"bad version":        func(l *ProgramChildLease) { l.Version = 0 },
		"bad parent":         func(l *ProgramChildLease) { l.ParentSessionID = "x" },
		"bad child":          func(l *ProgramChildLease) { l.ChildSessionID = "x" },
		"bad slug":           func(l *ProgramChildLease) { l.Slug = "Bad Slug" },
		"bad status":         func(l *ProgramChildLease) { l.Status = "bogus" },
		"bad acquired":       func(l *ProgramChildLease) { l.AcquiredAt = "nope" },
		"bad until":          func(l *ProgramChildLease) { l.LeaseUntil = "nope" },
		"until<=acquired":    func(l *ProgramChildLease) { l.LeaseUntil = l.AcquiredAt },
		"active has release": func(l *ProgramChildLease) { l.ReleasedAt = until },
		"released no time": func(l *ProgramChildLease) {
			l.Status = ProgramChildLeaseReleased
		},
		"released has escalated": func(l *ProgramChildLease) {
			l.Status = ProgramChildLeaseReleased
			l.ReleasedAt = until
			l.EscalatedAt = until
		},
		"escalated no time": func(l *ProgramChildLease) {
			l.Status = ProgramChildLeaseEscalated
		},
	}
	for name, mutate := range cases {
		l := valid
		mutate(&l)
		if err := validateProgramChildLease(l); err == nil {
			t.Errorf("%s: expected validation error", name)
		}
	}
}

func TestLoadProgramSessionNotFound(t *testing.T) {
	root := t.TempDir()
	_, err := LoadProgramSession(root, strings.Repeat("a", 32))
	if !errors.Is(err, errOrchestrationSessionNotFound) {
		t.Fatalf("err = %v, want not-found", err)
	}
}
