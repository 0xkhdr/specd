package core

import (
	"strings"
	"testing"
)

func TestWave4PRSummaryIncludesCommitLinks(t *testing.T) {
	state := &State{Tasks: map[string]TaskState{
		"T2": {},
		"T1": {},
	}}
	summary := BuildPRSummary(state, nil, nil, []CommitLink{
		{SHA: "abcdef0123456789", Tasks: []string{"T2", "T1"}},
	})
	if len(summary.Commits) != 1 {
		t.Fatalf("commits = %d, want 1", len(summary.Commits))
	}
	if summary.Commits[0].SHA != "abcdef0123456789" {
		t.Fatalf("sha = %q", summary.Commits[0].SHA)
	}
	if got := strings.Join(summary.Commits[0].Tasks, ","); got != "T2,T1" {
		t.Fatalf("tasks = %q", got)
	}
}

func TestWave4EscalationRecordAndRationale(t *testing.T) {
	verdict := EscalationVerdict{
		Triggered: true,
		Facts:     "verify_failures=3",
	}
	record := NewEscalationRecord(verdict, "T1")
	if record == nil {
		t.Fatal("expected escalation record")
	}
	if !strings.Contains(EscalationRationale(verdict, "T1"), "T1") {
		t.Fatalf("rationale missing task: %q", EscalationRationale(verdict, "T1"))
	}
}

func TestWave4EscalationDisabledAndRecommendation(t *testing.T) {
	verdict := EvaluateEscalation(EscalationFacts{}, EscalationConfig{Enabled: false})
	if verdict.Triggered {
		t.Fatalf("disabled escalation triggered: %#v", verdict)
	}
	if got := EscalationRationale(verdict, "T1"); got != "" {
		t.Fatalf("disabled rationale = %q", got)
	}

	state := &State{Escalation: NewEscalationRecord(EscalationVerdict{
		Triggered: true,
		Facts:     "verify_failures=3",
	}, "T1")}
	rec := RecommendConductorForEscalation(state)
	if rec.Recommended != "conductor" {
		t.Fatalf("expected conductor recommendation: %#v", rec)
	}
	if !strings.Contains(rec.Rationale, "T1") {
		t.Fatalf("recommendation rationale = %q", rec.Rationale)
	}
}
