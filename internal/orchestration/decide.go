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
)

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

	frontier := append([]core.FrontierTask(nil), snapshot.Frontier...)
	sort.SliceStable(frontier, func(i, j int) bool {
		return frontier[i].ID < frontier[j].ID
	})
	return Decision{Action: ActionDispatch, TaskID: frontier[0].ID, Reason: "frontier ready"}
}
