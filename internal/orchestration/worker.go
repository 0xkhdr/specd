package orchestration

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/core"
)

type WorkerV1 struct {
	WorkerID     string   `json:"worker_id"`
	Host         string   `json:"host"`
	Roles        []string `json:"roles"`
	Capabilities []string `json:"capabilities"`
}
type ClaimEcho struct {
	MissionID      string
	TaskID         string
	Role           string
	ContextDigest  string
	ConfigDigest   string
	PaletteDigest  string
	AuthorityRef   string
	SubjectHead    string
	DispatchDigest string
}

func ClaimMission(m MissionV1, w WorkerV1, e ClaimEcho, now time.Time, ttl time.Duration) (Lease, error) {
	if err := ValidateMission(m); err != nil {
		return Lease{}, err
	}
	if w.WorkerID == "" || w.WorkerID == "brain" || w.Host == "" {
		return Lease{}, fmt.Errorf("WORKER_IDENTITY_INVALID")
	}
	if !hasString(w.Roles, m.Role) {
		return Lease{}, fmt.Errorf("WORKER_ROLE_MISMATCH")
	}
	// R6.4: a mission the approved plan pinned to a named worker must be claimed
	// by that worker. A dash/empty worker is host-chooses and imposes no
	// restriction. This is an out-of-scope class refusal, not a warning.
	if planWorker := strings.TrimSpace(m.Worker); planWorker != "" && planWorker != "-" && w.WorkerID != planWorker {
		return Lease{}, core.Refusef("WORKER_OUT_OF_SCOPE", "mission %s (task %s) is pinned to worker %q but was claimed by %q", m.MissionID, m.TaskID, planWorker, w.WorkerID)
	}
	if e.MissionID != m.MissionID || e.TaskID != m.TaskID || e.Role != m.Role || e.ContextDigest != m.ContextDigest || e.ConfigDigest != m.ConfigDigest || e.PaletteDigest != m.PaletteDigest || e.AuthorityRef != m.AuthorityRef || e.SubjectHead != m.SubjectHead || (m.DispatchDigest != "" && e.DispatchDigest != m.DispatchDigest) {
		return Lease{}, fmt.Errorf("WORKER_PIN_MISMATCH")
	}
	if now.Before(m.IssuedAt) || !now.Before(m.ExpiresAt) || ttl <= 0 {
		return Lease{}, fmt.Errorf("MISSION_EXPIRED")
	}
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return Lease{}, err
	}
	expires := now.Add(ttl)
	if expires.After(m.ExpiresAt) {
		expires = m.ExpiresAt
	}
	authority, err := core.BuildAuthority(core.TaskRow{ID: m.TaskID, Role: m.Role, DeclaredFiles: append([]string(nil), m.DeclaredFiles...)}, "controller", w.WorkerID, m.SpecSlug, "execute", m.SubjectHead, m.PolicyDigest, "required", now, expires)
	if err != nil {
		return Lease{}, err
	}
	return Lease{LeaseID: hex.EncodeToString(b), MissionID: m.MissionID, TaskID: m.TaskID, Attempt: m.Attempt, WorkerID: w.WorkerID, IssuedAt: now, ExpiresAt: expires, PolicyDigest: m.PolicyDigest, DispatchDigest: m.DispatchDigest, State: LeaseActive, Authority: authority}, nil
}

func CheckClaimConflict(leases []Lease, m MissionV1, now time.Time) error {
	for _, l := range leases {
		if l.TaskID == m.TaskID && l.Attempt == m.Attempt && l.State == LeaseActive && now.Before(l.ExpiresAt) {
			return fmt.Errorf("LEASE_CONFLICT: task %s attempt %d already claimed", m.TaskID, m.Attempt)
		}
	}
	return nil
}
func hasString(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}
