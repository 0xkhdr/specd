package core

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"
)

// SenseOrchestration builds the current OrchestrationSnapshot for a spec
// session: it loads spec state, computes the runnable frontier and recent
// failures, gathers active (including suspended-but-resumable) worker
// leases, surfaces host-reported cost and session-expiry signals, carries
// forward compaction-ledger state, and — when checkpointing is enabled —
// attaches any resumable checkpoints, before validating the result.
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
			// Surface both heartbeating and suspended-within-window leases as
			// in-flight so a rate-limited worker counts toward MaxWorkers and its
			// task is never offered to a fresh worker (R3, Req 4.1). A suspended
			// lease reports its ResumeDeadline as the operative deadline. The
			// decision stays pure: suspension enters here, never via a clock read
			// inside DecideOrchestration.
			suspended := leaseIsSuspendedActive(lease, now)
			if !leaseIsActive(lease, now) && !suspended {
				continue
			}
			deadline := lease.LeaseUntil
			if suspended {
				deadline = lease.ResumeDeadline
			}
			snapshot.ActiveLeases = append(snapshot.ActiveLeases, OrchestrationLeaseSnapshot{
				WorkerID:   lease.WorkerID,
				TaskID:     lease.Task,
				Attempt:    lease.Attempt,
				LeaseUntil: deadline,
				Suspended:  suspended,
			})
		}
	}
	sort.Slice(snapshot.ActiveLeases, func(i, j int) bool {
		if snapshot.ActiveLeases[i].TaskID != snapshot.ActiveLeases[j].TaskID {
			return taskOrdinalLess(snapshot.ActiveLeases[i].TaskID, snapshot.ActiveLeases[j].TaskID)
		}
		return snapshot.ActiveLeases[i].WorkerID < snapshot.ActiveLeases[j].WorkerID
	})
	// Surface the newest progress-report time among in-flight workers so the
	// driver can weight stall waits without reading the clock in the pure
	// decision (R6). Best-effort: an unreadable history simply leaves it empty,
	// preserving today's unweighted behavior.
	snapshot.MostRecentProgressAt = senseMostRecentProgress(store, sessionID, snapshot.ActiveLeases)
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
	// Surface mid-task checkpoints so DecideOrchestration can prefer resume over a
	// fresh dispatch — but only under the resilience gate, keeping the disabled
	// path byte-identical to today (Req 6.3). The decision stays pure: checkpoint
	// state enters here, never via a clock read inside Decide.
	if policy.CheckpointEnabled {
		records, err := loadSessionCheckpoints(root, sessionID)
		if err != nil {
			return OrchestrationSnapshot{}, err
		}
		for _, rec := range records {
			snapshot.Checkpoints = append(snapshot.Checkpoints, OrchestrationCheckpointSnapshot{
				TaskID:          rec.TaskID,
				Attempt:         rec.Attempt,
				ProgressPercent: rec.ProgressPercent,
			})
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

// senseMostRecentProgress returns the newest server-side progress-report time
// (RFC3339Nano) among workers that currently hold an active lease, or "" when no
// in-flight worker has reported. It reads the persisted progress events' stamped
// LastReport (falling back to the envelope CreatedAt for pre-resilience records),
// so the value is a host-stamped time the worker cannot spoof. Best-effort: an
// unreadable history yields "".
func senseMostRecentProgress(store *ACPStore, sessionID string, leases []OrchestrationLeaseSnapshot) string {
	if len(leases) == 0 {
		return ""
	}
	inflight := make(map[string]struct{}, len(leases))
	for _, lease := range leases {
		inflight["pinky-"+lease.WorkerID] = struct{}{}
	}
	events, err := store.readAllEvents(sessionID)
	if err != nil {
		return ""
	}
	var newest time.Time
	out := ""
	for _, event := range events {
		if event.Type != ACPMessageProgress {
			continue
		}
		if _, ok := inflight[event.From]; !ok {
			continue
		}
		reported := event.CreatedAt
		var payload ACPProgressPayload
		if err := json.Unmarshal(event.Payload, &payload); err == nil && payload.LastReport != "" {
			reported = payload.LastReport
		}
		ts, err := parseACPTime("lastReport", reported)
		if err != nil {
			continue
		}
		if out == "" || ts.After(newest) {
			newest, out = ts, reported
		}
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
