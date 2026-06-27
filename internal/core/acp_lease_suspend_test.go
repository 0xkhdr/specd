package core

import (
	"errors"
	"strings"
	"testing"
	"time"
)

const suspendTestSession = "22222222222222222222222222222222"

func claimForSuspend(t *testing.T, store *ACPStore, now time.Time) ACPLease {
	t.Helper()
	lease, err := store.ClaimLease(
		suspendTestSession, "worker-1", "example", "T1", 1,
		time.Minute, now.Add(time.Hour),
	)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	return lease
}

// A suspended lease must survive past its original LeaseUntil without becoming
// reclaimable, then resume cleanly to an active lease — the core no-false-fail
// guarantee (Req 1, 4).
func TestSuspendLeaseHoldsThroughWindowThenResumes(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	lease := claimForSuspend(t, store, now)
	suspended, err := store.SuspendLease(
		lease.SessionID, lease.WorkerID, 1, "rate-limit",
		90*time.Second, 10*time.Second, 600*time.Second,
	)
	if err != nil {
		t.Fatal(err)
	}
	if suspended.Status != ACPLeaseSuspended || suspended.ResumeDeadline == "" {
		t.Fatalf("suspended lease = %#v", suspended)
	}

	// Past the original 60s lease window, but inside the 100s resume window.
	now = now.Add(80 * time.Second)
	reloaded, err := store.LoadLease(lease.SessionID, lease.WorkerID)
	if err != nil {
		t.Fatal(err)
	}
	if leaseIsReclaimable(reloaded, now) {
		t.Fatal("suspended lease within deadline is reclaimable")
	}
	if !leaseHoldsWork(reloaded, now) {
		t.Fatal("suspended lease within deadline does not hold work")
	}

	resumed, dur, err := store.ResumeLease(lease.SessionID, lease.WorkerID, 1, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if resumed.Status != ACPLeaseActive {
		t.Fatalf("resumed status = %q", resumed.Status)
	}
	if resumed.SuspendedAt != "" || resumed.ResumeDeadline != "" || resumed.SuspendReason != "" {
		t.Fatalf("resumed lease retained suspend metadata: %#v", resumed)
	}
	if resumed.LeaseUntil != now.Add(time.Minute).Format(time.RFC3339Nano) {
		t.Fatalf("resumed leaseUntil = %s", resumed.LeaseUntil)
	}
	if dur != 80*time.Second {
		t.Fatalf("suspended duration = %s, want 80s", dur)
	}
}

func TestSuspendLeaseRejectsBadReason(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	lease := claimForSuspend(t, store, now)
	if _, err := store.SuspendLease(
		lease.SessionID, lease.WorkerID, 1, "because", time.Minute, 10*time.Second, 600*time.Second,
	); err == nil || !strings.Contains(err.Error(), "unsupported suspend reason") {
		t.Fatalf("bad reason error = %v", err)
	}
}

// Repeated suspends accumulate toward the cap; the call that would exceed it is
// rejected and the prior suspension stands (Req 2.3, 2.4).
func TestSuspendLeaseEnforcesCumulativeCap(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	lease := claimForSuspend(t, store, now)
	if _, err := store.SuspendLease(
		lease.SessionID, lease.WorkerID, 1, "rate-limit", 60*time.Second, 5*time.Second, 100*time.Second,
	); err != nil {
		t.Fatal(err)
	}
	if _, err := store.SuspendLease(
		lease.SessionID, lease.WorkerID, 1, "rate-limit", 60*time.Second, 5*time.Second, 100*time.Second,
	); err == nil || !strings.Contains(err.Error(), "exceed cap") {
		t.Fatalf("over-cap error = %v", err)
	}
	reloaded, err := store.LoadLease(lease.SessionID, lease.WorkerID)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.SuspendSecondsTotal != 60 {
		t.Fatalf("cumulative suspend = %d, want 60 (over-cap call must not apply)", reloaded.SuspendSecondsTotal)
	}
}

func TestResumeAfterDeadlineRequiresReclaim(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	lease := claimForSuspend(t, store, now)
	if _, err := store.SuspendLease(
		lease.SessionID, lease.WorkerID, 1, "rate-limit", 30*time.Second, 5*time.Second, 600*time.Second,
	); err != nil {
		t.Fatal(err)
	}
	now = now.Add(time.Minute) // past the 35s resume deadline
	if _, _, err := store.ResumeLease(lease.SessionID, lease.WorkerID, 1, time.Minute); !errors.Is(err, errACPLeaseGone) {
		t.Fatalf("post-deadline resume error = %v, want errACPLeaseGone", err)
	}
}

func TestSuspendRequiresOwnership(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	lease := claimForSuspend(t, store, now)
	if _, err := store.SuspendLease(
		lease.SessionID, lease.WorkerID, 2, "rate-limit", time.Minute, 5*time.Second, 600*time.Second,
	); err == nil || !strings.Contains(err.Error(), "stale attempt") {
		t.Fatalf("stale-attempt suspend error = %v", err)
	}
}

// A suspended lease whose ResumeDeadline has passed is reclaimed exactly like an
// expired active lease, restoring the normal retry path (Req 4.2).
func TestReclaimReleasesSuspendedPastDeadline(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	lease := claimForSuspend(t, store, now)
	if _, err := store.SuspendLease(
		lease.SessionID, lease.WorkerID, 1, "rate-limit", 30*time.Second, 5*time.Second, 600*time.Second,
	); err != nil {
		t.Fatal(err)
	}

	// Within the window: nothing reclaimed.
	now = now.Add(20 * time.Second)
	if n, err := ReclaimExpiredLeases(store.paths.root, lease.SessionID); err != nil || n != 0 {
		t.Fatalf("reclaim within window = (%d,%v), want (0,nil)", n, err)
	}

	// Past the window: reclaimed once, suspend metadata dropped.
	now = now.Add(time.Minute)
	if n, err := ReclaimExpiredLeases(store.paths.root, lease.SessionID); err != nil || n != 1 {
		t.Fatalf("reclaim past window = (%d,%v), want (1,nil)", n, err)
	}
	reclaimed, err := store.LoadLease(lease.SessionID, lease.WorkerID)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed.Status != ACPLeaseReleased || reclaimed.ResumeDeadline != "" {
		t.Fatalf("reclaimed lease = %#v", reclaimed)
	}
}
