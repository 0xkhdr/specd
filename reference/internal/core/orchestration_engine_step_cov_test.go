package core

import (
	"strings"
	"testing"
	"time"
)

// orchestration_engine_step_cov_test.go covers the session-aware StepOrchestration
// branches and the active-session lookup that the recovery suite leaves open:
// the terminal-session idle path and ActiveOrchestrationSessionForSpec's
// present/absent/terminal cases.

func TestActiveOrchestrationSessionForSpecTerminalSkipped(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("8", 32)
	policy := validOrchestrationPolicy()
	clock := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return clock })
	t.Cleanup(restore)
	if _, err := StartOrchestrationSession(root, "demo", sessionID, "operator", policy); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Running session is reported active.
	if got, err := ActiveOrchestrationSessionForSpec(root, "demo"); err != nil || got == nil || got.SessionID != sessionID {
		t.Fatalf("active lookup = (%#v,%v), want the running session", got, err)
	}

	// Once terminal, the lookup skips it and reports no active session.
	if _, err := markOrchestrationSessionStatus(root, sessionID, OrchestrationSessionComplete); err != nil {
		t.Fatalf("mark complete: %v", err)
	}
	if got, err := ActiveOrchestrationSessionForSpec(root, "demo"); err != nil || got != nil {
		t.Fatalf("terminal lookup = (%v,%v), want (nil,nil)", got, err)
	}
}

func TestStepOrchestrationTerminalIdles(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("8", 32)
	cfg := DefaultConfig.Orchestration
	policy := validOrchestrationPolicy()
	clock := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return clock })
	t.Cleanup(restore)
	if _, err := StartOrchestrationSession(root, "demo", sessionID, "operator", policy); err != nil {
		t.Fatalf("start: %v", err)
	}

	// Force the session terminal, then a step must idle with no event.
	if _, err := markOrchestrationSessionStatus(root, sessionID, OrchestrationSessionComplete); err != nil {
		t.Fatalf("mark complete: %v", err)
	}
	res, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision.Action != OrchestrationIdle {
		t.Fatalf("terminal step = %s, want idle", res.Decision.Action)
	}
	if res.Event != nil {
		t.Fatalf("terminal step wrote event %#v, want none", res.Event)
	}
}
