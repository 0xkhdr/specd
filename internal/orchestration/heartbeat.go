package orchestration

import (
	"fmt"
	"time"
)

type HeartbeatV1 struct {
	LeaseID   string    `json:"lease_id"`
	MissionID string    `json:"mission_id"`
	WorkerID  string    `json:"worker_id"`
	Attempt   int       `json:"attempt"`
	At        time.Time `json:"at"`
}

func RenewLease(l Lease, h HeartbeatV1, extension, maxLifetime time.Duration) (Lease, error) {
	if l.State != LeaseActive || h.LeaseID != l.LeaseID || h.MissionID != l.MissionID || h.WorkerID != l.WorkerID || h.Attempt != l.Attempt {
		return Lease{}, fmt.Errorf("HEARTBEAT_LEASE_MISMATCH")
	}
	if h.At.IsZero() || !h.At.Before(l.ExpiresAt) || extension <= 0 || maxLifetime <= 0 {
		return Lease{}, fmt.Errorf("HEARTBEAT_LEASE_EXPIRED")
	}
	next := h.At.Add(extension)
	ceiling := l.IssuedAt.Add(maxLifetime)
	if next.After(ceiling) {
		next = ceiling
	}
	if !next.After(h.At) {
		return Lease{}, fmt.Errorf("HEARTBEAT_RENEWAL_LIMIT")
	}
	l.ExpiresAt = next
	return l, nil
}
