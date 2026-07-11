package orchestration

import (
	"testing"
	"time"
)

func TestHeartbeatRenewsMatchingLiveLease(t *testing.T) {
	now := time.Now().UTC()
	l := Lease{LeaseID: "l1", MissionID: "m1", TaskID: "T1", Attempt: 1, WorkerID: "w1", IssuedAt: now.Add(-time.Minute), ExpiresAt: now.Add(time.Minute), PolicyDigest: "p", State: LeaseActive}
	got, err := RenewLease(l, HeartbeatV1{LeaseID: "l1", MissionID: "m1", WorkerID: "w1", Attempt: 1, At: now}, 2*time.Minute, 5*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ExpiresAt.Equal(now.Add(2 * time.Minute)) {
		t.Fatalf("expiry = %s", got.ExpiresAt)
	}
	if _, err := RenewLease(l, HeartbeatV1{LeaseID: "wrong", MissionID: "m1", WorkerID: "w1", Attempt: 1, At: now}, time.Minute, 5*time.Minute); err == nil {
		t.Fatal("wrong lease renewed")
	}
}
