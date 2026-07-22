package orchestration

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

const MissionProtocolVersion = "1"

type MissionStatus string

// MissionPending is the only status the controller mints, and the only one
// ValidateMission accepts. Mission lifecycle after dispatch is tracked by lease
// state and by moving records PendingMissions -> Missions on claim, not by this
// field — so there is deliberately no enum of later states here.
const MissionPending MissionStatus = "pending"

type MissionState string

// MissionState constants represent the effective lifecycle states derived from
// ledger events and lease state (R4.1-R4.4).
const (
	MissionStatePending   MissionState = "pending"   // dispatched but not claimed
	MissionStateClaimed   MissionState = "claimed"   // claimed and lease is active
	MissionStateReleased  MissionState = "released"  // released by controller
	MissionStateExpired   MissionState = "expired"   // lease expired without release
	MissionStateFailed    MissionState = "failed"    // worker reported failure
	MissionStateCompleted MissionState = "completed" // worker reported success
)

type MissionLimits struct {
	MaxAttempts    int   `json:"max_attempts"`
	TimeoutSeconds int   `json:"timeout_seconds"`
	MaxTokens      int64 `json:"max_tokens,omitempty"`
	MaxCostMicros  int64 `json:"max_cost_micros,omitempty"`
}
type MissionV1 struct {
	ProtocolVersion string        `json:"protocol_version"`
	SessionID       string        `json:"session_id"`
	MissionID       string        `json:"mission_id"`
	SpecSlug        string        `json:"spec_slug"`
	TaskID          string        `json:"task_id"`
	Attempt         int           `json:"attempt"`
	Role            string        `json:"role"`
	AuthorityRef    string        `json:"authority_ref"`
	DeclaredFiles   []string      `json:"declared_files"`
	Acceptance      []string      `json:"acceptance"`
	Verify          string        `json:"verify"`
	ContextRef      string        `json:"context_ref"`
	ContextDigest   string        `json:"context_digest"`
	ConfigDigest    string        `json:"config_digest"`
	PaletteDigest   string        `json:"palette_digest"`
	PolicyDigest    string        `json:"policy_digest"`
	SubjectHead     string        `json:"subject_head"`
	DispatchDigest  string        `json:"dispatch_digest,omitempty"`
	DiffDigest      string        `json:"diff_digest,omitempty"`
	RouteClass      string        `json:"route_class"`
	RouteReason     string        `json:"route_reason"`
	Limits          MissionLimits `json:"limits"`
	IssuedAt        time.Time     `json:"issued_at"`
	ExpiresAt       time.Time     `json:"expires_at"`
	Status          MissionStatus `json:"status"`
}

func ValidateMission(m MissionV1) error {
	if m.ProtocolVersion != MissionProtocolVersion {
		return fmt.Errorf("MISSION_VERSION_UNSUPPORTED")
	}
	if m.SessionID == "" || m.MissionID == "" || m.SpecSlug == "" || m.TaskID == "" || m.Attempt < 1 || m.Role == "" || m.AuthorityRef == "" || m.Verify == "" || m.ContextRef == "" || m.ContextDigest == "" || m.ConfigDigest == "" || m.PaletteDigest == "" || m.PolicyDigest == "" || m.SubjectHead == "" || m.RouteClass == "" || m.RouteReason == "" || m.IssuedAt.IsZero() || !m.ExpiresAt.After(m.IssuedAt) {
		return fmt.Errorf("MISSION_REQUIRED_PIN_MISSING")
	}
	if m.Status != MissionPending {
		return fmt.Errorf("MISSION_INITIAL_STATUS_INVALID: %s", m.Status)
	}
	if m.Limits.MaxAttempts < 1 || m.Limits.TimeoutSeconds < 1 {
		return fmt.Errorf("MISSION_LIMIT_INVALID")
	}
	seen := map[string]bool{}
	for _, f := range m.DeclaredFiles {
		if f == "" || seen[f] {
			return fmt.Errorf("MISSION_FILE_INVALID")
		}
		seen[f] = true
	}
	return nil
}
func CanonicalizeMission(m *MissionV1) { sort.Strings(m.DeclaredFiles); sort.Strings(m.Acceptance) }
func MissionDigest(m MissionV1) string {
	CanonicalizeMission(&m)
	raw, _ := json.Marshal(m)
	return core.Digest(raw)
}
func MissionPayload(m MissionV1) (string, error) {
	if err := ValidateMission(m); err != nil {
		return "", err
	}
	CanonicalizeMission(&m)
	raw, err := json.Marshal(m)
	return string(raw), err
}

type DispatchPins struct {
	TaskID         string
	Role           string
	DeclaredFiles  []string
	Acceptance     []string
	Verify         string
	ContextDigest  string
	ConfigDigest   string
	PaletteDigest  string
	AuthorityRef   string
	SubjectHead    string
	DispatchDigest string
}

func ValidateMissionPins(m MissionV1, p DispatchPins) error {
	aFiles, bFiles := append([]string(nil), m.DeclaredFiles...), append([]string(nil), p.DeclaredFiles...)
	sort.Strings(aFiles)
	sort.Strings(bFiles)
	aAccept, bAccept := append([]string(nil), m.Acceptance...), append([]string(nil), p.Acceptance...)
	sort.Strings(aAccept)
	sort.Strings(bAccept)
	eq := func(a, b []string) bool {
		if len(a) != len(b) {
			return false
		}
		for i := range a {
			if a[i] != b[i] {
				return false
			}
		}
		return true
	}
	if m.TaskID != p.TaskID || m.Role != p.Role || !eq(aFiles, bFiles) || !eq(aAccept, bAccept) || m.Verify != p.Verify || m.ContextDigest != p.ContextDigest || m.ConfigDigest != p.ConfigDigest || m.PaletteDigest != p.PaletteDigest || m.AuthorityRef != p.AuthorityRef || m.SubjectHead != p.SubjectHead || (m.DispatchDigest != "" && m.DispatchDigest != p.DispatchDigest) {
		return fmt.Errorf("MISSION_PIN_MISMATCH: re-bootstrap and dispatch a fresh mission")
	}
	return nil
}

// ComputeMissionState derives the effective mission state from ledger events
// and lease state (R4.1-R4.4). A mission progresses through:
//   - pending: dispatched, not yet claimed
//   - claimed: claimed by a worker and lease is active
//   - released: released by controller (explicit release reason)
//   - expired: lease expired without explicit release
//   - failed: worker reported failure
//   - completed: worker reported success
func ComputeMissionState(missionID string, lease *Lease, events []ACPEvent, now time.Time) MissionState {
	// Find events related to this mission
	var reportEvent ACPEvent
	hasReport := false

	for _, e := range events {
		if e.MissionID != missionID {
			continue
		}
		if e.Kind == ACPKindReport && !hasReport {
			reportEvent = e
			hasReport = true
		}
	}

	// If no lease, the mission was never claimed
	if lease == nil {
		return MissionStatePending
	}

	// Mission was claimed, now determine its current state
	// Check if mission completed (report exists with success status)
	if hasReport && reportEvent.Payload != "" {
		// Need to parse report to check status
		var rep WorkerReportV1
		if err := json.Unmarshal([]byte(reportEvent.Payload), &rep); err == nil {
			if rep.Status == "complete" {
				return MissionStateCompleted
			}
			if rep.Status == "failed" {
				return MissionStateFailed
			}
		}
	}

	// Check if mission was explicitly released
	if lease.State == LeaseRevoked && lease.RevocationReason != "" {
		return MissionStateReleased
	}

	// Check if lease expired
	if lease.State == LeaseExpired || !now.Before(lease.ExpiresAt) {
		return MissionStateExpired
	}

	// Lease is active
	if lease.State == LeaseActive && now.Before(lease.ExpiresAt) {
		return MissionStateClaimed
	}

	// Default to pending if uncertain
	return MissionStatePending
}

// SelectBaseline returns the mission to use as baseline, preferring live claimed
// missions over expired or abandoned ones (R4.4). Returns nil if no suitable
// baseline exists.
func SelectBaseline(missions []MissionV1, leases []Lease, events []ACPEvent, now time.Time) *MissionV1 {
	// Build a map of mission -> lease for quick lookup
	leaseByMissionID := make(map[string]*Lease)
	for i, l := range leases {
		leaseByMissionID[l.MissionID] = &leases[i]
	}

	// Sort by descending priority: claimed > released > expired > pending
	// Among same priority, prefer most recent
	best := -1
	bestState := ""
	bestTime := time.Time{}

	stateRank := map[MissionState]int{
		MissionStateClaimed:   100,
		MissionStateReleased:  80,
		MissionStateExpired:   60,
		MissionStatePending:   40,
		MissionStateFailed:    20,
		MissionStateCompleted: 10,
	}

	for i, m := range missions {
		lease := leaseByMissionID[m.MissionID]
		state := ComputeMissionState(m.MissionID, lease, events, now)
		rank := stateRank[state]

		// Prefer higher rank, or same rank but more recent
		if best == -1 || rank > stateRank[MissionState(bestState)] ||
			(rank == stateRank[MissionState(bestState)] && m.IssuedAt.After(bestTime)) {
			best = i
			bestState = string(state)
			bestTime = m.IssuedAt
		}
	}

	if best >= 0 {
		return &missions[best]
	}
	return nil
}

// ReleaseMission creates a cancel event to immediately release a mission
// without TTL wait (R4.3). The lease is marked as revoked with the given reason.
func ReleaseMission(lease Lease, reason string) Lease {
	lease.State = LeaseRevoked
	lease.RevocationReason = reason
	return lease
}
