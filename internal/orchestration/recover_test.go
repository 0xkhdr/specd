package orchestration

import (
	"testing"
	"time"
)

// TestBrainResume covers the pure resume reconciliation: PlanResume decides
// whether a checkpointed dispatch must be re-issued, left alone, or refused as a
// conflict, and DeriveStatus reports a crashed session when the checkpoint outran
// the ledger (spec 07 R3/R4/R6).
func TestBrainResume(t *testing.T) {
	cp := Checkpoint{SessionID: "demo", Step: 2, MissionID: "demo.s2.T3", TaskID: "T3"}

	t.Run("no_checkpoint_is_a_noop", func(t *testing.T) {
		plan := PlanResume(Checkpoint{}, false, nil)
		if plan.Reissue || plan.Conflict != "" {
			t.Fatalf("expected noop, got %+v", plan)
		}
	})

	t.Run("mission_absent_from_ledger_reissues", func(t *testing.T) {
		plan := PlanResume(cp, true, nil)
		if !plan.Reissue || plan.Conflict != "" {
			t.Fatalf("expected reissue, got %+v", plan)
		}
		if plan.Mission.MissionID != cp.MissionID {
			t.Fatalf("reissue names wrong mission: %+v", plan.Mission)
		}
	})

	t.Run("mission_present_in_ledger_is_a_noop", func(t *testing.T) {
		events := []ACPEvent{{Kind: ACPKindDispatch, MissionID: cp.MissionID, TaskID: cp.TaskID}}
		plan := PlanResume(cp, true, events)
		if plan.Reissue || plan.Conflict != "" {
			t.Fatalf("expected noop, got %+v", plan)
		}
	})

	t.Run("task_disagreement_refuses_as_conflict", func(t *testing.T) {
		events := []ACPEvent{{Kind: ACPKindDispatch, MissionID: cp.MissionID, TaskID: "T9"}}
		plan := PlanResume(cp, true, events)
		if plan.Conflict == "" {
			t.Fatalf("expected conflict, got %+v", plan)
		}
	})

	t.Run("checkpoint_outran_ledger_reports_crashed", func(t *testing.T) {
		got := DeriveStatus(Session{}, cp, true, nil)
		if got != SessionCrashed {
			t.Fatalf("expected crashed, got %q", got)
		}
	})

	t.Run("mission_recorded_reports_running", func(t *testing.T) {
		events := []ACPEvent{{Kind: ACPKindDispatch, MissionID: cp.MissionID, TaskID: cp.TaskID}}
		got := DeriveStatus(Session{}, cp, true, events)
		if got != SessionRunning {
			t.Fatalf("expected running, got %q", got)
		}
	})

	t.Run("terminal_session_reports_its_persisted_state", func(t *testing.T) {
		got := DeriveStatus(Session{State: SessionCancelled}, cp, true, nil)
		if got != SessionCancelled {
			t.Fatalf("expected cancelled, got %q", got)
		}
	})
}

// TestLeaseExclusive pins the lease-liveness discriminator that gates a second
// start/resume: a lease inside its TTL is live (blocks), an expired lease is not
// (recoverable by resume) (spec 07 R5).
func TestLeaseExclusive(t *testing.T) {
	now := time.Date(2026, 7, 5, 12, 0, 0, 0, time.UTC)

	t.Run("lease_within_ttl_is_live", func(t *testing.T) {
		leases := []Lease{{TaskID: "T1", WorkerID: "brain", ExpiresAt: now.Add(time.Minute)}}
		if !HasLiveLease(leases, now) {
			t.Fatal("expected live lease")
		}
	})

	t.Run("expired_lease_is_not_live", func(t *testing.T) {
		leases := []Lease{{TaskID: "T1", WorkerID: "brain", ExpiresAt: now.Add(-time.Minute)}}
		if HasLiveLease(leases, now) {
			t.Fatal("expected expired lease to be recoverable")
		}
	})

	t.Run("no_leases_is_not_live", func(t *testing.T) {
		if HasLiveLease(nil, now) {
			t.Fatal("expected no live lease")
		}
	})
}

// TestCrashInjection drives the two kill points around the write-ahead dispatch
// and asserts both converge on resume with exactly one dispatch of the mission —
// no double-dispatch (spec 07 R1, T7). The ledger append is modelled by
// AppendDispatch's in-memory contract: a re-issue of an already-present mission
// is a noop; a re-issue of an absent one records it exactly once.
func TestCrashInjection(t *testing.T) {
	cp := Checkpoint{SessionID: "demo", Step: 1, MissionID: "demo.s1.T1", TaskID: "T1"}

	// applyResume mimics the command path: plan, then re-issue only when the plan
	// says to, appending idempotently (the duplicate guard is the real ledger's).
	applyResume := func(events []ACPEvent) []ACPEvent {
		plan := PlanResume(cp, true, events)
		if plan.Conflict != "" {
			t.Fatalf("unexpected conflict: %s", plan.Conflict)
		}
		if plan.Reissue && !HasMission(events, cp.MissionID) {
			events = append(events, ACPEvent{Kind: ACPKindDispatch, MissionID: cp.MissionID, TaskID: cp.TaskID})
		}
		return events
	}

	t.Run("kill_post_checkpoint_pre_ledger_reissues_once", func(t *testing.T) {
		var ledger []ACPEvent // dispatch never reached the ledger
		if DeriveStatus(Session{}, cp, true, ledger) != SessionCrashed {
			t.Fatal("expected crashed before resume")
		}
		ledger = applyResume(ledger)
		ledger = applyResume(ledger) // a second racing resume must not double-dispatch
		if n := countMission(ledger, cp.MissionID); n != 1 {
			t.Fatalf("expected exactly one dispatch, got %d", n)
		}
	})

	t.Run("kill_post_ledger_does_not_reissue", func(t *testing.T) {
		ledger := []ACPEvent{{Kind: ACPKindDispatch, MissionID: cp.MissionID, TaskID: cp.TaskID}}
		if DeriveStatus(Session{}, cp, true, ledger) != SessionRunning {
			t.Fatal("expected running after ledger append")
		}
		ledger = applyResume(ledger)
		if n := countMission(ledger, cp.MissionID); n != 1 {
			t.Fatalf("expected exactly one dispatch, got %d", n)
		}
	})
}

func countMission(events []ACPEvent, missionID string) int {
	n := 0
	for _, e := range events {
		if e.MissionID == missionID {
			n++
		}
	}
	return n
}
