package core

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestOrchestrationModelCanonicalSnapshot(t *testing.T) {
	snapshot := validOrchestrationSnapshot()
	snapshot.Runnable = []OrchestrationTaskSnapshot{
		{ID: "T10", Wave: 2, Status: TaskPending, Attempt: 1, Role: "builder", Depends: nil},
		{ID: "T2", Wave: 1, Status: TaskPending, Attempt: 1, Role: "investigator", Depends: []string{"T3", "T1"}},
	}
	snapshot.ActiveLeases = nil
	snapshot.RecentFailures = nil

	first, err := CanonicalOrchestrationJSON(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalOrchestrationJSON(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("canonical snapshots differ:\n%s\n%s", first, second)
	}
	text := string(first)
	if strings.Contains(text, "null") {
		t.Fatalf("canonical snapshot contains null list:\n%s", text)
	}
	if strings.Index(text, `"id": "T2"`) > strings.Index(text, `"id": "T10"`) {
		t.Fatalf("tasks are not ordinal-sorted:\n%s", text)
	}
	if strings.Index(text, `"T1"`) > strings.Index(text, `"T3"`) {
		t.Fatalf("dependencies are not sorted:\n%s", text)
	}
}

func TestOrchestrationModelCoversLifecycleAndActions(t *testing.T) {
	statuses := []SpecStatus{
		StatusRequirements, StatusDesign, StatusTasks, StatusExecuting,
		StatusVerifying, StatusComplete, StatusBlocked,
	}
	for _, status := range statuses {
		snapshot := validOrchestrationSnapshot()
		snapshot.Status = status
		snapshot.Phase = PhaseForStatus(status)
		if _, err := CanonicalOrchestrationJSON(snapshot); err != nil {
			t.Errorf("status %q: %v", status, err)
		}
	}
	for _, gate := range []Gate{GateNone, GateAwaitingApproval} {
		snapshot := validOrchestrationSnapshot()
		snapshot.Gate = gate
		if _, err := CanonicalOrchestrationJSON(snapshot); err != nil {
			t.Errorf("gate %q: %v", gate, err)
		}
	}

	actions := []OrchestrationAction{
		OrchestrationIdle, OrchestrationRequestApproval, OrchestrationDispatch,
		OrchestrationWait, OrchestrationRetry, OrchestrationCancel,
		OrchestrationReplan, OrchestrationEscalate, OrchestrationCompleteSession,
	}
	for _, action := range actions {
		decision := validOrchestrationDecision(action)
		if _, err := CanonicalOrchestrationJSON(decision); err != nil {
			t.Errorf("action %q: %v", action, err)
		}
	}
}

func TestOrchestrationModelPolicyBoundaries(t *testing.T) {
	cfg := DefaultConfig.Orchestration
	policy, err := NewOrchestrationPolicy(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if policy.ApprovalPolicy != "manual" || policy.MaxWorkers != cfg.MaxWorkers ||
		policy.SessionTimeoutSeconds != cfg.SessionTimeoutMinutes*60 {
		t.Fatalf("policy = %#v, want config parity", policy)
	}

	tests := []struct {
		name   string
		mutate func(*OrchestrationPolicy)
	}{
		{"approval", func(p *OrchestrationPolicy) { p.ApprovalPolicy = "always" }},
		{"workers", func(p *OrchestrationPolicy) { p.MaxWorkers = 0 }},
		{"retries", func(p *OrchestrationPolicy) { p.MaxRetries = maxMaxRetries + 1 }},
		{"timeout", func(p *OrchestrationPolicy) { p.SessionTimeoutSeconds = 0 }},
		{"cost", func(p *OrchestrationPolicy) { p.HostReportedCostLimitUSD = -1 }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			invalid := policy
			tt.mutate(&invalid)
			if err := ValidateOrchestrationPolicy(invalid); err == nil {
				t.Fatalf("invalid policy accepted: %#v", invalid)
			}
		})
	}
}

func TestOrchestrationModelSessionDeterministic(t *testing.T) {
	policy, err := NewOrchestrationPolicy(DefaultConfig.Orchestration)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	session := OrchestrationSession{
		Version:      OrchestrationModelVersion,
		SessionID:    strings.Repeat("2", 32),
		Spec:         "example",
		Owner:        "cli",
		Status:       OrchestrationSessionRunning,
		Policy:       policy,
		CreatedAt:    created.Format(time.RFC3339Nano),
		UpdatedAt:    created.Format(time.RFC3339Nano),
		ExpiresAt:    orchestrationSessionExpiry(created, policy).Format(time.RFC3339Nano),
		LastSequence: 0,
	}
	first, err := CanonicalOrchestrationJSON(session)
	if err != nil {
		t.Fatal(err)
	}
	second, err := CanonicalOrchestrationJSON(session)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("session JSON drifted:\n%s\n%s", first, second)
	}
}

func TestOrchestrationModelRejectsUnknownValues(t *testing.T) {
	snapshot := validOrchestrationSnapshot()
	snapshot.Status = "mystery"
	if _, err := CanonicalOrchestrationJSON(snapshot); err == nil {
		t.Fatal("unknown status accepted")
	}

	decision := validOrchestrationDecision(OrchestrationIdle)
	decision.Action = "invent"
	if _, err := CanonicalOrchestrationJSON(decision); err == nil {
		t.Fatal("unknown action accepted")
	}

	decision = validOrchestrationDecision(OrchestrationEscalate)
	decision.Escalation.Code = EscalationNone
	if _, err := CanonicalOrchestrationJSON(decision); err == nil {
		t.Fatal("escalation without code accepted")
	}

	snapshot = validOrchestrationSnapshot()
	snapshot.Runnable = []OrchestrationTaskSnapshot{
		{ID: "T1", Wave: 1, Status: TaskPending, Attempt: 1, Role: "builder", Depends: []string{}},
		{ID: "T1", Wave: 1, Status: TaskPending, Attempt: 1, Role: "builder", Depends: []string{}},
	}
	if _, err := CanonicalOrchestrationJSON(snapshot); err == nil {
		t.Fatal("duplicate runnable task accepted")
	}
}

func validOrchestrationSnapshot() OrchestrationSnapshot {
	return OrchestrationSnapshot{
		Version:          OrchestrationModelVersion,
		SessionID:        strings.Repeat("2", 32),
		Spec:             "example",
		Revision:         4,
		Status:           StatusExecuting,
		Phase:            PhaseExecute,
		Gate:             GateNone,
		Runnable:         []OrchestrationTaskSnapshot{},
		ActiveLeases:     []OrchestrationLeaseSnapshot{},
		RecentFailures:   []OrchestrationFailure{},
		SessionExpiresAt: time.Date(2026, 6, 18, 14, 0, 0, 0, time.UTC).Format(time.RFC3339Nano),
	}
}

func validOrchestrationDecision(action OrchestrationAction) OrchestrationDecision {
	decision := OrchestrationDecision{
		Version:        OrchestrationModelVersion,
		Action:         action,
		Spec:           "example",
		Reason:         "deterministic test",
		IdempotencyKey: "revision:4:action:" + string(action),
		Escalation:     OrchestrationEscalation{Code: EscalationNone},
	}
	if action == OrchestrationDispatch || action == OrchestrationRetry || action == OrchestrationCancel {
		decision.TaskID = "T1"
		decision.Attempt = 1
	}
	if action == OrchestrationEscalate {
		decision.Escalation = OrchestrationEscalation{
			Code:    EscalationHumanIntervention,
			Message: "operator review required",
		}
	}
	return decision
}
