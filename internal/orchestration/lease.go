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
