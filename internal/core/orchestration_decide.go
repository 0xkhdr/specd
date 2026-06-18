package core

import (
	"fmt"
	"sort"
)

func DecideOrchestration(snapshot OrchestrationSnapshot, policy OrchestrationPolicy) (OrchestrationDecision, error) {
	if err := ValidateOrchestrationPolicy(policy); err != nil {
		return OrchestrationDecision{}, err
	}
	if err := ValidateOrchestrationSnapshot(snapshot); err != nil {
		return OrchestrationDecision{}, err
	}
	decision := OrchestrationDecision{
		Version:    OrchestrationModelVersion,
		Spec:       snapshot.Spec,
		Escalation: OrchestrationEscalation{Code: EscalationNone},
	}
	switch {
	case snapshot.Status == StatusComplete:
		decision.Action = OrchestrationIdle
		decision.Reason = "spec already complete"
	case snapshot.Status == StatusBlocked:
		decision.Action = OrchestrationEscalate
		decision.Escalation = OrchestrationEscalation{Code: EscalationHumanIntervention, Message: "spec blocked"}
		decision.Reason = "spec is blocked"
	case snapshot.Gate == GateAwaitingApproval:
		decision.Action = OrchestrationRequestApproval
		decision.Reason = "awaiting human approval"
	case len(snapshot.RecentFailures) > 0 && retryableFailuresExhausted(snapshot.RecentFailures, policy.MaxRetries):
		decision.Action = OrchestrationEscalate
		decision.Escalation = OrchestrationEscalation{Code: EscalationRetriesExhausted, Message: "retry limit reached"}
		decision.Reason = "retry limit reached"
	case len(snapshot.ActiveLeases) >= policy.MaxWorkers:
		decision.Action = OrchestrationWait
		decision.Reason = "worker limit reached"
	case len(snapshot.Runnable) > 0:
		task := firstUnleasedRunnable(snapshot.Runnable, snapshot.ActiveLeases)
		if task.ID == "" {
			decision.Action = OrchestrationWait
			decision.Reason = "runnable work already leased"
			break
		}
		decision.Action = OrchestrationDispatch
		decision.TaskID = task.ID
		decision.Attempt = task.Attempt
		decision.Reason = "dispatch next runnable task"
	case snapshot.Status == StatusVerifying:
		decision.Action = OrchestrationCompleteSession
		decision.Reason = "no runnable tasks and spec verifying"
	default:
		decision.Action = OrchestrationWait
		decision.Reason = "no runnable work"
	}
	decision.IdempotencyKey = fmt.Sprintf("%s:%d:%s:%s:%d", snapshot.SessionID, snapshot.Revision, decision.Action, decision.TaskID, decision.Attempt)
	if err := ValidateOrchestrationDecision(decision); err != nil {
		return OrchestrationDecision{}, err
	}
	return decision, nil
}

func retryableFailuresExhausted(failures []OrchestrationFailure, maxRetries int) bool {
	for _, failure := range failures {
		if failure.Retryable && failure.Attempt <= maxRetries {
			return false
		}
	}
	return len(failures) > 0
}

func firstUnleasedRunnable(tasks []OrchestrationTaskSnapshot, leases []OrchestrationLeaseSnapshot) OrchestrationTaskSnapshot {
	leased := map[string]bool{}
	for _, lease := range leases {
		leased[lease.TaskID] = true
	}
	candidates := append([]OrchestrationTaskSnapshot{}, tasks...)
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Wave != candidates[j].Wave {
			return candidates[i].Wave < candidates[j].Wave
		}
		return taskOrdinalLess(candidates[i].ID, candidates[j].ID)
	})
	for _, task := range candidates {
		if !leased[task.ID] {
			return task
		}
	}
	return OrchestrationTaskSnapshot{}
}
