package orchestration

import (
	"fmt"
	"time"
)

type RecoveryAction string

const (
	RecoveryRetry    RecoveryAction = "retry"
	RecoveryEscalate RecoveryAction = "escalate"
)

type RetryPolicy struct {
	MaxAttempts     int
	EscalationOwner string
}

type RecoveryDecision struct {
	Action RecoveryAction
	Retry  bool
	Owner  string
	Reason string
}

func CancelLease(l Lease, reason string, now time.Time) (Lease, bool, error) {
	if reason == "" || now.IsZero() {
		return l, false, fmt.Errorf("CANCEL_REASON_OR_TIME_REQUIRED")
	}
	if l.State == LeaseRevoked {
		if l.RevocationReason != reason {
			return l, false, fmt.Errorf("CANCEL_REASON_CONFLICT")
		}
		return l, false, nil
	}
	if l.State != LeaseActive && l.State != "" {
		return l, false, fmt.Errorf("CANCEL_LEASE_NOT_ACTIVE")
	}
	l.State = LeaseRevoked
	l.RevocationReason = reason
	return l, true, nil
}

func ClassifyRecovery(l Lease, p RetryPolicy) RecoveryDecision {
	if p.MaxAttempts > 0 && l.Retries < p.MaxAttempts {
		return RecoveryDecision{Action: RecoveryRetry, Retry: true, Reason: l.RevocationReason}
	}
	return RecoveryDecision{Action: RecoveryEscalate, Owner: p.EscalationOwner, Reason: "retry limit exceeded"}
}
