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
	case EvaluateCostBrake(snapshot.AccumulatedCostUSD, policy.HostReportedCostLimitUSD) == CostBrakeHalt:
		// Host-reported cost is untrusted, but the operator brake still acts:
		// halt-and-escalate rather than dispatch more work (GAP-4/W4).
		decision.Action = OrchestrationEscalate
		decision.Escalation = OrchestrationEscalation{
			Code:    EscalationPolicyViolation,
			Message: fmt.Sprintf("host-reported cost $%.2f reached hard limit $%.2f (advisory, untrusted)", snapshot.AccumulatedCostUSD, policy.HostReportedCostLimitUSD),
		}
		decision.Reason = "host-reported cost hard limit reached"
	case snapshot.SessionExpired:
		// The session's wall-clock deadline forces a terminal decision instead of
		// waiting for leases to expire one by one (GAP-4).
		decision.Action = OrchestrationEscalate
		decision.Escalation = OrchestrationEscalation{Code: EscalationPolicyViolation, Message: "session timeout reached"}
		decision.Reason = "session timeout reached"
	case policy.compactsOnPhase() && compactionPhaseBoundary(snapshot) &&
		snapshot.LastCompactionStep < uint64(snapshot.Revision):
		// A planning ratchet (design→tasks | tasks→executing) or the
		// executing→verifying transition is the natural seam to shed accreted
		// context. The LastCompactionStep guard fires this at most once per
		// boundary; the engine sets it to the post-compaction revision.
		decision.Action = OrchestrationCompact
		decision.Reason = "phase-boundary"
	case policy.compactsOnBudget() && compactionBudgetExceeded(snapshot, policy.CompactionBudgetThreshold):
		// Mid-execution token pressure: the last manifest estimate crossed the
		// budget threshold. The compaction ledger entry resets the estimate, so the
		// trigger settles after one fire rather than looping.
		decision.Action = OrchestrationCompact
		decision.Reason = "budget-threshold"
	case len(snapshot.ActiveLeases) >= policy.MaxWorkers:
		decision.Action = OrchestrationWait
		decision.Reason = "worker limit reached"
	case len(snapshot.Runnable) > 0 && isExecutionStatus(snapshot.Status):
		// The execution frontier only dispatches once the spec is executing.
		// During planning statuses, tasks.md may already be reconciled into
		// state (LoadSpec auto-reconciles), but those tasks must not run before
		// the tasks→executing gate is cleared — authoring/advance branches below
		// own the planning phases.
		task := firstUnleasedRunnable(snapshot.Runnable, snapshot.ActiveLeases)
		if task.ID == "" {
			decision.Action = OrchestrationWait
			decision.Reason = "runnable work already leased"
			break
		}
		decision.TaskID = task.ID
		decision.Attempt = task.Attempt
		// Prefer resuming from a checkpoint of this exact (task, attempt) over a
		// fresh dispatch (Req 4). The attempt-guard is strict equality: a
		// stale-attempt checkpoint (recorded for an earlier, now-superseded
		// attempt) never matches, so retried work restarts clean (Req 6.2).
		if hasResumableCheckpoint(task, snapshot.Checkpoints) {
			decision.Action = OrchestrationResume
			decision.Reason = "resume task from checkpoint"
		} else {
			decision.Action = OrchestrationDispatch
			decision.Reason = "dispatch next runnable task"
		}
	case snapshot.Authoring != nil:
		// Planning-phase artifact is absent or failing its gate. Author it under
		// autonomy; defer to a human under the manual policy. (GAP-1)
		work := snapshot.Authoring
		if !planningAutonomyAllowed(policy.ApprovalPolicy) {
			decision.Action = OrchestrationRequestApproval
			decision.Reason = fmt.Sprintf("manual policy: author %s before continuing", work.Artifact)
			break
		}
		if authoringLeased(work.WorkID, snapshot.ActiveLeases) {
			decision.Action = OrchestrationWait
			decision.Reason = fmt.Sprintf("authoring %s already in flight", work.Artifact)
			break
		}
		decision.Action = OrchestrationDispatchAuthor
		decision.TaskID = work.WorkID
		decision.Attempt = 1
		decision.Artifact = work.Artifact
		decision.Reason = fmt.Sprintf("dispatch authoring mission for %s", work.Artifact)
	case snapshot.PlanningReady:
		// Current planning artifact passes its gate; the phase is ready to
		// advance. Ratchet under autonomy; request human approval under manual.
		artifact := planningArtifactForStatus(snapshot.Status)
		if !planningAutonomyAllowed(policy.ApprovalPolicy) {
			decision.Action = OrchestrationRequestApproval
			decision.Reason = fmt.Sprintf("manual policy: approve %s phase", snapshot.Status)
			break
		}
		decision.Action = OrchestrationAdvancePhase
		decision.Artifact = artifact
		decision.Reason = fmt.Sprintf("%s gate satisfied — advance phase", snapshot.Status)
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

// compactionPhaseBoundary reports whether the snapshot sits at a phase seam
// where stage-aware compaction applies: the design or tasks planning gate is
// satisfied (about to ratchet) or the spec has entered verifying. requirements→
// design is intentionally excluded (too early to have accreted context).
func compactionPhaseBoundary(snapshot OrchestrationSnapshot) bool {
	switch {
	case snapshot.PlanningReady && (snapshot.Status == StatusDesign || snapshot.Status == StatusTasks):
		return true
	case snapshot.Status == StatusVerifying:
		return true
	}
	return false
}

// compactionBudgetExceeded reports whether the ledger tail's estimated tokens
// have reached budget*threshold. A non-positive threshold or budget disables the
// trigger (fail-closed: never compact without a real budget to measure against).
func compactionBudgetExceeded(snapshot OrchestrationSnapshot, threshold float64) bool {
	if threshold <= 0 || snapshot.LedgerBudget <= 0 {
		return false
	}
	return snapshot.LedgerEstimatedTokens >= int(float64(snapshot.LedgerBudget)*threshold)
}

// planningAutonomyAllowed reports whether the approval policy lets the brain
// author planning artifacts and ratchet planning phases without a human. The
// `manual` policy never does; `planning` and `session` do. Mid-requirement
// human-only gates (GateAwaitingApproval) are checked earlier and are never
// auto-cleared by this path.
func planningAutonomyAllowed(approvalPolicy string) bool {
	return approvalPolicy == "planning" || approvalPolicy == "session"
}

// isExecutionStatus reports whether the spec is in a phase where the task DAG
// runs. Planning statuses (requirements/design/tasks) are owned by the authoring
// frontier instead.
func isExecutionStatus(status SpecStatus) bool {
	return status == StatusExecuting || status == StatusBlocked
}

// hasResumableCheckpoint reports whether a checkpoint exists for the task's
// exact current attempt. Matching on attempt (not just task) is the attempt
// guard: it ignores checkpoints left behind by a superseded attempt so a retried
// task is never resumed from stale mid-progress.
func hasResumableCheckpoint(task OrchestrationTaskSnapshot, checkpoints []OrchestrationCheckpointSnapshot) bool {
	for _, cp := range checkpoints {
		if cp.TaskID == task.ID && cp.Attempt == task.Attempt {
			return true
		}
	}
	return false
}

func authoringLeased(workID string, leases []OrchestrationLeaseSnapshot) bool {
	for _, lease := range leases {
		if lease.TaskID == workID {
			return true
		}
	}
	return false
}

func planningArtifactForStatus(status SpecStatus) string {
	if step, ok := authoringSteps[status]; ok {
		return step.Artifact
	}
	return ""
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
