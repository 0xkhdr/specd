package core

import (
	"errors"
	"os"
	"sort"
	"time"
)

// ResumableSession is one entry in the host-facing resume discovery list (R5).
// It is a pure projection of a persisted session plus its last recorded Brain
// decision; it carries no secrets and is safe to print as JSON on startup.
type ResumableSession struct {
	SessionID    string                     `json:"sessionID"`
	Spec         string                     `json:"spec"`
	Status       OrchestrationSessionStatus `json:"status"`
	UpdatedAt    string                     `json:"updatedAt"`
	PausedSince  string                     `json:"pausedSince,omitempty"`
	LastDecision string                     `json:"lastDecision,omitempty"`
}

// ListResumableSessions enumerates every session worth resuming after a host
// restart: those whose status is running or paused, optionally bounded by
// maxAge against UpdatedAt (zero disables the age filter). The result is sorted
// most-recently-updated first so a host can auto-resume the head entry. It is a
// pure read — no session, lease, or event state is mutated — so repeated startup
// calls are free of side effects.
func ListResumableSessions(root string, maxAge time.Duration) ([]ResumableSession, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return nil, err
	}
	dir, err := paths.SessionsDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return []ResumableSession{}, nil
	}
	if err != nil {
		return nil, err
	}

	now := Clock().UTC()
	out := make([]ResumableSession, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		if err := validateACPOpaqueID("session ID", entry.Name()); err != nil {
			// Skip foreign directories rather than failing the whole listing.
			continue
		}
		session, err := LoadOrchestrationSession(root, entry.Name())
		if errors.Is(err, errOrchestrationSessionNotFound) {
			continue
		}
		if err != nil {
			return nil, err
		}
		switch session.Status {
		case OrchestrationSessionRunning, OrchestrationSessionPaused:
			// resumable
		default:
			continue
		}
		updated, err := parseACPTime("updatedAt", session.UpdatedAt)
		if err != nil {
			continue
		}
		if maxAge > 0 && now.Sub(updated) > maxAge {
			continue
		}
		item := ResumableSession{
			SessionID:    session.SessionID,
			Spec:         session.Spec,
			Status:       session.Status,
			UpdatedAt:    session.UpdatedAt,
			LastDecision: lastRecordedDecision(root, session.SessionID),
		}
		if session.Status == OrchestrationSessionPaused {
			item.PausedSince = session.UpdatedAt
		}
		out = append(out, item)
	}

	sort.SliceStable(out, func(i, j int) bool {
		ti, _ := parseACPTime("updatedAt", out[i].UpdatedAt)
		tj, _ := parseACPTime("updatedAt", out[j].UpdatedAt)
		if ti.Equal(tj) {
			return out[i].SessionID < out[j].SessionID
		}
		return ti.After(tj)
	})
	return out, nil
}

// lastRecordedDecision derives the session's most recent Brain decision from the
// event tail, reusing the decision already persisted on decision-bearing events
// rather than introducing a new write path. Best-effort: an unreadable or
// decision-free history yields the empty string.
func lastRecordedDecision(root, sessionID string) string {
	store, err := NewACPStore(root)
	if err != nil {
		return ""
	}
	events, err := store.readAllEvents(sessionID)
	if err != nil {
		return ""
	}
	last := ""
	for _, event := range events {
		if event.Decision != nil {
			last = string(event.Decision.Action)
		}
	}
	return last
}
