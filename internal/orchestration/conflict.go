package orchestration

import (
	"fmt"
	"path/filepath"
	"slices"
	"time"
)

type CoordinationRule struct {
	Digest       string   `json:"digest,omitempty"`
	OrderedTasks []string `json:"ordered_tasks,omitempty"`
}

func CheckParallelConflict(candidate MissionV1, missions []MissionV1, leases []Lease, rule CoordinationRule, now time.Time) error {
	byID := make(map[string]MissionV1, len(missions))
	for _, m := range missions {
		byID[m.MissionID] = m
	}
	for _, lease := range leases {
		if lease.State != LeaseActive || !now.Before(lease.ExpiresAt) {
			continue
		}
		active, ok := byID[lease.MissionID]
		if !ok {
			return fmt.Errorf("PARALLEL_MISSION_UNKNOWN: %s", lease.MissionID)
		}
		if !scopesOverlap(candidate.DeclaredFiles, active.DeclaredFiles) {
			continue
		}
		if coordinated(active.TaskID, candidate.TaskID, rule) {
			continue
		}
		return fmt.Errorf("WRITE_SCOPE_CONFLICT: %s and %s", active.TaskID, candidate.TaskID)
	}
	return nil
}

func scopesOverlap(a, b []string) bool {
	seen := make(map[string]struct{}, len(a))
	for _, p := range a {
		if p = normalizeScope(p); p != "" {
			seen[p] = struct{}{}
		}
	}
	for _, p := range b {
		if _, ok := seen[normalizeScope(p)]; ok {
			return true
		}
	}
	return false
}

func normalizeScope(p string) string {
	if p == "" {
		return ""
	}
	return filepath.ToSlash(filepath.Clean(p))
}

func coordinated(first, second string, rule CoordinationRule) bool {
	if rule.Digest == "" || len(rule.OrderedTasks) < 2 {
		return false
	}
	i, j := slices.Index(rule.OrderedTasks, first), slices.Index(rule.OrderedTasks, second)
	return i >= 0 && j == i+1
}
