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
	// ActionWaitApproval is the explicit lifecycle stop (R4.1): execution is
	// finished and the spec cannot advance until someone approves. It is
	// separate from ActionWait because the two need different answers — a wait
	// clears when workers report, this one clears only when a human or an
	// operator-authorized grant acts.
	ActionWaitApproval Action = "waiting_approval"
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
	Action  Action
	TaskID  string
	Reason  string
	Handoff *ApprovalHandoff
}

// ApprovalHandoff is the lifecycle approval a controller run is blocked on. The
// caller resolves it — reading gates, state, and any grant an operator named —
// and hands it in; this package only reports it. That split is what keeps R4.4
// structural: the controller has no code that mints, widens, or spends a grant,
// because it never holds one. It names the route and stops.
type ApprovalHandoff struct {
	// Gate is the lifecycle gate awaiting approval.
	Gate string
	// Route is the exact command that unblocks the run, and Actor is who may
	// run it. A delegated route is named only when an operator supplied a grant
	// that already covers this transition — naming one is not issuing one.
	Route string
	Actor string
	// Blocked is true when the readiness gates currently refuse. Then no
	// approval route works, delegated or not, and the route names the check
	// instead of pretending an approval would land (R4.3 gate drift).
	Blocked bool
}

func (h ApprovalHandoff) Reason() string {
	if h.Blocked {
		return "waiting_approval: lifecycle gate " + h.Gate + " cannot be approved by anyone while its readiness gates refuse; run `" + h.Route + "`"
	}
	return "waiting_approval: lifecycle gate " + h.Gate + " requires " + h.Actor + " approval; run `" + h.Route + "`"
}

type DecisionLimits struct {
	Deadline         time.Time
	MaxRetries       int
	AllowDispatch    bool
	MaxCostMicros    int64
	MaxTokens        int64
	RequireTelemetry bool
	Workers          WorkerPresence
	// Approval is set only when execution has nothing left to run and the spec
	// is sitting at a lifecycle gate. Nil keeps today's frontier wait.
	Approval *ApprovalHandoff
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
		// An empty frontier because the work is done and the lifecycle is
		// blocked is a different stop from an empty frontier because no task is
		// ready yet, and the operator needs a different lever for each (R4.1).
		if limits.Approval != nil {
			return Decision{Action: ActionWaitApproval, Reason: limits.Approval.Reason(), Handoff: limits.Approval}
		}
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
