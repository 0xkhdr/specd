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

func validOrchestrationPolicy() OrchestrationPolicy {
	policy, err := NewOrchestrationPolicy(DefaultConfig.Orchestration)
	if err != nil {
		panic(err)
	}
	return policy
}
