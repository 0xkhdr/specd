package core

import "testing"

func TestOrchestrationDecideTable(t *testing.T) {
	policy := validOrchestrationPolicy()
	tests := []struct {
		name   string
		mutate func(*OrchestrationSnapshot)
		want   OrchestrationAction
		task   string
	}{
		{"approval", func(s *OrchestrationSnapshot) { s.Gate = GateAwaitingApproval; s.HumanOnlyGate = true }, OrchestrationRequestApproval, ""},
		{"dispatch", func(s *OrchestrationSnapshot) {
			s.Runnable = []OrchestrationTaskSnapshot{{ID: "T2", Wave: 1, Status: TaskPending, Attempt: 1, Role: "builder", Depends: []string{}, Verified: false}}
		}, OrchestrationDispatch, "T2"},
		{"worker-limit", func(s *OrchestrationSnapshot) {
			s.ActiveLeases = []OrchestrationLeaseSnapshot{{WorkerID: "pinky-a", TaskID: "T1", Attempt: 1, LeaseUntil: s.SessionExpiresAt}}
			policy.MaxWorkers = 1
		}, OrchestrationWait, ""},
		{"complete", func(s *OrchestrationSnapshot) { s.Status = StatusVerifying }, OrchestrationCompleteSession, ""},
		{"blocked", func(s *OrchestrationSnapshot) { s.Status = StatusBlocked }, OrchestrationEscalate, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			policy = validOrchestrationPolicy()
			snapshot := validOrchestrationSnapshot()
			tt.mutate(&snapshot)
			first, err := DecideOrchestration(snapshot, policy)
			if err != nil {
				t.Fatal(err)
			}
			second, err := DecideOrchestration(snapshot, policy)
			if err != nil {
				t.Fatal(err)
			}
			if first != second {
				t.Fatalf("decision not stable:\n%#v\n%#v", first, second)
			}
			if first.Action != tt.want || first.TaskID != tt.task {
				t.Fatalf("decision = %#v, want action %s task %q", first, tt.want, tt.task)
			}
		})
	}
}

// TestOrchestrationCostLimitEscalates proves GAP-4: once cumulative
// host-reported cost reaches the limit, the next decision escalates instead of
// dispatching runnable work.
func TestOrchestrationCostLimitEscalates(t *testing.T) {
	policy := validOrchestrationPolicy()
	policy.HostReportedCostLimitUSD = 5.0

	snapshot := validOrchestrationSnapshot()
	snapshot.Runnable = []OrchestrationTaskSnapshot{{ID: "T2", Wave: 1, Status: TaskPending, Attempt: 1, Role: "builder", Depends: []string{}}}

	// Under the limit: dispatch proceeds.
	snapshot.AccumulatedCostUSD = 4.99
	under, err := DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if under.Action != OrchestrationDispatch {
		t.Fatalf("under limit action = %s, want dispatch", under.Action)
	}

	// At/over the limit: escalate, no dispatch.
	snapshot.AccumulatedCostUSD = 5.0
	over, err := DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if over.Action != OrchestrationEscalate || over.Escalation.Code != EscalationPolicyViolation {
		t.Fatalf("over limit decision = %#v, want escalate/policy-violation", over)
	}

	// Limit of 0 disables enforcement.
	policy.HostReportedCostLimitUSD = 0
	snapshot.AccumulatedCostUSD = 9999
	disabled, err := DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if disabled.Action != OrchestrationDispatch {
		t.Fatalf("disabled limit action = %s, want dispatch", disabled.Action)
	}
}

// TestOrchestrationSessionTimeoutEscalates proves GAP-4: an expired session
// forces a terminal escalation rather than dispatching more work.
func TestOrchestrationSessionTimeoutEscalates(t *testing.T) {
	policy := validOrchestrationPolicy()
	snapshot := validOrchestrationSnapshot()
	snapshot.Runnable = []OrchestrationTaskSnapshot{{ID: "T2", Wave: 1, Status: TaskPending, Attempt: 1, Role: "builder", Depends: []string{}}}
	snapshot.SessionExpired = true

	decision, err := DecideOrchestration(snapshot, policy)
	if err != nil {
		t.Fatal(err)
	}
	if decision.Action != OrchestrationEscalate || decision.Escalation.Code != EscalationPolicyViolation {
		t.Fatalf("expired session decision = %#v, want escalate/policy-violation", decision)
	}
}

func TestParseHostCostUSD(t *testing.T) {
	cases := map[string]float64{
		"":          0,
		"$1.50":     1.5,
		"2.00":      2,
		"$1,234.50": 1234.5,
		"  $3 ":     3, //nolint:gocritic // intentional surrounding whitespace exercises cost-string trimming
		"garbage":   0,
		"-5":        0,
		"NaN":       0,
	}
	for in, want := range cases {
		if got := parseHostCostUSD(in); got != want {
			t.Fatalf("parseHostCostUSD(%q) = %v, want %v", in, got, want)
		}
	}
}

func validOrchestrationPolicy() OrchestrationPolicy {
	policy, err := NewOrchestrationPolicy(DefaultConfig.Orchestration)
	if err != nil {
		panic(err)
	}
	return policy
}
