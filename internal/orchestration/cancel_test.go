package orchestration

import (
	"testing"
	"time"
)

func TestCancelAndRetryPolicy(t *testing.T) {
	now := time.Date(2026, 7, 12, 12, 0, 0, 0, time.UTC)
	l := Lease{LeaseID: "l1", State: LeaseActive, ExpiresAt: now.Add(time.Minute), Retries: 0}
	next, changed, err := CancelLease(l, "operator", now)
	if err != nil || !changed || next.State != LeaseRevoked || next.RevocationReason != "operator" {
		t.Fatalf("cancel failed: lease=%+v changed=%v err=%v", next, changed, err)
	}
	again, changed, err := CancelLease(next, "operator", now)
	if err != nil || changed || again.State != LeaseRevoked {
		t.Fatalf("cancel not idempotent: lease=%+v changed=%v err=%v", again, changed, err)
	}
	if got := ClassifyRecovery(next, RetryPolicy{MaxAttempts: 2, EscalationOwner: "human"}); !got.Retry || got.Action != RecoveryRetry {
		t.Fatalf("expected retry, got %+v", got)
	}
	next.Retries = 2
	if got := ClassifyRecovery(next, RetryPolicy{MaxAttempts: 2, EscalationOwner: "human"}); got.Action != RecoveryEscalate || got.Owner != "human" {
		t.Fatalf("expected escalation, got %+v", got)
	}
}
