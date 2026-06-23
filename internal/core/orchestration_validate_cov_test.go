package core

import (
	"math"
	"strings"
	"testing"
	"time"
)

// orchestration_validate_cov_test.go drives the field-level error branches of
// ValidateOrchestrationSnapshot/Decision/Session by mutating one field of a
// known-valid value at a time.

func validOrchestrationSessionForTest(t *testing.T) OrchestrationSession {
	t.Helper()
	policy, err := NewOrchestrationPolicy(DefaultConfig.Orchestration)
	if err != nil {
		t.Fatal(err)
	}
	created := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	return OrchestrationSession{
		Version:   OrchestrationModelVersion,
		SessionID: strings.Repeat("2", 32),
		Spec:      "example",
		Owner:     "cli",
		Status:    OrchestrationSessionRunning,
		Policy:    policy,
		CreatedAt: created.Format(time.RFC3339Nano),
		UpdatedAt: created.Format(time.RFC3339Nano),
		ExpiresAt: orchestrationSessionExpiry(created, policy).Format(time.RFC3339Nano),
	}
}

func TestValidateOrchestrationSnapshotBranches(t *testing.T) {
	if err := ValidateOrchestrationSnapshot(validOrchestrationSnapshot()); err != nil {
		t.Fatalf("valid snapshot rejected: %v", err)
	}
	cases := map[string]func(s *OrchestrationSnapshot){
		"bad version":  func(s *OrchestrationSnapshot) { s.Version = 0 },
		"bad session":  func(s *OrchestrationSnapshot) { s.SessionID = "short" },
		"bad spec":     func(s *OrchestrationSnapshot) { s.Spec = "../x" },
		"neg revision": func(s *OrchestrationSnapshot) { s.Revision = -1 },
		"bad status":   func(s *OrchestrationSnapshot) { s.Status = "mystery" },
		"bad phase":    func(s *OrchestrationSnapshot) { s.Phase = "mystery" },
		"bad gate":     func(s *OrchestrationSnapshot) { s.Gate = "mystery" },
		"bad expires":  func(s *OrchestrationSnapshot) { s.SessionExpiresAt = "nope" },
		"nan cost":     func(s *OrchestrationSnapshot) { s.AccumulatedCostUSD = math.NaN() },
		"inf cost":     func(s *OrchestrationSnapshot) { s.AccumulatedCostUSD = math.Inf(1) },
		"neg cost":     func(s *OrchestrationSnapshot) { s.AccumulatedCostUSD = -1 },
		"bad runnable id": func(s *OrchestrationSnapshot) {
			s.Runnable = []OrchestrationTaskSnapshot{{ID: "bad id", Wave: 1, Attempt: 1, Status: TaskPending, Role: "builder"}}
		},
		"bad runnable role": func(s *OrchestrationSnapshot) {
			s.Runnable = []OrchestrationTaskSnapshot{{ID: "T1", Wave: 1, Attempt: 1, Status: TaskPending, Role: "wizard"}}
		},
		"dup dependency": func(s *OrchestrationSnapshot) {
			s.Runnable = []OrchestrationTaskSnapshot{{ID: "T1", Wave: 1, Attempt: 1, Status: TaskPending, Role: "builder", Depends: []string{"T2", "T2"}}}
		},
		"bad dependency": func(s *OrchestrationSnapshot) {
			s.Runnable = []OrchestrationTaskSnapshot{{ID: "T1", Wave: 1, Attempt: 1, Status: TaskPending, Role: "builder", Depends: []string{"bad dep"}}}
		},
		"bad lease task": func(s *OrchestrationSnapshot) {
			s.ActiveLeases = []OrchestrationLeaseSnapshot{{WorkerID: "w1", TaskID: "bad", Attempt: 1, LeaseUntil: s.SessionExpiresAt}}
		},
		"dup worker": func(s *OrchestrationSnapshot) {
			s.ActiveLeases = []OrchestrationLeaseSnapshot{
				{WorkerID: "w1", TaskID: "T1", Attempt: 1, LeaseUntil: s.SessionExpiresAt},
				{WorkerID: "w1", TaskID: "T2", Attempt: 1, LeaseUntil: s.SessionExpiresAt},
			}
		},
		"bad failure": func(s *OrchestrationSnapshot) {
			s.RecentFailures = []OrchestrationFailure{{TaskID: "T1", Attempt: 1, Kind: "", Message: ""}}
		},
		"human gate mismatch": func(s *OrchestrationSnapshot) {
			s.HumanOnlyGate = true
			s.Gate = GateNone
		},
	}
	for name, mutate := range cases {
		s := validOrchestrationSnapshot()
		mutate(&s)
		if err := ValidateOrchestrationSnapshot(s); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}

func TestValidateOrchestrationDecisionBranches(t *testing.T) {
	if err := ValidateOrchestrationDecision(validOrchestrationDecision(OrchestrationIdle)); err != nil {
		t.Fatalf("valid decision rejected: %v", err)
	}
	cases := map[string]func(d *OrchestrationDecision){
		"bad version": func(d *OrchestrationDecision) { d.Version = 0 },
		"bad action":  func(d *OrchestrationDecision) { d.Action = "invent" },
		"bad spec":    func(d *OrchestrationDecision) { d.Spec = "../x" },
		"bad taskID":  func(d *OrchestrationDecision) { d.TaskID = "bad id" },
		"attempt without task": func(d *OrchestrationDecision) {
			d.Attempt = 2
			d.TaskID = ""
		},
		"artifact on non-author": func(d *OrchestrationDecision) { d.Artifact = "design.md" },
		"empty reason":           func(d *OrchestrationDecision) { d.Reason = "" },
		"empty idempotency":      func(d *OrchestrationDecision) { d.IdempotencyKey = "" },
		"bad escalation code": func(d *OrchestrationDecision) {
			d.Escalation.Code = "mystery"
		},
		"non-escalate with code": func(d *OrchestrationDecision) {
			d.Escalation.Code = EscalationHumanIntervention
		},
	}
	for name, mutate := range cases {
		d := validOrchestrationDecision(OrchestrationIdle)
		mutate(&d)
		if err := ValidateOrchestrationDecision(d); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}

	// Escalate without a code / message.
	esc := validOrchestrationDecision(OrchestrationEscalate)
	esc.Escalation.Code = EscalationNone
	if err := ValidateOrchestrationDecision(esc); err == nil {
		t.Error("escalate without code should error")
	}
	esc = validOrchestrationDecision(OrchestrationEscalate)
	esc.Escalation.Message = ""
	if err := ValidateOrchestrationDecision(esc); err == nil {
		t.Error("escalate without message should error")
	}

	// Author decision missing artifact.
	author := validOrchestrationDecision(OrchestrationDispatchAuthor)
	author.Artifact = ""
	if err := ValidateOrchestrationDecision(author); err == nil {
		t.Error("author without artifact should error")
	}
}

func TestValidateOrchestrationSessionBranches(t *testing.T) {
	if err := ValidateOrchestrationSession(validOrchestrationSessionForTest(t)); err != nil {
		t.Fatalf("valid session rejected: %v", err)
	}
	cases := map[string]func(s *OrchestrationSession){
		"bad version": func(s *OrchestrationSession) { s.Version = 0 },
		"bad session": func(s *OrchestrationSession) { s.SessionID = "short" },
		"bad spec":    func(s *OrchestrationSession) { s.Spec = "../x" },
		"empty owner": func(s *OrchestrationSession) { s.Owner = "" },
		"bad status":  func(s *OrchestrationSession) { s.Status = "mystery" },
		"bad created": func(s *OrchestrationSession) { s.CreatedAt = "nope" },
		"bad updated": func(s *OrchestrationSession) { s.UpdatedAt = "nope" },
		"bad expires": func(s *OrchestrationSession) { s.ExpiresAt = "nope" },
		"updated<created": func(s *OrchestrationSession) {
			s.UpdatedAt = "2020-01-01T00:00:00Z"
		},
		"expires<=created": func(s *OrchestrationSession) {
			s.ExpiresAt = s.CreatedAt
		},
	}
	for name, mutate := range cases {
		s := validOrchestrationSessionForTest(t)
		mutate(&s)
		if err := ValidateOrchestrationSession(s); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}
}
