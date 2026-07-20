package orchestration

import (
	"fmt"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

type LeaseState string

const (
	LeaseActive  LeaseState = "active"
	LeaseRevoked LeaseState = "revoked"
	LeaseExpired LeaseState = "expired"
)

type Lease struct {
	LeaseID          string           `json:"lease_id,omitempty"`
	MissionID        string           `json:"mission_id,omitempty"`
	TaskID           string           `json:"task_id"`
	Attempt          int              `json:"attempt,omitempty"`
	WorkerID         string           `json:"worker_id"`
	IssuedAt         time.Time        `json:"issued_at,omitempty"`
	ExpiresAt        time.Time        `json:"expires_at"`
	PolicyDigest     string           `json:"policy_digest,omitempty"`
	DispatchDigest   string           `json:"dispatch_digest,omitempty"`
	State            LeaseState       `json:"state,omitempty"`
	RevocationReason string           `json:"revocation_reason,omitempty"`
	Authority        core.AuthorityV1 `json:"authority"`
	Retries          int              `json:"retries"`

	// DriverSessionID binds the lease to the driver session that governs its
	// task (R6.1). Without it, a lease and a session are two independent
	// authorities over the same work: a session can outlive the lease it was
	// opened for, and a lease can be claimed by a host holding no session at
	// all. Empty means the lease predates session binding, which stays valid so
	// existing sessions are not invalidated by an upgrade.
	DriverSessionID string `json:"driver_session_id,omitempty"`
}

// BindLeaseToSession attaches a driver session to a lease (R6.1). Rebinding a
// lease already bound to a different session is refused: that is either a
// second host claiming live work, or the same host having lost and reopened its
// session, and both need the lease reissued rather than quietly retargeted.
func BindLeaseToSession(lease Lease, sessionID string) (Lease, error) {
	if sessionID == "" {
		return Lease{}, core.Refuse("SESSION_UNKNOWN", "cannot bind a lease to an empty driver session id")
	}
	if lease.DriverSessionID != "" && lease.DriverSessionID != sessionID {
		return Lease{}, core.Refusef("LEASE_SESSION_CONFLICT", "lease %s is already bound to driver session %s", lease.LeaseID, lease.DriverSessionID)
	}
	lease.DriverSessionID = sessionID
	return lease, nil
}

// ValidateLeaseSession checks that an operation carrying sessionID is entitled
// to act under this lease (R6.1). An unbound lease is accepted for backward
// compatibility; a bound one must match exactly.
func ValidateLeaseSession(lease Lease, sessionID string) error {
	if lease.DriverSessionID == "" {
		return nil
	}
	if sessionID != lease.DriverSessionID {
		return core.Refusef("LEASE_SESSION_MISMATCH", "lease %s is bound to driver session %s, not %s", lease.LeaseID, lease.DriverSessionID, sessionID).
			WithRecovery(core.RefusalActorAgent, "specd session open <slug> --driver <host>")
	}
	return nil
}

func ValidateLease(l Lease) error {
	if l.LeaseID == "" || l.MissionID == "" || l.TaskID == "" || l.Attempt < 1 || l.WorkerID == "" || l.WorkerID == "brain" || l.IssuedAt.IsZero() || !l.ExpiresAt.After(l.IssuedAt) || l.PolicyDigest == "" {
		return fmt.Errorf("LEASE_REQUIRED_FIELD_OR_WORKER_INVALID")
	}
	if l.State != LeaseActive && l.State != LeaseRevoked && l.State != LeaseExpired {
		return fmt.Errorf("LEASE_STATE_INVALID")
	}
	if l.State == LeaseRevoked && l.RevocationReason == "" {
		return fmt.Errorf("LEASE_REVOCATION_REASON_REQUIRED")
	}
	if err := core.ValidateAuthority(l.Authority, l.IssuedAt, l.Authority.Phase); err != nil {
		return fmt.Errorf("LEASE_AUTHORITY_INVALID: %w", err)
	}
	return nil
}

type Reclaim struct {
	TaskID string
	Retry  bool
	Reason string
}

func ReclaimExpired(leases []Lease, now time.Time, maxRetries int) []Reclaim {
	var reclaimed []Reclaim
	for _, lease := range leases {
		if now.Before(lease.ExpiresAt) {
			continue
		}
		reclaimed = append(reclaimed, Reclaim{
			TaskID: lease.TaskID,
			Retry:  lease.Retries < maxRetries,
			Reason: "lease expired",
		})
	}
	return reclaimed
}

func Escalation(leases []Lease, maxRetries int, now time.Time) Reclaim {
	for _, reclaim := range ReclaimExpired(leases, now, maxRetries) {
		if !reclaim.Retry {
			reclaim.Reason = "retry limit exceeded"
			return reclaim
		}
	}
	return Reclaim{}
}
