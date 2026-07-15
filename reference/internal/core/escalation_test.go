package core

import (
	"testing"
	"time"
)

func TestEvaluateEscalationTableDriven(t *testing.T) {
	base := EscalationConfig{Enabled: true, ComplexityThreshold: 8}
	cases := []struct {
		name      string
		cfg       EscalationConfig
		facts     EscalationFacts
		wantRule  EscalationRuleName
		wantFired int
	}{
		{"disabled never fires", EscalationConfig{Enabled: false}, EscalationFacts{VerifyFailCount: 99, BlockerCount: 99}, "", 0},
		{"below all thresholds", base, EscalationFacts{VerifyFailCount: 1, BlockerCount: 0}, "", 0},
		{"verify fail at threshold", base, EscalationFacts{VerifyFailCount: 2}, RuleVerifyFail, 1},
		{"retry exhausted", base, EscalationFacts{RetryCount: 3, MaxRetries: 3}, RuleRetryExhausted, 1},
		{"retry ignored when no budget", base, EscalationFacts{RetryCount: 3, MaxRetries: 0}, "", 0},
		{"blocker fires", base, EscalationFacts{BlockerCount: 1}, RuleBlocker, 1},
		{"cost over budget", base, EscalationFacts{CostUSD: 1.5, TierBudgetUSD: 1.0}, RuleCostOverBudget, 1},
		{"cost ignored when no budget", base, EscalationFacts{CostUSD: 9.0, TierBudgetUSD: 0}, "", 0},
		{"complexity fires", base, EscalationFacts{ComplexityScore: 8}, RuleComplexity, 1},
		{"complexity disabled at zero threshold", EscalationConfig{Enabled: true}, EscalationFacts{ComplexityScore: 999}, "", 0},
		{"priority: verify-fail wins over blocker", base, EscalationFacts{VerifyFailCount: 2, BlockerCount: 5}, RuleVerifyFail, 2},
		{"custom verify threshold", EscalationConfig{Enabled: true, VerifyFailThreshold: 5}, EscalationFacts{VerifyFailCount: 4}, "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := EvaluateEscalation(tc.facts, tc.cfg)
			if got.Rule != tc.wantRule {
				t.Fatalf("rule = %q, want %q", got.Rule, tc.wantRule)
			}
			if len(got.Fired) != tc.wantFired {
				t.Fatalf("fired = %v (%d), want %d", got.Fired, len(got.Fired), tc.wantFired)
			}
			if tc.wantRule == "" && got.Triggered {
				t.Fatalf("expected untriggered, got triggered")
			}
			if tc.wantRule != "" && !got.Triggered {
				t.Fatalf("expected triggered, got untriggered")
			}
		})
	}
}

func TestEvaluateEscalationDeterministicFacts(t *testing.T) {
	cfg := EscalationConfig{Enabled: true}
	facts := EscalationFacts{VerifyFailCount: 3, BlockerCount: 2}
	a := EvaluateEscalation(facts, cfg)
	b := EvaluateEscalation(facts, cfg)
	if a.Facts != b.Facts {
		t.Fatalf("facts not byte-stable: %q vs %q", a.Facts, b.Facts)
	}
	// Sorted key rendering: blockers before verifyFail.
	if a.Facts != "blockers=2 verifyFail=3" {
		t.Fatalf("unexpected facts rendering: %q", a.Facts)
	}
}

func TestEscalationFactsForTaskFromTrajectory(t *testing.T) {
	ec := func(n int) *int { return &n }
	traj := []TrajectoryEvent{
		{Tool: "verify", TaskIDs: []string{"T1"}, ExitCode: ec(1)},
		{Tool: "verify", TaskIDs: []string{"T1"}, ExitCode: ec(1)},
		{Tool: "verify", TaskIDs: []string{"T1"}, ExitCode: ec(0)},
		{Tool: "verify", TaskIDs: []string{"T2"}, ExitCode: ec(1)}, // other task
		{Tool: "read", TaskIDs: []string{"T1"}, ExitCode: ec(0)},   // non-verify
	}
	state := &State{Blockers: []Blocker{{Task: "T1"}, {Task: "T2"}}}
	facts := EscalationFactsForTask(state, traj, "T1", 2, 0, 0)
	if facts.VerifyFailCount != 2 {
		t.Fatalf("VerifyFailCount = %d, want 2", facts.VerifyFailCount)
	}
	if facts.RetryCount != 2 { // 3 attempts - 1
		t.Fatalf("RetryCount = %d, want 2", facts.RetryCount)
	}
	if facts.BlockerCount != 1 {
		t.Fatalf("BlockerCount = %d, want 1", facts.BlockerCount)
	}
}

func TestNewEscalationRecordAndRationale(t *testing.T) {
	orig := Clock
	Clock = func() time.Time { return time.Date(2026, 7, 2, 12, 0, 0, 0, time.UTC) }
	defer func() { Clock = orig }()
	v := EvaluateEscalation(EscalationFacts{VerifyFailCount: 2}, EscalationConfig{Enabled: true})
	rec := NewEscalationRecord(v, "T1")
	if rec.Rule != RuleVerifyFail || rec.Task != "T1" {
		t.Fatalf("bad record: %+v", rec)
	}
	if rec.Time == "" {
		t.Fatalf("record missing time")
	}
	r := EscalationRationale(v, "T1")
	if r == "" {
		t.Fatalf("expected rationale")
	}
}
