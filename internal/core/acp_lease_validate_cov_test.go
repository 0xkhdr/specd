package core

import (
	"testing"
	"time"
)

// acp_lease_validate_cov_test.go drives validateACPLeaseInput and
// validateACPLease error branches via single-field mutation of a known-valid
// lease — the lock/round-trip tests only ever construct valid leases.

func validACPLeaseForTest() ACPLease {
	base := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	return ACPLease{
		Version:          1,
		SessionID:        "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		WorkerID:         "worker-1",
		Spec:             "example",
		Task:             "T1",
		Attempt:          1,
		Status:           ACPLeaseActive,
		AcquiredAt:       base.Format(time.RFC3339Nano),
		HeartbeatAt:      base.Add(time.Minute).Format(time.RFC3339Nano),
		LeaseUntil:       base.Add(2 * time.Minute).Format(time.RFC3339Nano),
		MessageExpiresAt: base.Add(time.Hour).Format(time.RFC3339Nano),
	}
}

func TestValidateACPLeaseInputBranches(t *testing.T) {
	if err := validateACPLeaseInput("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "worker-1", "example", "T1", 1, time.Minute); err != nil {
		t.Fatalf("valid input rejected: %v", err)
	}
	cases := []struct {
		name                        string
		session, worker, spec, task string
		attempt                     int
		dur                         time.Duration
	}{
		{"bad session", "short", "worker-1", "example", "T1", 1, time.Minute},
		{"bad worker", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "../bad", "example", "T1", 1, time.Minute},
		{"bad spec", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "worker-1", "../x", "T1", 1, time.Minute},
		{"bad task", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "worker-1", "example", "bad id", 1, time.Minute},
		{"bad attempt", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "worker-1", "example", "T1", 0, time.Minute},
		{"bad duration", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", "worker-1", "example", "T1", 1, 0},
	}
	for _, c := range cases {
		if err := validateACPLeaseInput(c.session, c.worker, c.spec, c.task, c.attempt, c.dur); err == nil {
			t.Errorf("%s: expected error", c.name)
		}
	}
}

func TestValidateACPLeaseBranches(t *testing.T) {
	valid := validACPLeaseForTest()
	if err := validateACPLease(valid, valid.SessionID, valid.WorkerID); err != nil {
		t.Fatalf("valid lease rejected: %v", err)
	}
	cases := map[string]func(l *ACPLease){
		"bad version":    func(l *ACPLease) { l.Version = 2 },
		"bad input":      func(l *ACPLease) { l.Task = "bad id" },
		"bad status":     func(l *ACPLease) { l.Status = "mystery" },
		"bad acquired":   func(l *ACPLease) { l.AcquiredAt = "nope" },
		"bad heartbeat":  func(l *ACPLease) { l.HeartbeatAt = "nope" },
		"bad until":      func(l *ACPLease) { l.LeaseUntil = "nope" },
		"bad msg expiry": func(l *ACPLease) { l.MessageExpiresAt = "nope" },
		"heartbeat before acquired": func(l *ACPLease) {
			l.HeartbeatAt = "2020-01-01T00:00:00Z"
		},
		"until before heartbeat": func(l *ACPLease) {
			l.LeaseUntil = l.HeartbeatAt
		},
		"until after msg expiry": func(l *ACPLease) {
			l.MessageExpiresAt = l.HeartbeatAt
		},
		"active has released": func(l *ACPLease) {
			l.ReleasedAt = l.LeaseUntil
		},
	}
	for name, mutate := range cases {
		l := validACPLeaseForTest()
		mutate(&l)
		if err := validateACPLease(l, l.SessionID, l.WorkerID); err == nil {
			t.Errorf("%s: expected error", name)
		}
	}

	// Identity mismatch: caller-supplied session/worker differs from the lease.
	if err := validateACPLease(valid, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb", valid.WorkerID); err == nil {
		t.Error("session identity mismatch should error")
	}

	// Released lease: releasedAt must not precede acquiredAt.
	rel := validACPLeaseForTest()
	rel.Status = ACPLeaseReleased
	rel.ReleasedAt = "2020-01-01T00:00:00Z"
	if err := validateACPLease(rel, rel.SessionID, rel.WorkerID); err == nil {
		t.Error("released-before-acquired should error")
	}
	// Valid released lease.
	rel2 := validACPLeaseForTest()
	rel2.Status = ACPLeaseReleased
	rel2.ReleasedAt = rel2.LeaseUntil
	if err := validateACPLease(rel2, rel2.SessionID, rel2.WorkerID); err != nil {
		t.Errorf("valid released lease rejected: %v", err)
	}
}

func TestValidateLeaseOwnershipBranches(t *testing.T) {
	now := time.Date(2026, 6, 18, 12, 1, 0, 0, time.UTC)
	lease := validACPLeaseForTest()
	if err := validateLeaseOwnership(lease, "worker-1", 1, now); err != nil {
		t.Fatalf("valid ownership rejected: %v", err)
	}
	if err := validateLeaseOwnership(lease, "other", 1, now); err == nil {
		t.Error("wrong worker should error")
	}
	if err := validateLeaseOwnership(lease, "worker-1", 2, now); err == nil {
		t.Error("stale attempt should error")
	}
	released := lease
	released.Status = ACPLeaseReleased
	if err := validateLeaseOwnership(released, "worker-1", 1, now); err == nil {
		t.Error("released lease should error")
	}
	// Expired: now past LeaseUntil.
	expired := time.Date(2026, 6, 18, 13, 0, 0, 0, time.UTC)
	if err := validateLeaseOwnership(lease, "worker-1", 1, expired); err == nil {
		t.Error("expired lease should error")
	}
}
