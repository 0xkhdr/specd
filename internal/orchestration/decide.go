package orchestration

import (
	"sort"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

type Action string

const (
	ActionWait     Action = "wait"
	ActionDispatch Action = "dispatch"
	ActionHalt     Action = "halt"
	ActionTimeout  Action = "timeout"
	ActionEscalate Action = "escalate"
)

type WorkerState string

const (
	WorkerStateActive  WorkerState = "active"
	WorkerStateExpired WorkerState = "expired"
)

func LeaseWorkerState(lease Lease, now time.Time) WorkerState {
	if now.Before(lease.ExpiresAt) {
		return WorkerStateActive
	}
	return WorkerStateExpired
}

// Wait reasons are distinct per blocking condition (spec R3.1/R3.2): each
// states why the loop is waiting and names the command that unblocks it.
const (
	ReasonWaitAuthorityAbsent = "waiting: dispatch authority absent; grant it with `specd brain run <slug> --authority`"
	ReasonWaitFrontierEmpty   = "waiting: frontier empty (no task has all dependencies resolved); inspect with `specd status <slug> --guide`"
	ReasonWaitNoWorker        = "waiting: no worker definition for active harness; repair with `specd init --repair`"
)

// WorkerPresence keeps harness-specific filesystem checks outside the pure
// decision package. Callers inject a deterministic snapshot/probe result.
type WorkerPresence interface {
	WorkerAvailable() bool
}

type Decision struct {
	Action Action
	TaskID string
	Reason string
}

type DecisionLimits struct {
	Deadline         time.Time
	MaxRetries       int
	AllowDispatch    bool
	MaxCostMicros    int64
	MaxTokens        int64
	RequireTelemetry bool
	Workers          WorkerPresence
}

func Decide(snapshot Snapshot, limits DecisionLimits) Decision {
	if brake := EvaluateBrakes(snapshot, limits); brake.Action != "" {
		return brake
	}
	if escalation := Escalation(snapshot.Leases, limits.MaxRetries, snapshot.Now); escalation.TaskID != "" {
		return Decision{Action: ActionEscalate, TaskID: escalation.TaskID, Reason: escalation.Reason}
	}
	// Each wait reason is distinct and names its unblock command (spec R3.1,
	// R3.2), so an operator reading a stalled loop knows which lever to pull.
	// Authority is checked before the frontier: without dispatch authority the
	// frontier's contents are irrelevant. (A third no-worker reason lands later.)
	if !limits.AllowDispatch {
		return Decision{Action: ActionWait, Reason: ReasonWaitAuthorityAbsent}
	}
	if len(snapshot.Frontier) == 0 {
		return Decision{Action: ActionWait, Reason: ReasonWaitFrontierEmpty}
	}
	if limits.Workers != nil && !limits.Workers.WorkerAvailable() {
		return Decision{Action: ActionWait, Reason: ReasonWaitNoWorker}
	}

	frontier := append([]core.FrontierTask(nil), snapshot.Frontier...)
	sort.SliceStable(frontier, func(i, j int) bool {
		return frontier[i].ID < frontier[j].ID
	})
	return Decision{Action: ActionDispatch, TaskID: frontier[0].ID, Reason: "frontier ready"}
}

// ReasonWaitScopeConflict is the wait a task earns when every runnable option
// overlaps live work. Like the other wait reasons it names the lever, which
// here is time rather than a command: the conflict clears when the lease does.
const ReasonWaitScopeConflict = "waiting: every frontier task overlaps an active lease's write scope; wait for the lease to release or inspect with `specd brain status <slug>`"

// PreDispatchConflict reports whether dispatching taskID with the given write
// scope would overlap live work (R6.2).
//
// It runs before dispatch, not at claim. CheckParallelConflict already guards
// the claim, but by then a mission exists and a worker is asking for it — the
// overlap has been created and is merely being refused. Detecting it here means
// the conflicting mission is never minted, so the loser is a task that waits
// rather than a worker that fails.
//
// Callers hold the spec lock, so the lease set cannot change underneath this
// between the check and the dispatch it guards.
func PreDispatchConflict(taskID string, declared []string, missions []MissionV1, leases []Lease, now time.Time) error {
	byID := make(map[string]MissionV1, len(missions))
	for _, mission := range missions {
		byID[mission.MissionID] = mission
	}
	for _, lease := range leases {
		if lease.State != LeaseActive || !now.Before(lease.ExpiresAt) {
			continue
		}
		// A lease on the same task is this task's own prior attempt, not a
		// competitor; refusing it would deadlock every retry.
		if lease.TaskID == taskID {
			continue
		}
		active, ok := byID[lease.MissionID]
		if !ok {
			continue
		}
		if scopesOverlap(declared, active.DeclaredFiles) {
			return core.Refusef("WRITE_SCOPE_CONFLICT", "task %s overlaps the write scope held by task %s under lease %s",
				taskID, active.TaskID, lease.LeaseID)
		}
	}
	return nil
}
