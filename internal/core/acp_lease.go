package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"time"
)

type ACPLeaseStatus string

const (
	ACPLeaseActive   ACPLeaseStatus = "active"
	ACPLeaseReleased ACPLeaseStatus = "released"
)

var errACPLeaseNotFound = errors.New("acp lease not found")

type ACPLease struct {
	Version          int            `json:"version"`
	SessionID        string         `json:"sessionId"`
	WorkerID         string         `json:"workerId"`
	Spec             string         `json:"spec"`
	Task             string         `json:"task"`
	Attempt          int            `json:"attempt"`
	Status           ACPLeaseStatus `json:"status"`
	AcquiredAt       string         `json:"acquiredAt"`
	HeartbeatAt      string         `json:"heartbeatAt"`
	LeaseUntil       string         `json:"leaseUntil"`
	MessageExpiresAt string         `json:"messageExpiresAt"`
	ReleasedAt       string         `json:"releasedAt,omitempty"`
}

func (s *ACPStore) ClaimLease(
	sessionID, workerID, spec, task string,
	attempt int,
	leaseDuration time.Duration,
	messageExpiresAt time.Time,
) (ACPLease, error) {
	if err := validateACPLeaseInput(sessionID, workerID, spec, task, attempt, leaseDuration); err != nil {
		return ACPLease{}, err
	}
	now := Clock().UTC()
	messageExpiresAt = messageExpiresAt.UTC()
	if !messageExpiresAt.After(now) {
		return ACPLease{}, fmt.Errorf("acp lease: message TTL has expired")
	}

	var claimed ACPLease
	err := s.withSessionLock(sessionID, func() error {
		leases, err := s.loadSessionLeases(sessionID)
		if err != nil {
			return err
		}
		maxAttempt := 0
		for _, lease := range leases {
			if lease.WorkerID == workerID {
				if leaseIsActive(lease, now) {
					return fmt.Errorf("acp lease: worker %s already owns active work", workerID)
				}
				if lease.Spec != spec || lease.Task != task {
					return fmt.Errorf("acp lease: worker identity %s was already used for %s/%s", workerID, lease.Spec, lease.Task)
				}
			}
			if lease.Spec != spec || lease.Task != task {
				continue
			}
			if lease.Attempt > maxAttempt {
				maxAttempt = lease.Attempt
			}
			if leaseIsActive(lease, now) {
				return fmt.Errorf("acp lease: %s/%s is owned by worker %s", spec, task, lease.WorkerID)
			}
		}
		wantAttempt := maxAttempt + 1
		if attempt != wantAttempt {
			return fmt.Errorf("acp lease: stale or skipped attempt %d, want %d", attempt, wantAttempt)
		}

		leaseUntil := now.Add(leaseDuration)
		if leaseUntil.After(messageExpiresAt) {
			leaseUntil = messageExpiresAt
		}
		claimed = ACPLease{
			Version:          1,
			SessionID:        sessionID,
			WorkerID:         workerID,
			Spec:             spec,
			Task:             task,
			Attempt:          attempt,
			Status:           ACPLeaseActive,
			AcquiredAt:       now.Format(time.RFC3339Nano),
			HeartbeatAt:      now.Format(time.RFC3339Nano),
			LeaseUntil:       leaseUntil.Format(time.RFC3339Nano),
			MessageExpiresAt: messageExpiresAt.Format(time.RFC3339Nano),
		}
		return s.saveLease(claimed)
	})
	return claimed, err
}

func (s *ACPStore) RenewLease(sessionID, workerID string, attempt int, leaseDuration time.Duration) (ACPLease, error) {
	if leaseDuration <= 0 {
		return ACPLease{}, fmt.Errorf("acp lease: lease duration must be positive")
	}
	now := Clock().UTC()
	var renewed ACPLease
	err := s.withSessionLock(sessionID, func() error {
		lease, err := s.loadLease(sessionID, workerID)
		if err != nil {
			return err
		}
		if err := validateLeaseOwnership(lease, workerID, attempt, now); err != nil {
			return err
		}
		messageExpiry, _ := parseACPTime("lease messageExpiresAt", lease.MessageExpiresAt)
		leaseUntil := now.Add(leaseDuration)
		if leaseUntil.After(messageExpiry) {
			leaseUntil = messageExpiry
		}
		if !leaseUntil.After(now) {
			return fmt.Errorf("acp lease: message TTL has expired")
		}
		lease.HeartbeatAt = now.Format(time.RFC3339Nano)
		lease.LeaseUntil = leaseUntil.Format(time.RFC3339Nano)
		if err := s.saveLease(lease); err != nil {
			return err
		}
		renewed = lease
		return nil
	})
	return renewed, err
}

func (s *ACPStore) ReleaseLease(sessionID, workerID string, attempt int) error {
	now := Clock().UTC()
	return s.withSessionLock(sessionID, func() error {
		lease, err := s.loadLease(sessionID, workerID)
		if err != nil {
			return err
		}
		if lease.Status == ACPLeaseReleased && lease.Attempt == attempt {
			return nil
		}
		if err := validateLeaseOwnership(lease, workerID, attempt, now); err != nil {
			return err
		}
		lease.Status = ACPLeaseReleased
		lease.ReleasedAt = now.Format(time.RFC3339Nano)
		return s.saveLease(lease)
	})
}

// ValidateActiveLease is the terminal-report ownership gate. It rejects a
// released, expired, wrong-worker, or stale-attempt lease.
func (s *ACPStore) ValidateActiveLease(sessionID, workerID, spec, task string, attempt int) error {
	lease, err := s.loadLease(sessionID, workerID)
	if err != nil {
		return err
	}
	if lease.Spec != spec || lease.Task != task {
		return fmt.Errorf("acp lease: work identity mismatch")
	}
	return validateLeaseOwnership(lease, workerID, attempt, Clock().UTC())
}

func (s *ACPStore) LoadLease(sessionID, workerID string) (ACPLease, error) {
	return s.loadLease(sessionID, workerID)
}

func (s *ACPStore) loadLease(sessionID, workerID string) (ACPLease, error) {
	path, err := s.paths.LeasePath(sessionID, workerID)
	if err != nil {
		return ACPLease{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ACPLease{}, fmt.Errorf("%w: worker %s", errACPLeaseNotFound, workerID)
	}
	if err != nil {
		return ACPLease{}, fmt.Errorf("acp lease: read lease: %w", err)
	}
	var lease ACPLease
	if err := decodeACPStrict(raw, &lease); err != nil {
		return ACPLease{}, fmt.Errorf("acp lease: corrupt lease: %w", err)
	}
	if err := validateACPLease(lease, sessionID, workerID); err != nil {
		return ACPLease{}, fmt.Errorf("acp lease: corrupt lease: %w", err)
	}
	return lease, nil
}

func (s *ACPStore) loadSessionLeases(sessionID string) ([]ACPLease, error) {
	workersDir, err := s.paths.WorkersDir(sessionID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(workersDir)
	if errors.Is(err, os.ErrNotExist) {
		return []ACPLease{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("acp lease: read workers: %w", err)
	}
	leases := make([]ACPLease, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lease, err := s.loadLease(sessionID, entry.Name())
		if errors.Is(err, errACPLeaseNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		leases = append(leases, lease)
	}
	sort.Slice(leases, func(i, j int) bool {
		if leases[i].Spec != leases[j].Spec {
			return leases[i].Spec < leases[j].Spec
		}
		if leases[i].Task != leases[j].Task {
			return leases[i].Task < leases[j].Task
		}
		if leases[i].Attempt != leases[j].Attempt {
			return leases[i].Attempt < leases[j].Attempt
		}
		return leases[i].WorkerID < leases[j].WorkerID
	})
	return leases, nil
}

func (s *ACPStore) saveLease(lease ACPLease) error {
	if err := validateACPLease(lease, lease.SessionID, lease.WorkerID); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(lease, "", "  ")
	if err != nil {
		return fmt.Errorf("acp lease: encode lease: %w", err)
	}
	raw = append(raw, '\n')
	path, err := s.paths.LeasePath(lease.SessionID, lease.WorkerID)
	if err != nil {
		return err
	}
	if err := atomicWritePrivate(path, raw); err != nil {
		return fmt.Errorf("acp lease: save lease: %w", err)
	}
	return nil
}

func validateACPLeaseInput(
	sessionID, workerID, spec, task string,
	attempt int,
	leaseDuration time.Duration,
) error {
	if err := validateACPOpaqueID("session ID", sessionID); err != nil {
		return err
	}
	if err := validateACPRuntimeSegment("worker ID", workerID); err != nil {
		return err
	}
	if err := ValidateSlug(spec); err != nil {
		return fmt.Errorf("acp lease: invalid spec: %w", err)
	}
	if !acpTaskIDRE.MatchString(task) {
		return fmt.Errorf("acp lease: invalid task")
	}
	if attempt < 1 {
		return fmt.Errorf("acp lease: attempt must be greater than zero")
	}
	if leaseDuration <= 0 {
		return fmt.Errorf("acp lease: lease duration must be positive")
	}
	return nil
}

func validateACPLease(lease ACPLease, sessionID, workerID string) error {
	if lease.Version != 1 {
		return fmt.Errorf("unsupported version %d", lease.Version)
	}
	if lease.SessionID != sessionID || lease.WorkerID != workerID {
		return fmt.Errorf("identity mismatch")
	}
	if err := validateACPLeaseInput(
		lease.SessionID,
		lease.WorkerID,
		lease.Spec,
		lease.Task,
		lease.Attempt,
		time.Second,
	); err != nil {
		return err
	}
	if lease.Status != ACPLeaseActive && lease.Status != ACPLeaseReleased {
		return fmt.Errorf("unsupported status %q", lease.Status)
	}
	acquired, err := parseACPTime("lease acquiredAt", lease.AcquiredAt)
	if err != nil {
		return err
	}
	heartbeat, err := parseACPTime("lease heartbeatAt", lease.HeartbeatAt)
	if err != nil {
		return err
	}
	leaseUntil, err := parseACPTime("lease leaseUntil", lease.LeaseUntil)
	if err != nil {
		return err
	}
	messageExpiry, err := parseACPTime("lease messageExpiresAt", lease.MessageExpiresAt)
	if err != nil {
		return err
	}
	if heartbeat.Before(acquired) || !leaseUntil.After(heartbeat) || leaseUntil.After(messageExpiry) {
		return fmt.Errorf("invalid lease time ordering")
	}
	if lease.Status == ACPLeaseReleased {
		released, err := parseACPTime("lease releasedAt", lease.ReleasedAt)
		if err != nil {
			return err
		}
		if released.Before(acquired) {
			return fmt.Errorf("releasedAt precedes acquiredAt")
		}
	} else if lease.ReleasedAt != "" {
		return fmt.Errorf("active lease has releasedAt")
	}
	return nil
}

func validateLeaseOwnership(lease ACPLease, workerID string, attempt int, now time.Time) error {
	if lease.WorkerID != workerID {
		return fmt.Errorf("acp lease: wrong worker")
	}
	if lease.Attempt != attempt {
		return fmt.Errorf("acp lease: stale attempt %d, current attempt is %d", attempt, lease.Attempt)
	}
	if lease.Status != ACPLeaseActive {
		return fmt.Errorf("acp lease: lease is %s", lease.Status)
	}
	if !leaseIsActive(lease, now) {
		return fmt.Errorf("acp lease: lease has expired")
	}
	return nil
}

func leaseIsActive(lease ACPLease, now time.Time) bool {
	if lease.Status != ACPLeaseActive {
		return false
	}
	leaseUntil, err := time.Parse(time.RFC3339Nano, lease.LeaseUntil)
	if err != nil {
		return false
	}
	messageExpiry, err := time.Parse(time.RFC3339Nano, lease.MessageExpiresAt)
	if err != nil {
		return false
	}
	return now.Before(leaseUntil) && now.Before(messageExpiry)
}
