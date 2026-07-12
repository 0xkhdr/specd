package orchestration

import (
	"encoding/json"
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

// ReconcileSession projects a checkpoint and append-only ledger into session
// state. It is pure and idempotent, allowing callers to persist with CAS.
func ReconcileSession(s Session, cp Checkpoint, cpExists bool, events []ACPEvent) (Session, bool, error) {
	plan := PlanResume(cp, cpExists, events)
	if plan.Conflict != "" {
		return s, false, fmt.Errorf("%s", plan.Conflict)
	}
	changed := false
	if cpExists && cp.Mission != nil && !sessionHasMission(s, cp.Mission.MissionID) {
		s.PendingMissions = append(s.PendingMissions, *cp.Mission)
		changed = true
	}
	if cpExists && cp.Step > s.Step {
		s.Step, changed = cp.Step, true
	}
	for _, event := range events {
		switch event.Kind {
		case ACPKindDispatch:
			if event.Payload == "" || sessionHasMission(s, event.MissionID) {
				continue
			}
			var m MissionV1
			if err := json.Unmarshal([]byte(event.Payload), &m); err != nil {
				return s, changed, fmt.Errorf("decode dispatch mission: %w", err)
			}
			s.PendingMissions = append(s.PendingMissions, m)
			changed = true
		case ACPKindClaim:
			if event.Payload == "" || leaseIndex(s.Leases, event.MissionID) >= 0 {
				continue
			}
			var l Lease
			if err := json.Unmarshal([]byte(event.Payload), &l); err != nil {
				return s, changed, fmt.Errorf("decode claim lease: %w", err)
			}
			s.Leases = append(s.Leases, l)
			changed = true
		case ACPKindCancel:
			if event.Payload == "" {
				continue
			}
			var l Lease
			if err := json.Unmarshal([]byte(event.Payload), &l); err != nil {
				return s, changed, fmt.Errorf("decode cancelled lease: %w", err)
			}
			if i := leaseIndex(s.Leases, event.MissionID); i >= 0 && s.Leases[i].State != l.State {
				s.Leases[i] = l
				changed = true
			}
		case ACPKindReport:
			if i := leaseIndex(s.Leases, event.MissionID); i >= 0 {
				s.Leases = append(s.Leases[:i], s.Leases[i+1:]...)
				changed = true
			}
		}
	}
	return s, changed, nil
}

func sessionHasMission(s Session, id string) bool {
	for _, m := range s.PendingMissions {
		if m.MissionID == id {
			return true
		}
	}
	for _, m := range s.Missions {
		if m.MissionID == id {
			return true
		}
	}
	return false
}

func leaseIndex(leases []Lease, missionID string) int {
	for i := range leases {
		if leases[i].MissionID == missionID {
			return i
		}
	}
	return -1
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
		if (lease.State == LeaseActive || lease.State == "") && now.Before(lease.ExpiresAt) {
			return true
		}
	}
	return false
}
