package core

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

// recoveryFixture starts a running session over the demo spec under a fixed,
// mutable clock. The returned pointer lets a test advance wall time to expire
// leases deterministically.
func recoveryFixture(t *testing.T) (root, sessionID string, cfg OrchestrationCfg, policy OrchestrationPolicy, now *time.Time) {
	t.Helper()
	root = writePinkySpec(t)
	sessionID = strings.Repeat("8", 32)
	cfg = DefaultConfig.Orchestration
	policy = validOrchestrationPolicy()
	clock := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	now = &clock
	restore := setCoreClock(func() time.Time { return *now })
	t.Cleanup(restore)
	if _, err := StartOrchestrationSession(root, "demo", sessionID, "operator", policy); err != nil {
		t.Fatalf("start session: %v", err)
	}
	return root, sessionID, cfg, policy, now
}

func claimDemoLease(t *testing.T, root, sessionID, workerID string, attempt int, cfg OrchestrationCfg) ACPLease {
	t.Helper()
	mission, err := BuildPinkyMission(root, "demo", sessionID, workerID, "T1", attempt, cfg)
	if err != nil {
		t.Fatalf("build mission: %v", err)
	}
	claim, err := ClaimPinkyMission(root, mission, cfg)
	if err != nil {
		t.Fatalf("claim mission: %v", err)
	}
	return claim.Lease
}

func TestOrchestrationPauseStopsDispatch(t *testing.T) {
	root, sessionID, cfg, policy, _ := recoveryFixture(t)

	first, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if first.Decision.Action != OrchestrationDispatch || first.Event == nil {
		t.Fatalf("running step = %#v / event %v, want dispatch", first.Decision, first.Event)
	}

	if _, err := PauseOrchestration(root, sessionID); err != nil {
		t.Fatalf("pause: %v", err)
	}
	paused, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if paused.Decision.Action != OrchestrationWait {
		t.Fatalf("paused decision = %s, want wait", paused.Decision.Action)
	}
	if paused.Event != nil {
		t.Fatalf("paused step wrote an event %#v — pause must stop new dispatch", paused.Event)
	}
}

func TestOrchestrationResumeRestoresDispatch(t *testing.T) {
	root, sessionID, cfg, policy, _ := recoveryFixture(t)

	if _, err := PauseOrchestration(root, sessionID); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if _, err := ResumeOrchestration(root, sessionID); err != nil {
		t.Fatalf("resume: %v", err)
	}
	resumed, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Decision.Action != OrchestrationDispatch || resumed.Decision.TaskID != "T1" {
		t.Fatalf("resumed decision = %#v, want dispatch T1", resumed.Decision)
	}
}

func TestOrchestrationResumeRejectsTerminalSession(t *testing.T) {
	root, sessionID, _, _, _ := recoveryFixture(t)
	if _, err := CancelOrchestration(root, sessionID); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if _, err := ResumeOrchestration(root, sessionID); err == nil {
		t.Fatal("resume of a cancelling session should fail closed")
	}
}

func TestOrchestrationCancelEmitsDirectivesAndCompletes(t *testing.T) {
	root, sessionID, cfg, policy, now := recoveryFixture(t)
	claimDemoLease(t, root, sessionID, "wkr", 1, cfg)

	if _, err := CancelOrchestration(root, sessionID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	// First cancelling step issues exactly one cooperative directive.
	directed, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if directed.Decision.Action != OrchestrationCancel || directed.Decision.TaskID != "T1" {
		t.Fatalf("cancel step decision = %#v, want cancel T1", directed.Decision)
	}
	if directed.Event == nil || directed.Event.Type != ACPMessageDirective {
		t.Fatalf("cancel step event = %#v, want a directive", directed.Event)
	}

	// While the lease is still active and already directed, the loop waits — it
	// does not re-issue and does not complete.
	waiting, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if waiting.Decision.Action != OrchestrationWait || waiting.Event != nil {
		t.Fatalf("second cancel step = %#v / event %v, want wait without event", waiting.Decision, waiting.Event)
	}

	// Expire the lease, then the session drains to complete.
	*now = now.Add(time.Hour)
	done, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if done.Decision.Action != OrchestrationCompleteSession {
		t.Fatalf("drained cancel step = %s, want complete-session", done.Decision.Action)
	}
	session, err := LoadOrchestrationSession(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if session.Status != OrchestrationSessionComplete {
		t.Fatalf("session status = %s, want complete", session.Status)
	}
}

func TestOrchestrationCancelIsCooperativeNotKill(t *testing.T) {
	root, sessionID, cfg, policy, _ := recoveryFixture(t)
	claimDemoLease(t, root, sessionID, "wkr", 1, cfg)
	if _, err := CancelOrchestration(root, sessionID); err != nil {
		t.Fatalf("cancel: %v", err)
	}

	step, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}

	// The directive is a cooperative request, not a kill: its payload action is
	// "cancel" and the worker's lease is left intact for the host to stop itself.
	var payload ACPDirectivePayload
	if err := json.Unmarshal(step.Event.Payload, &payload); err != nil {
		t.Fatalf("decode directive payload: %v", err)
	}
	if payload.Action != "cancel" {
		t.Fatalf("directive action = %q, want cancel", payload.Action)
	}
	store, err := NewACPStore(root)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := store.LoadLease(sessionID, "wkr")
	if err != nil {
		t.Fatalf("worker lease should still exist after cancel: %v", err)
	}
	if lease.Status != ACPLeaseActive {
		t.Fatalf("lease status = %s, want still active — cancellation must not force-terminate", lease.Status)
	}
}

func TestOrchestrationRecoveryConvergesAcrossRestart(t *testing.T) {
	root, sessionID, cfg, policy, _ := recoveryFixture(t)

	// Recovery at the zero-event boundary reconciles to sequence 0.
	recovered, err := RecoverOrchestration(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.LastSequence != 0 {
		t.Fatalf("recovered LastSequence = %d, want 0", recovered.LastSequence)
	}

	// Commit one event, then recovery converges to sequence 1 regardless of the
	// stale LastSequence in session.json.
	if _, err := StepOrchestration(root, "demo", sessionID, policy, cfg); err != nil {
		t.Fatal(err)
	}
	first, err := RecoverOrchestration(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if first.LastSequence != 1 {
		t.Fatalf("recovered LastSequence = %d, want 1", first.LastSequence)
	}

	// Recovering again is a no-op and byte-identical: restart converges.
	second, err := RecoverOrchestration(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	a, _ := json.Marshal(first)
	b, _ := json.Marshal(second)
	if string(a) != string(b) {
		t.Fatalf("repeated recovery diverged:\n%s\n%s", a, b)
	}
}

func TestOrchestrationRecoveryReclaimsExpiredLeases(t *testing.T) {
	root, sessionID, cfg, _, now := recoveryFixture(t)
	claimDemoLease(t, root, sessionID, "wkr", 1, cfg)

	// An active lease is not reclaimable.
	if n, err := ReclaimExpiredLeases(root, sessionID); err != nil || n != 0 {
		t.Fatalf("reclaim active = (%d,%v), want (0,nil)", n, err)
	}

	// Once expired it is reclaimed exactly once; reclaiming again is a no-op.
	*now = now.Add(time.Hour)
	if n, err := ReclaimExpiredLeases(root, sessionID); err != nil || n != 1 {
		t.Fatalf("reclaim expired = (%d,%v), want (1,nil)", n, err)
	}
	if n, err := ReclaimExpiredLeases(root, sessionID); err != nil || n != 0 {
		t.Fatalf("second reclaim = (%d,%v), want (0,nil)", n, err)
	}
	store, err := NewACPStore(root)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := store.LoadLease(sessionID, "wkr")
	if err != nil {
		t.Fatal(err)
	}
	if lease.Status != ACPLeaseReleased {
		t.Fatalf("reclaimed lease status = %s, want released", lease.Status)
	}
}

func TestOrchestrationRetryReentersAfterReclaim(t *testing.T) {
	root, sessionID, cfg, _, now := recoveryFixture(t)
	claimDemoLease(t, root, sessionID, "wkr", 1, cfg)

	// Expire and reclaim attempt 1, then a fresh worker may claim attempt 2.
	*now = now.Add(time.Hour)
	if _, err := ReclaimExpiredLeases(root, sessionID); err != nil {
		t.Fatalf("reclaim: %v", err)
	}
	retry := claimDemoLease(t, root, sessionID, "wkr2", 2, cfg)
	if retry.Attempt != 2 {
		t.Fatalf("retry lease attempt = %d, want 2", retry.Attempt)
	}
}
