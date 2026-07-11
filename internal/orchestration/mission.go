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

const (
	MissionPending   MissionStatus = "pending"
	MissionDelivered MissionStatus = "delivered"
	MissionClaimed   MissionStatus = "claimed"
	MissionActive    MissionStatus = "active"
	MissionReported  MissionStatus = "reported"
	MissionExpired   MissionStatus = "expired"
	MissionCancelled MissionStatus = "cancelled"
	MissionEscalated MissionStatus = "escalated"
	MissionTerminal  MissionStatus = "terminal"
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
	TaskID        string
	Role          string
	DeclaredFiles []string
	Acceptance    []string
	Verify        string
	ContextDigest string
	ConfigDigest  string
	PaletteDigest string
	AuthorityRef  string
	SubjectHead   string
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
	if m.TaskID != p.TaskID || m.Role != p.Role || !eq(aFiles, bFiles) || !eq(aAccept, bAccept) || m.Verify != p.Verify || m.ContextDigest != p.ContextDigest || m.ConfigDigest != p.ConfigDigest || m.PaletteDigest != p.PaletteDigest || m.AuthorityRef != p.AuthorityRef || m.SubjectHead != p.SubjectHead {
		return fmt.Errorf("MISSION_PIN_MISMATCH: re-bootstrap and dispatch a fresh mission")
	}
	return nil
}
