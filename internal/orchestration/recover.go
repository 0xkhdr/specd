package orchestration

import (
	"fmt"
	"time"
)

// ResumePlan is the reconciliation of the write-ahead checkpoint against the ACP
// ledger (spec 07 R3/R4). Exactly one of the outcomes holds:
//   - Conflict != "": the checkpoint and ledger disagree irreconcilably; resume
//     must refuse (exit 1) rather than guess.
//   - Reissue == true: the checkpoint's mission never reached the ledger (crash
//     between checkpoint and dispatch); resume re-issues exactly that dispatch.
//   - both zero: the checkpoint's mission is already in the ledger (or there is
//     no outstanding mission); resume only reconstructs session state.
type ResumePlan struct {
	Reissue  bool
	Mission  Checkpoint
	Conflict string
}

// PlanResume computes the resume plan from the checkpoint and the ledger. It is
// pure: the caller supplies the loaded checkpoint (with existence flag) and the
// ledger events.
func PlanResume(cp Checkpoint, cpExists bool, events []ACPEvent) ResumePlan {
	if !cpExists || cp.MissionID == "" {
		return ResumePlan{}
	}
	event, ok := MissionEvent(events, cp.MissionID)
	if !ok {
		// Checkpoint outran the ledger: the dispatch was made durable in the
		// checkpoint but the process died before appending it. Re-issue it.
		return ResumePlan{Reissue: true, Mission: cp}
	}
	if event.TaskID != cp.TaskID {
		return ResumePlan{Conflict: fmt.Sprintf("mission %s names task %q in the ledger but %q in the checkpoint", cp.MissionID, event.TaskID, cp.TaskID)}
	}
	// Already applied — nothing to re-issue.
	return ResumePlan{}
}

// DeriveStatus is the effective status reported by `brain status`. A terminal
// session reports its persisted state. A running session whose checkpoint names
// a mission missing from the ledger is reported as crashed: the process died
// mid-dispatch and a resume is needed to converge (spec 07 R6).
func DeriveStatus(session Session, cp Checkpoint, cpExists bool, events []ACPEvent) SessionState {
	if session.IsTerminal() {
		return session.Status()
	}
	if cpExists && cp.MissionID != "" && !HasMission(events, cp.MissionID) {
		return SessionCrashed
	}
	return SessionRunning
}

// HasLiveLease reports whether any lease is still within its TTL at now. A live
// lease means another controller is actively holding work and blocks a second
// start/resume; only expired/orphaned leases are recoverable by resume (R5).
func HasLiveLease(leases []Lease, now time.Time) bool {
	for _, lease := range leases {
		if now.Before(lease.ExpiresAt) {
			return true
		}
	}
	return false
}
