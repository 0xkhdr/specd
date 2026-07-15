package core

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func startTestSession(t *testing.T, root string) string {
	t.Helper()
	sessionID := strings.Repeat("a", 32)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()
	if _, err := StartOrchestrationSession(root, "demo", sessionID, "tester", validOrchestrationPolicy()); err != nil {
		t.Fatalf("start session: %v", err)
	}
	return sessionID
}

func TestCompactOrchestrationSessionManual(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := startTestSession(t, root)
	now := time.Date(2026, 6, 18, 12, 5, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	outcome, err := CompactOrchestrationSession(root, "demo", sessionID, "")
	if err != nil {
		t.Fatalf("compact: %v", err)
	}
	if outcome.Entry.Reason != "manual-clear" || !outcome.Entry.Compacted {
		t.Fatalf("unexpected entry: %+v", outcome.Entry)
	}
	if _, err := os.Stat(filepath.Join(root, outcome.SummaryFile)); err != nil {
		t.Fatalf("summary file missing: %v", err)
	}
	state, _ := LoadState(root, "demo")
	if state.Turn != 1 {
		t.Fatalf("want Turn bumped to 1, got %d", state.Turn)
	}
	session, err := LoadOrchestrationSession(root, sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(session.ContextLedger) != 1 || session.LastCompactionStep == 0 {
		t.Fatalf("ledger not recorded: %+v step=%d", session.ContextLedger, session.LastCompactionStep)
	}

	// A non-running session cannot be compacted.
	if _, err := markOrchestrationSessionStatus(root, sessionID, OrchestrationSessionComplete); err != nil {
		t.Fatal(err)
	}
	if _, err := CompactOrchestrationSession(root, "demo", sessionID, ""); err == nil {
		t.Fatalf("compacted a non-running session")
	}
}

func TestStepOrchestrationBudgetCompaction(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := startTestSession(t, root)
	now := time.Date(2026, 6, 18, 12, 5, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	// Seed a pre-dispatch ledger entry whose estimate blows the budget.
	if err := recordSessionLedgerEntry(root, sessionID, ContextLedgerEntry{
		Action: "dispatch", EstimatedTokens: 9000, Budget: 1000, SoftCeiling: 1000, Reason: "pre-dispatch",
	}); err != nil {
		t.Fatal(err)
	}

	policy := validOrchestrationPolicy()
	policy.CompactionPolicy = CompactionBudget
	policy.CompactionBudgetThreshold = 0.5
	cfg := DefaultConfig.Orchestration

	res, err := StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision.Action != OrchestrationCompact || res.Compaction == nil {
		t.Fatalf("want compact decision + outcome, got %s / %v", res.Decision.Action, res.Compaction)
	}

	// The compaction ledger entry resets the estimate, so the next step settles to
	// normal dispatch rather than re-compacting.
	res, err = StepOrchestration(root, "demo", sessionID, policy, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if res.Decision.Action == OrchestrationCompact {
		t.Fatalf("budget compaction did not settle — re-emitted")
	}
	if res.Decision.Action != OrchestrationDispatch {
		t.Fatalf("want dispatch after compaction settles, got %s", res.Decision.Action)
	}
}

func TestOrchestrationSessionBackwardCompatBytes(t *testing.T) {
	// A pre-compaction session.json (no ledger fields) must round-trip byte-identical.
	old := `{
  "version": 1,
  "sessionId": "` + strings.Repeat("a", 32) + `",
  "spec": "demo",
  "owner": "tester",
  "status": "running",
  "policy": {
    "approvalPolicy": "planning",
    "maxWorkers": 2,
    "maxRetries": 2,
    "sessionTimeoutSeconds": 3600,
    "hostReportedCostLimitUSD": 0
  },
  "createdAt": "2026-06-18T12:00:00Z",
  "updatedAt": "2026-06-18T12:00:00Z",
  "expiresAt": "2026-06-18T13:00:00Z",
  "lastSequence": 0
}`
	var session OrchestrationSession
	if err := json.Unmarshal([]byte(old), &session); err != nil {
		t.Fatalf("decode: %v", err)
	}
	out, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != old {
		t.Fatalf("byte-stability broken:\n--- got ---\n%s\n--- want ---\n%s", out, old)
	}
}

func TestDecideCompactionPhaseBoundary(t *testing.T) {
	policy := validOrchestrationPolicy()
	policy.ApprovalPolicy = "session"
	policy.CompactionPolicy = CompactionPhase

	// tasks gate satisfied, ready to advance: a phase boundary not yet compacted.
	snap := validOrchestrationSnapshot()
	snap.Status = StatusTasks
	snap.Phase = PhasePlan
	snap.PlanningReady = true
	snap.Revision = 4
	snap.LastCompactionStep = 0

	decision, err := DecideOrchestration(snap, policy)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if decision.Action != OrchestrationCompact || decision.Reason != "phase-boundary" {
		t.Fatalf("want compact/phase-boundary, got %s/%s", decision.Action, decision.Reason)
	}

	// Same boundary already compacted (guard): must not re-emit compact.
	snap.LastCompactionStep = uint64(snap.Revision)
	decision, err = DecideOrchestration(snap, policy)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if decision.Action == OrchestrationCompact {
		t.Fatalf("re-emitted compact on the same boundary")
	}
	if decision.Action != OrchestrationAdvancePhase {
		t.Fatalf("want advance-phase after compaction, got %s", decision.Action)
	}
}

func TestDecideCompactionBudgetThreshold(t *testing.T) {
	policy := validOrchestrationPolicy()
	policy.CompactionPolicy = CompactionBudget
	policy.CompactionBudgetThreshold = 0.8

	snap := validOrchestrationSnapshot() // executing, no runnable
	snap.LedgerBudget = 1000

	// Below threshold: no compaction.
	snap.LedgerEstimatedTokens = 700
	decision, err := DecideOrchestration(snap, policy)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if decision.Action == OrchestrationCompact {
		t.Fatalf("compacted below threshold")
	}

	// At/above threshold: compaction fires.
	snap.LedgerEstimatedTokens = 850
	decision, err = DecideOrchestration(snap, policy)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if decision.Action != OrchestrationCompact || decision.Reason != "budget-threshold" {
		t.Fatalf("want compact/budget-threshold, got %s/%s", decision.Action, decision.Reason)
	}
}

func TestDecideCompactionDisabledByDefault(t *testing.T) {
	policy := validOrchestrationPolicy() // CompactionPolicy empty => none
	snap := validOrchestrationSnapshot()
	snap.Status = StatusTasks
	snap.Phase = PhasePlan
	snap.PlanningReady = true
	snap.LedgerBudget = 1000
	snap.LedgerEstimatedTokens = 999
	decision, err := DecideOrchestration(snap, policy)
	if err != nil {
		t.Fatalf("decide: %v", err)
	}
	if decision.Action == OrchestrationCompact {
		t.Fatalf("compacted with policy none")
	}
}

func TestAppendContextLedgerPeak(t *testing.T) {
	var session OrchestrationSession
	AppendContextLedger(&session, ContextLedgerEntry{Action: "dispatch", EstimatedTokens: 1200, Budget: 4000})
	AppendContextLedger(&session, ContextLedgerEntry{Action: "step", HostReportedTokens: 3100})
	AppendContextLedger(&session, ContextLedgerEntry{Action: "compact", EstimatedTokens: 200})
	if len(session.ContextLedger) != 3 {
		t.Fatalf("want 3 entries, got %d", len(session.ContextLedger))
	}
	if session.PeakTokens != 3100 {
		t.Fatalf("want peak 3100, got %d", session.PeakTokens)
	}
	if tail, ok := lastContextLedgerEntry(session); !ok || tail.EstimatedTokens != 200 {
		t.Fatalf("unexpected ledger tail: %+v ok=%v", tail, ok)
	}
}
