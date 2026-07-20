package orchestration

import (
	"strings"
	"testing"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

func activeLease(taskID string) Lease {
	return Lease{
		LeaseID: "lease-" + taskID, MissionID: "m-" + taskID, TaskID: taskID,
		WorkerID: "worker-1", State: LeaseActive, Attempt: 1,
		ExpiresAt: time.Now().Add(time.Hour),
	}
}

// R6.1: a lease binds to the driver session that governs its task.
func TestSessionLeaseBindingAttachesSession(t *testing.T) {
	bound, err := BindLeaseToSession(activeLease("T1"), "ds-1")
	if err != nil {
		t.Fatalf("bind: %v", err)
	}
	if bound.DriverSessionID != "ds-1" {
		t.Fatalf("driver_session_id = %q, want ds-1", bound.DriverSessionID)
	}
	if err := ValidateLeaseSession(bound, "ds-1"); err != nil {
		t.Fatalf("bound session rejected by its own lease: %v", err)
	}
}

// R6.1: a second session cannot take over live work by rebinding the lease.
func TestSessionLeaseBindingRefusesRebindToDifferentSession(t *testing.T) {
	bound, err := BindLeaseToSession(activeLease("T1"), "ds-1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = BindLeaseToSession(bound, "ds-2")
	if err == nil {
		t.Fatal("a second session rebound a lease already held by another")
	}
	if refusal, ok := core.AsRefusal(err); !ok || refusal.Code != "LEASE_SESSION_CONFLICT" {
		t.Fatalf("got %v, want LEASE_SESSION_CONFLICT", err)
	}
	// Rebinding to the same session is idempotent: a retry must not be a conflict.
	if _, err := BindLeaseToSession(bound, "ds-1"); err != nil {
		t.Fatalf("idempotent rebind refused: %v", err)
	}
}

// R6.1: an operation under the wrong session is refused.
func TestSessionLeaseBindingRefusesForeignSession(t *testing.T) {
	bound, _ := BindLeaseToSession(activeLease("T1"), "ds-1")
	err := ValidateLeaseSession(bound, "ds-other")
	if err == nil {
		t.Fatal("a foreign session acted under a bound lease")
	}
	refusal, ok := core.AsRefusal(err)
	if !ok || refusal.Code != "LEASE_SESSION_MISMATCH" {
		t.Fatalf("got %v, want LEASE_SESSION_MISMATCH", err)
	}
	if refusal.RecoveryCommand == "" {
		t.Fatalf("refusal names no recovery: %+v", refusal)
	}
}

// An unbound lease predates session binding and stays usable, so upgrading does
// not invalidate sessions already in flight.
func TestSessionLeaseBindingUnboundLeaseStaysValid(t *testing.T) {
	if err := ValidateLeaseSession(activeLease("T1"), "ds-anything"); err != nil {
		t.Fatalf("unbound lease refused a session: %v", err)
	}
	if err := ValidateLeaseSession(activeLease("T1"), ""); err != nil {
		t.Fatalf("unbound lease refused an empty session: %v", err)
	}
}

func TestSessionLeaseBindingRefusesEmptySession(t *testing.T) {
	if _, err := BindLeaseToSession(activeLease("T1"), ""); err == nil {
		t.Fatal("bound a lease to an empty session id")
	}
}

// R6.2: overlapping scope is detected before dispatch, so the conflicting
// mission is never minted.
func TestSessionLeaseBindingPreDispatchDetectsOverlap(t *testing.T) {
	missions := []MissionV1{{MissionID: "m-T1", TaskID: "T1", DeclaredFiles: []string{"internal/core/thing.go"}}}
	leases := []Lease{activeLease("T1")}

	err := PreDispatchConflict("T2", []string{"internal/core/thing.go"}, missions, leases, time.Now())
	if err == nil {
		t.Fatal("overlapping dispatch was allowed")
	}
	refusal, ok := core.AsRefusal(err)
	if !ok || refusal.Code != "WRITE_SCOPE_CONFLICT" {
		t.Fatalf("got %v, want WRITE_SCOPE_CONFLICT", err)
	}
	if !strings.Contains(err.Error(), "T1") || !strings.Contains(err.Error(), "T2") {
		t.Errorf("refusal does not name both tasks: %v", err)
	}
}

func TestSessionLeaseBindingPreDispatchAllowsDisjointScope(t *testing.T) {
	missions := []MissionV1{{MissionID: "m-T1", TaskID: "T1", DeclaredFiles: []string{"internal/core/thing.go"}}}
	leases := []Lease{activeLease("T1")}

	if err := PreDispatchConflict("T2", []string{"internal/cmd/other.go"}, missions, leases, time.Now()); err != nil {
		t.Fatalf("disjoint scope refused: %v", err)
	}
}

// A task's own prior lease is not a competitor; treating it as one would
// deadlock every retry.
func TestSessionLeaseBindingPreDispatchIgnoresOwnLease(t *testing.T) {
	missions := []MissionV1{{MissionID: "m-T1", TaskID: "T1", DeclaredFiles: []string{"internal/core/thing.go"}}}
	leases := []Lease{activeLease("T1")}

	if err := PreDispatchConflict("T1", []string{"internal/core/thing.go"}, missions, leases, time.Now()); err != nil {
		t.Fatalf("task blocked by its own lease: %v", err)
	}
}

// A released or expired lease holds nothing.
func TestSessionLeaseBindingPreDispatchIgnoresDeadLeases(t *testing.T) {
	missions := []MissionV1{{MissionID: "m-T1", TaskID: "T1", DeclaredFiles: []string{"internal/core/thing.go"}}}
	now := time.Now()

	expired := activeLease("T1")
	expired.ExpiresAt = now.Add(-time.Minute)
	revoked := activeLease("T1")
	revoked.State = LeaseRevoked

	for name, lease := range map[string]Lease{"expired": expired, "revoked": revoked} {
		t.Run(name, func(t *testing.T) {
			if err := PreDispatchConflict("T2", []string{"internal/core/thing.go"}, missions, []Lease{lease}, now); err != nil {
				t.Fatalf("%s lease blocked a dispatch: %v", name, err)
			}
		})
	}
}

// R6.3: a report produced against a baseline the mission did not pin is stale.
func TestSessionLeaseBindingRejectsStaleMissionReport(t *testing.T) {
	now := time.Now()
	mission := MissionV1{MissionID: "m-T1", TaskID: "T1", Attempt: 1, Role: "craftsman", SubjectHead: "abc123"}
	lease := Lease{LeaseID: "lease-1", MissionID: "m-T1", TaskID: "T1", WorkerID: "worker-1",
		Attempt: 1, State: LeaseActive, ExpiresAt: now.Add(time.Hour)}
	report := WorkerReportV1{
		MissionID: "m-T1", LeaseID: "lease-1", WorkerID: "worker-1", TaskID: "T1",
		Attempt: 1, Role: "craftsman", SubjectHead: "abc123", VerifyRef: "ref", Status: "complete",
	}

	if err := ValidateWorkerReport(report, mission, lease, now); err != nil {
		t.Fatalf("matching report refused: %v", err)
	}

	// The worker reports against a head the mission never pinned.
	drifted := report
	drifted.SubjectHead = "def456"
	err := ValidateWorkerReport(drifted, mission, lease, now)
	if err == nil {
		t.Fatal("report against a drifted baseline was accepted")
	}
	if !strings.Contains(err.Error(), "REPORT_BASELINE_STALE") {
		t.Fatalf("got %v, want REPORT_BASELINE_STALE", err)
	}
}
