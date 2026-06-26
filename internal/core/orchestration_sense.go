package core

import (
	"fmt"
	"sort"
	"time"
)

func SenseOrchestration(root, slug, sessionID string, policy OrchestrationPolicy) (OrchestrationSnapshot, error) {
	loaded, err := LoadSpec(root, slug)
	if err != nil {
		return OrchestrationSnapshot{}, err
	}
	expires := Clock().UTC().Add(time.Duration(policy.SessionTimeoutSeconds) * time.Second)
	snapshot := OrchestrationSnapshot{
		Version:          OrchestrationModelVersion,
		SessionID:        sessionID,
		Spec:             slug,
		Revision:         loaded.State.Revision,
		Status:           loaded.State.Status,
		Phase:            loaded.State.Phase,
		Gate:             loaded.State.Gate,
		HumanOnlyGate:    loaded.State.Gate == GateAwaitingApproval,
		Runnable:         senseRunnableTasks(loaded.State),
		ActiveLeases:     []OrchestrationLeaseSnapshot{},
		RecentFailures:   senseRecentFailures(loaded.State),
		SessionExpiresAt: expires.Format(time.RFC3339Nano),
	}
	snapshot.Authoring, snapshot.PlanningReady = senseAuthoring(
		loaded.State.Status,
		ReadArtifact(root, slug, "requirements.md"),
		ReadArtifact(root, slug, "design.md"),
		loaded.Doc,
	)
	store, err := NewACPStore(root)
	if err != nil {
		return OrchestrationSnapshot{}, err
	}
	leases, err := store.loadSessionLeases(sessionID)
	if err == nil {
		now := Clock().UTC()
		for _, lease := range leases {
			if leaseIsActive(lease, now) {
				snapshot.ActiveLeases = append(snapshot.ActiveLeases, OrchestrationLeaseSnapshot{
					WorkerID:   lease.WorkerID,
					TaskID:     lease.Task,
					Attempt:    lease.Attempt,
					LeaseUntil: lease.LeaseUntil,
				})
			}
		}
	}
	sort.Slice(snapshot.ActiveLeases, func(i, j int) bool {
		if snapshot.ActiveLeases[i].TaskID != snapshot.ActiveLeases[j].TaskID {
			return taskOrdinalLess(snapshot.ActiveLeases[i].TaskID, snapshot.ActiveLeases[j].TaskID)
		}
		return snapshot.ActiveLeases[i].WorkerID < snapshot.ActiveLeases[j].WorkerID
	})
	cost, err := senseHostReportedCost(root, sessionID)
	if err != nil {
		return OrchestrationSnapshot{}, err
	}
	snapshot.AccumulatedCostUSD = cost
	expired, err := senseSessionExpired(root, sessionID)
	if err != nil {
		return OrchestrationSnapshot{}, err
	}
	snapshot.SessionExpired = expired
	// Carry the ledger tail + last compaction step from the persisted session so
	// DecideOrchestration can evaluate the compaction triggers while staying a pure
	// function of (snapshot, policy). Absent a session there is nothing to compact.
	if session, ok, err := loadOrchestrationSessionIfExists(root, sessionID); err != nil {
		return OrchestrationSnapshot{}, err
	} else if ok {
		snapshot.LastCompactionStep = session.LastCompactionStep
		if tail, has := lastContextLedgerEntry(session); has {
			snapshot.LedgerEstimatedTokens = tail.EstimatedTokens
			snapshot.LedgerBudget = tail.Budget
		}
	}
	if err := ValidateOrchestrationSnapshot(snapshot); err != nil {
		return OrchestrationSnapshot{}, err
	}
	return snapshot, nil
}

func senseRunnableTasks(state *State) []OrchestrationTaskSnapshot {
	frontier := RunnableFrontier(DagTasksFromState(state))
	out := make([]OrchestrationTaskSnapshot, 0, len(frontier))
	for _, task := range frontier {
		full := state.Tasks[task.ID]
		attempt := 1
		if full.Telemetry != nil {
			attempt += full.Telemetry.Retries
		}
		out = append(out, OrchestrationTaskSnapshot{
			ID:       full.ID,
			Wave:     full.Wave,
			Status:   full.Status,
			Attempt:  attempt,
			Role:     full.Role,
			Depends:  append([]string{}, full.Depends...),
			Verified: full.Verification != nil && full.Verification.Verified,
		})
	}
	return out
}

func senseRecentFailures(state *State) []OrchestrationFailure {
	failures := make([]OrchestrationFailure, 0)
	for _, task := range state.Tasks {
		if task.Verification == nil || task.Verification.Verified {
			continue
		}
		attempt := 1
		if task.Telemetry != nil {
			attempt += task.Telemetry.Retries
		}
		failures = append(failures, OrchestrationFailure{
			TaskID:    task.ID,
			Attempt:   attempt,
			Kind:      "verify-failed",
			Message:   fmt.Sprintf("exit %d: %s", task.Verification.ExitCode, task.Verification.Command),
			Retryable: true,
		})
	}
	sort.Slice(failures, func(i, j int) bool {
		return taskOrdinalLess(failures[i].TaskID, failures[j].TaskID)
	})
	return failures
}
