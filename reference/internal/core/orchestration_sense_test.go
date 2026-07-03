package core

import (
	"strings"
	"testing"
	"time"
)

func TestOrchestrationSenseBuildsStableSnapshot(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("6", 32)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()
	policy := validOrchestrationPolicy()
	store, err := NewACPStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimLease(sessionID, "pinky-a", "demo", "T1", 1, time.Minute, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	first, err := SenseOrchestration(root, "demo", sessionID, policy)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SenseOrchestration(root, "demo", sessionID, policy)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != second.Revision || first.SessionExpiresAt != second.SessionExpiresAt {
		t.Fatalf("snapshot not stable:\n%#v\n%#v", first, second)
	}
	if len(first.Runnable) != 1 || first.Runnable[0].ID != "T1" {
		t.Fatalf("runnable = %#v, want T1", first.Runnable)
	}
	if len(first.ActiveLeases) != 1 || first.ActiveLeases[0].WorkerID != "pinky-a" {
		t.Fatalf("leases = %#v, want active pinky-a", first.ActiveLeases)
	}
}

// TestOrchestrationCostLimitEndToEnd proves GAP-4 end-to-end: host-reported
// cost accumulated from real evidence events drives a step escalation and marks
// the session failed, instead of dispatching more work.
func TestOrchestrationCostLimitEndToEnd(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("7", 32)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	cfg := DefaultConfig.Orchestration
	cfg.HostReportedCostLimitUSD = 5.0
	policy, err := NewOrchestrationPolicy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := StartOrchestrationSession(root, "demo", sessionID, "operator", policy); err != nil {
		t.Fatalf("start session: %v", err)
	}

	// A worker claims, then reports terminal evidence with cost over the limit.
	mission, err := BuildPinkyMission(root, "demo", sessionID, "pinky-a", "T1", 1, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ClaimPinkyMission(root, mission, cfg); err != nil {
		t.Fatal(err)
	}
	if _, err := RecordPinkyTerminalReport(root, PinkyTerminalReport{
		SessionID:       sessionID,
		WorkerID:        "pinky-a",
		Spec:            "demo",
		TaskID:          "T1",
		Attempt:         1,
		VerificationRef: strings.Repeat("a", 32),
		Summary:         "done",
		HostCost:        "$6.00",
	}, cfg); err != nil {
		t.Fatalf("record report: %v", err)
	}

	snapshot, err := SenseOrchestration(root, "demo", sessionID, policy)
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.AccumulatedCostUSD != 6.0 {
		t.Fatalf("accumulated cost = %v, want 6", snapshot.AccumulatedCostUSD)
	}

	result, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if result.Decision.Action != OrchestrationEscalate ||
		result.Decision.Escalation.Code != EscalationPolicyViolation {
		t.Fatalf("decision = %#v, want escalate/policy-violation", result.Decision)
	}
	session, err := LoadOrchestrationSession(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != OrchestrationSessionFailed {
		t.Fatalf("session status = %s, want failed", session.Status)
	}
}
