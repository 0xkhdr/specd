package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type ProgramChildLeaseStatus string

const (
	ProgramChildLeaseActive    ProgramChildLeaseStatus = "active"
	ProgramChildLeaseReleased  ProgramChildLeaseStatus = "released"
	ProgramChildLeaseEscalated ProgramChildLeaseStatus = "escalated"
)

type ProgramChildLease struct {
	Version         int                     `json:"version"`
	ParentSessionID string                  `json:"parentSessionId"`
	ChildSessionID  string                  `json:"childSessionId"`
	Slug            string                  `json:"slug"`
	Status          ProgramChildLeaseStatus `json:"status"`
	AcquiredAt      string                  `json:"acquiredAt"`
	LeaseUntil      string                  `json:"leaseUntil"`
	ReleasedAt      string                  `json:"releasedAt,omitempty"`
	EscalatedAt     string                  `json:"escalatedAt,omitempty"`
}

var (
	programChildLeaseLocksMu sync.Mutex
	programChildLeaseLocks   = map[string]*sync.Mutex{}
)

func AcquireProgramChildLease(root, parentSessionID, slug string, cfg OrchestrationCfg) (ProgramChildLease, error) {
	if err := validateACPOpaqueID("parent session ID", parentSessionID); err != nil {
		return ProgramChildLease{}, err
	}
	if err := ValidateSlug(slug); err != nil {
		return ProgramChildLease{}, fmt.Errorf("program orchestration: invalid child slug: %w", err)
	}
	if err := ValidateOrchestrationConfig(&cfg); err != nil {
		return ProgramChildLease{}, err
	}
	var acquired ProgramChildLease
	err := withProgramChildLeaseLock(root, slug, func() error {
		now := Clock().UTC()
		existing, ok, err := loadProgramChildLease(root, slug)
		if err != nil {
			return err
		}
		if ok && programChildLeaseIsActive(existing, now) {
			if existing.ParentSessionID == parentSessionID {
				acquired = existing
				return nil
			}
			return fmt.Errorf("program orchestration: child %s is owned by parent session %s", slug, existing.ParentSessionID)
		}
		childSessionID, err := NewACPID()
		if err != nil {
			return fmt.Errorf("program orchestration: create child session ID: %w", err)
		}
		lease := ProgramChildLease{
			Version:         OrchestrationModelVersion,
			ParentSessionID: parentSessionID,
			ChildSessionID:  childSessionID,
			Slug:            slug,
			Status:          ProgramChildLeaseActive,
			AcquiredAt:      now.Format(time.RFC3339Nano),
			LeaseUntil:      now.Add(time.Duration(cfg.Transport.LeaseSeconds) * time.Second).Format(time.RFC3339Nano),
		}
		if err := saveProgramChildLease(root, lease); err != nil {
			return err
		}
		acquired = lease
		return nil
	})
	return acquired, err
}

func ReleaseProgramChildLease(root, parentSessionID, slug string) (ProgramChildLease, error) {
	if err := validateACPOpaqueID("parent session ID", parentSessionID); err != nil {
		return ProgramChildLease{}, err
	}
	var released ProgramChildLease
	err := withProgramChildLeaseLock(root, slug, func() error {
		lease, ok, err := loadProgramChildLease(root, slug)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("program orchestration: child %s has no lease", slug)
		}
		if lease.ParentSessionID != parentSessionID {
			return fmt.Errorf("program orchestration: child %s is owned by parent session %s", slug, lease.ParentSessionID)
		}
		if lease.Status == ProgramChildLeaseReleased {
			released = lease
			return nil
		}
		lease.Status = ProgramChildLeaseReleased
		lease.ReleasedAt = Clock().UTC().Format(time.RFC3339Nano)
		lease.EscalatedAt = ""
		if err := saveProgramChildLease(root, lease); err != nil {
			return err
		}
		released = lease
		return nil
	})
	return released, err
}

func markProgramChildLeaseEscalated(root, parentSessionID, slug string) (ProgramChildLease, error) {
	var escalated ProgramChildLease
	err := withProgramChildLeaseLock(root, slug, func() error {
		lease, ok, err := loadProgramChildLease(root, slug)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("program orchestration: child %s has no lease", slug)
		}
		if lease.ParentSessionID != parentSessionID {
			return fmt.Errorf("program orchestration: child %s is owned by parent session %s", slug, lease.ParentSessionID)
		}
		if lease.Status == ProgramChildLeaseEscalated {
			escalated = lease
			return nil
		}
		if lease.Status == ProgramChildLeaseReleased {
			return fmt.Errorf("program orchestration: child %s lease already released", slug)
		}
		lease.Status = ProgramChildLeaseEscalated
		lease.EscalatedAt = Clock().UTC().Format(time.RFC3339Nano)
		if err := saveProgramChildLease(root, lease); err != nil {
			return err
		}
		escalated = lease
		return nil
	})
	return escalated, err
}

func LoadProgramChildLeases(root string) ([]ProgramChildLease, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return nil, err
	}
	dir, err := paths.ProgramChildrenDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []ProgramChildLease{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("program orchestration: read child leases: %w", err)
	}
	leases := make([]ProgramChildLease, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		lease, ok, err := loadProgramChildLease(root, entry.Name())
		if err != nil {
			return nil, err
		}
		if ok {
			leases = append(leases, lease)
		}
	}
	sortProgramChildLeases(leases)
	return leases, nil
}

func releaseCompleteProgramChildren(root string, graph ProgramGraph) error {
	complete := make(map[string]bool, len(graph.Specs))
	for _, spec := range graph.Specs {
		complete[spec.Slug] = spec.Complete
	}
	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		return err
	}
	now := Clock().UTC()
	for _, lease := range leases {
		if !complete[lease.Slug] || lease.Status == ProgramChildLeaseReleased {
			continue
		}
		if !programChildLeaseIsActive(lease, now) && lease.Status != ProgramChildLeaseEscalated {
			continue
		}
		if _, err := releaseProgramChildLeaseAnyParent(root, lease.Slug); err != nil {
			return err
		}
	}
	return nil
}

func releaseProgramChildLeaseAnyParent(root, slug string) (ProgramChildLease, error) {
	var released ProgramChildLease
	err := withProgramChildLeaseLock(root, slug, func() error {
		lease, ok, err := loadProgramChildLease(root, slug)
		if err != nil {
			return err
		}
		if !ok {
			return nil
		}
		if lease.Status == ProgramChildLeaseReleased {
			released = lease
			return nil
		}
		lease.Status = ProgramChildLeaseReleased
		lease.ReleasedAt = Clock().UTC().Format(time.RFC3339Nano)
		lease.EscalatedAt = ""
		if err := saveProgramChildLease(root, lease); err != nil {
			return err
		}
		released = lease
		return nil
	})
	return released, err
}

func activeProgramChildren(root string) (map[string]bool, error) {
	runtime, err := programChildRuntime(root)
	if err != nil {
		return nil, err
	}
	active := map[string]bool{}
	for slug, childRuntime := range runtime {
		if childRuntime.Active {
			active[slug] = true
		}
	}
	return active, nil
}

func programChildRuntime(root string) (map[string]ProgramChildRuntime, error) {
	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		return nil, err
	}
	now := Clock().UTC()
	runtime := map[string]ProgramChildRuntime{}
	for _, lease := range leases {
		childRuntime := runtime[lease.Slug]
		childRuntime.ChildSessionID = lease.ChildSessionID
		if programChildLeaseIsActive(lease, now) {
			childRuntime.Active = true
		}
		if lease.Status == ProgramChildLeaseEscalated {
			childRuntime.Escalated = true
		}
		runtime[lease.Slug] = childRuntime
	}
	return runtime, nil
}

func programLeasesToStep(graph ProgramGraph, leases []ProgramChildLease, parentSessionID string, capacity int) []ProgramChildLease {
	now := Clock().UTC()
	waves := make(map[string]int, len(graph.Specs))
	for _, spec := range graph.Specs {
		waves[spec.Slug] = spec.Wave
	}
	out := make([]ProgramChildLease, 0, len(leases))
	for _, lease := range leases {
		if lease.ParentSessionID == parentSessionID && programChildLeaseIsActive(lease, now) {
			out = append(out, lease)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if waves[out[i].Slug] != waves[out[j].Slug] {
			return waves[out[i].Slug] < waves[out[j].Slug]
		}
		return out[i].Slug < out[j].Slug
	})
	if capacity > 0 && len(out) > capacity {
		out = out[:capacity]
	}
	return out
}

func ensureProgramChildSession(root string, lease ProgramChildLease, policy OrchestrationPolicy) error {
	session, err := LoadOrchestrationSession(root, lease.ChildSessionID)
	if errors.Is(err, errOrchestrationSessionNotFound) {
		_, err = StartOrchestrationSession(root, lease.Slug, lease.ChildSessionID, "program:"+lease.ParentSessionID, policy)
		return err
	}
	if err != nil {
		return err
	}
	if session.Spec != lease.Slug {
		return fmt.Errorf("program orchestration: child session %s belongs to %s, not %s", lease.ChildSessionID, session.Spec, lease.Slug)
	}
	return nil
}

func loadProgramChildLease(root, slug string) (ProgramChildLease, bool, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return ProgramChildLease{}, false, err
	}
	path, err := paths.ProgramChildLeasePath(slug)
	if err != nil {
		return ProgramChildLease{}, false, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ProgramChildLease{}, false, nil
	}
	if err != nil {
		return ProgramChildLease{}, false, fmt.Errorf("program orchestration: read child lease: %w", err)
	}
	var lease ProgramChildLease
	if err := decodeACPStrict(raw, &lease); err != nil {
		return ProgramChildLease{}, false, fmt.Errorf("program orchestration: corrupt child lease: %w", err)
	}
	if lease.Slug != slug {
		return ProgramChildLease{}, false, fmt.Errorf("program orchestration: child lease identity mismatch")
	}
	if err := validateProgramChildLease(lease); err != nil {
		return ProgramChildLease{}, false, fmt.Errorf("program orchestration: corrupt child lease: %w", err)
	}
	return lease, true, nil
}

func saveProgramChildLease(root string, lease ProgramChildLease) error {
	if err := validateProgramChildLease(lease); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(lease, "", "  ")
	if err != nil {
		return fmt.Errorf("program orchestration: encode child lease: %w", err)
	}
	raw = append(raw, '\n')
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return err
	}
	path, err := paths.ProgramChildLeasePath(lease.Slug)
	if err != nil {
		return err
	}
	if err := atomicWritePrivate(path, raw); err != nil {
		return fmt.Errorf("program orchestration: save child lease: %w", err)
	}
	return nil
}

func validateProgramChildLease(lease ProgramChildLease) error {
	if lease.Version != OrchestrationModelVersion {
		return fmt.Errorf("unsupported version %d", lease.Version)
	}
	if err := validateACPOpaqueID("parent session ID", lease.ParentSessionID); err != nil {
		return err
	}
	if err := validateACPOpaqueID("child session ID", lease.ChildSessionID); err != nil {
		return err
	}
	if err := ValidateSlug(lease.Slug); err != nil {
		return fmt.Errorf("invalid child slug: %w", err)
	}
	if lease.Status != ProgramChildLeaseActive && lease.Status != ProgramChildLeaseReleased && lease.Status != ProgramChildLeaseEscalated {
		return fmt.Errorf("unsupported status %q", lease.Status)
	}
	acquired, err := parseACPTime("program child lease acquiredAt", lease.AcquiredAt)
	if err != nil {
		return err
	}
	leaseUntil, err := parseACPTime("program child lease leaseUntil", lease.LeaseUntil)
	if err != nil {
		return err
	}
	if !leaseUntil.After(acquired) {
		return fmt.Errorf("invalid lease time ordering")
	}
	switch lease.Status {
	case ProgramChildLeaseReleased:
		released, err := parseACPTime("program child lease releasedAt", lease.ReleasedAt)
		if err != nil {
			return err
		}
		if released.Before(acquired) {
			return fmt.Errorf("releasedAt precedes acquiredAt")
		}
		if lease.EscalatedAt != "" {
			return fmt.Errorf("released lease has escalatedAt")
		}
	case ProgramChildLeaseEscalated:
		escalated, err := parseACPTime("program child lease escalatedAt", lease.EscalatedAt)
		if err != nil {
			return err
		}
		if escalated.Before(acquired) {
			return fmt.Errorf("escalatedAt precedes acquiredAt")
		}
		if lease.ReleasedAt != "" {
			return fmt.Errorf("escalated lease has releasedAt")
		}
	default:
		if lease.ReleasedAt != "" || lease.EscalatedAt != "" {
			return fmt.Errorf("active lease has terminal time")
		}
	}
	return nil
}

func programChildLeaseIsActive(lease ProgramChildLease, now time.Time) bool {
	if lease.Status != ProgramChildLeaseActive {
		return false
	}
	leaseUntil, err := time.Parse(time.RFC3339Nano, lease.LeaseUntil)
	if err != nil {
		return false
	}
	return now.UTC().Before(leaseUntil)
}

func sortProgramChildLeases(leases []ProgramChildLease) {
	sort.Slice(leases, func(i, j int) bool {
		if leases[i].Slug != leases[j].Slug {
			return leases[i].Slug < leases[j].Slug
		}
		if leases[i].ParentSessionID != leases[j].ParentSessionID {
			return leases[i].ParentSessionID < leases[j].ParentSessionID
		}
		return leases[i].ChildSessionID < leases[j].ChildSessionID
	})
}

func withProgramChildLeaseLock(root, slug string, fn func() error) error {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return err
	}
	dir, err := paths.ProgramChildDir(slug)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("program orchestration: create child lease directory: %w", err)
	}
	lockPath := filepath.Join(dir, ".lease.lock")

	programChildLeaseLocksMu.Lock()
	mu := programChildLeaseLocks[lockPath]
	if mu == nil {
		mu = &sync.Mutex{}
		programChildLeaseLocks[lockPath] = mu
	}
	programChildLeaseLocksMu.Unlock()

	mu.Lock()
	defer mu.Unlock()

	deadline := time.Now().Add(acpStoreLockTimeout)
	for {
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			if _, writeErr := fmt.Fprintf(lock, "%d %d\n", os.Getpid(), time.Now().UnixMilli()); writeErr != nil {
				lock.Close()
				os.Remove(lockPath)
				return fmt.Errorf("program orchestration: write child lease lock: %w", writeErr)
			}
			if closeErr := lock.Close(); closeErr != nil {
				os.Remove(lockPath)
				return fmt.Errorf("program orchestration: close child lease lock: %w", closeErr)
			}
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("program orchestration: acquire child lease lock: %w", err)
		}
		if isStale(lockPath) {
			if err := os.Remove(lockPath); err == nil || errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("program orchestration: child %s is locked", slug)
		}
		time.Sleep(retryInterval)
	}
	defer os.Remove(lockPath)
	return fn()
}
