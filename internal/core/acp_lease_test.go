package core

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestACPLeaseClaimRaceHasOneWinner(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	const workers = 16
	var wg sync.WaitGroup
	var mu sync.Mutex
	winners := []ACPLease{}
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			lease, err := store.ClaimLease(
				strings.Repeat("2", 32),
				"worker-"+string(rune('a'+n)),
				"example",
				"T1",
				1,
				2*time.Minute,
				now.Add(time.Hour),
			)
			if err == nil {
				mu.Lock()
				winners = append(winners, lease)
				mu.Unlock()
			}
		}(i)
	}
	wg.Wait()
	if len(winners) != 1 {
		t.Fatalf("claim winners = %d, want 1", len(winners))
	}
}

func TestACPLeaseHeartbeatAndRelease(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	lease, err := store.ClaimLease(
		strings.Repeat("2", 32), "worker-1", "example", "T1", 1,
		time.Minute, now.Add(10*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(30 * time.Second)
	renewed, err := store.RenewLease(lease.SessionID, lease.WorkerID, lease.Attempt, 2*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if renewed.HeartbeatAt != now.Format(time.RFC3339Nano) ||
		renewed.LeaseUntil != now.Add(2*time.Minute).Format(time.RFC3339Nano) {
		t.Fatalf("renewed lease = %#v", renewed)
	}
	if err := store.ReleaseLease(lease.SessionID, lease.WorkerID, lease.Attempt); err != nil {
		t.Fatal(err)
	}
	if err := store.ReleaseLease(lease.SessionID, lease.WorkerID, lease.Attempt); err != nil {
		t.Fatalf("idempotent release failed: %v", err)
	}
	if err := store.ValidateActiveLease(lease.SessionID, lease.WorkerID, "example", "T1", 1); err == nil ||
		!strings.Contains(err.Error(), "released") {
		t.Fatalf("released lease validation error = %v", err)
	}
}

func TestACPLeaseExpiryAndReclaim(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	first, err := store.ClaimLease(
		strings.Repeat("2", 32), "worker-1", "example", "T1", 1,
		time.Minute, now.Add(10*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if err := store.ValidateActiveLease(first.SessionID, first.WorkerID, "example", "T1", 1); err == nil ||
		!strings.Contains(err.Error(), "expired") {
		t.Fatalf("expired validation error = %v", err)
	}
	if _, err := store.RenewLease(first.SessionID, first.WorkerID, 1, time.Minute); err == nil {
		t.Fatal("expired worker renewed its lease")
	}
	if _, err := store.ClaimLease(
		first.SessionID, "worker-2", "example", "T1", 1,
		time.Minute, now.Add(10*time.Minute),
	); err == nil || !strings.Contains(err.Error(), "want 2") {
		t.Fatalf("stale reclaim error = %v, want attempt 2", err)
	}
	second, err := store.ClaimLease(
		first.SessionID, "worker-2", "example", "T1", 2,
		time.Minute, now.Add(10*time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateActiveLease(second.SessionID, second.WorkerID, "example", "T1", 2); err != nil {
		t.Fatal(err)
	}
	if err := store.ValidateActiveLease(first.SessionID, first.WorkerID, "example", "T1", 1); err == nil {
		t.Fatal("stale worker retained terminal-report authority")
	}
}

func TestACPLeaseTTLBoundsHeartbeat(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	expiry := now.Add(90 * time.Second)
	lease, err := store.ClaimLease(
		strings.Repeat("2", 32), "worker-1", "example", "T1", 1,
		5*time.Minute, expiry,
	)
	if err != nil {
		t.Fatal(err)
	}
	if lease.LeaseUntil != expiry.Format(time.RFC3339Nano) {
		t.Fatalf("leaseUntil = %s, want message expiry %s", lease.LeaseUntil, expiry.Format(time.RFC3339Nano))
	}
	now = expiry
	if _, err := store.RenewLease(lease.SessionID, lease.WorkerID, 1, time.Minute); err == nil {
		t.Fatal("worker renewed after message TTL")
	}
	if err := store.ValidateActiveLease(lease.SessionID, lease.WorkerID, "example", "T1", 1); err == nil {
		t.Fatal("terminal report accepted after message TTL")
	}
}

func TestACPLeaseRejectsCorruptAndStaleOwnership(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	lease, err := store.ClaimLease(
		strings.Repeat("2", 32), "worker-1", "example", "T1", 1,
		time.Minute, now.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.RenewLease(lease.SessionID, lease.WorkerID, 2, time.Minute); err == nil ||
		!strings.Contains(err.Error(), "stale attempt") {
		t.Fatalf("stale heartbeat error = %v", err)
	}
	path, err := store.paths.LeasePath(lease.SessionID, lease.WorkerID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.LoadLease(lease.SessionID, lease.WorkerID); err == nil ||
		!strings.Contains(err.Error(), "corrupt lease") {
		t.Fatalf("corrupt lease error = %v", err)
	}
}

func TestACPLeaseWorkerIdentityCannotEraseAttemptHistory(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	lease, err := store.ClaimLease(
		strings.Repeat("2", 32), "worker-1", "example", "T1", 1,
		time.Minute, now.Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ReleaseLease(lease.SessionID, lease.WorkerID, lease.Attempt); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimLease(
		lease.SessionID, lease.WorkerID, "example", "T2", 1,
		time.Minute, now.Add(time.Hour),
	); err == nil || !strings.Contains(err.Error(), "already used") {
		t.Fatalf("worker reuse error = %v, want history-preserving rejection", err)
	}
}

func setCoreClock(clock func() time.Time) func() {
	previous := Clock
	Clock = clock
	return func() { Clock = previous }
}
