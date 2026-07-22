package orchestration

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/0xkhdr/specd/internal/core"
)

var ErrSessionRevisionConflict = errors.New("session revision conflict")

// SessionState is the controller lifecycle (spec 07 R2). The persisted state
// machine is running → {cancelled | complete}; both terminal states refuse
// further step/run. An empty state decodes as running so pre-spec-07 sessions
// stay valid. "crashed" is never persisted — it is derived by `brain status`
// from a checkpoint that outran the ledger (see DeriveStatus).
type SessionState string

const (
	SessionRunning   SessionState = "running"
	SessionCancelled SessionState = "cancelled"
	SessionComplete  SessionState = "complete"
	SessionCrashed   SessionState = "crashed"
)

type Session struct {
	// ID is a stable identifier minted at `brain start`, used to derive
	// deterministic mission ids (session/step/task) that survive a resume.
	ID       string       `json:"id,omitempty"`
	Revision int64        `json:"revision"`
	State    SessionState `json:"state,omitempty"`
	// Step is the controller tick counter, incremented per dispatch. It feeds the
	// mission id so a re-issued dispatch after resume reuses the same id.
	Step            int         `json:"step,omitempty"`
	Leases          []Lease     `json:"leases,omitempty"`
	PendingMissions []MissionV1 `json:"pending_missions,omitempty"`
	Missions        []MissionV1 `json:"missions,omitempty"`
	// WaitingApproval names the lifecycle gate the controller halted on, or is
	// empty when it is not waiting on one (R4.1). It is a marker beside the
	// session, never a replacement for it: leases, missions, and the step
	// counter are untouched by the halt, so the run resumes where it stopped
	// once the approval lands (R4.3).
	WaitingApproval string `json:"waiting_approval,omitempty"`
}

// Status returns the effective lifecycle state, treating the empty zero value as
// running (pre-spec-07 compatibility).
func (s Session) Status() SessionState {
	if s.State == "" {
		return SessionRunning
	}
	return s.State
}

// IsTerminal reports whether the session refuses further step/run/resume.
func (s Session) IsTerminal() bool {
	st := s.Status()
	return st == SessionCancelled || st == SessionComplete
}

func LoadSession(path string) (Session, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return Session{Revision: 0}, nil
	}
	if err != nil {
		return Session{}, fmt.Errorf("read session: %w", err)
	}
	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return Session{}, fmt.Errorf("decode session: %w", err)
	}
	return session, nil
}

func SaveSessionCAS(root, path string, expectedRevision int64, next Session) error {
	_, err := core.WithSpecLock(root, func() (struct{}, error) {
		current, err := LoadSession(path)
		if err != nil {
			return struct{}{}, err
		}
		if current.Revision != expectedRevision {
			return struct{}{}, ErrSessionRevisionConflict
		}
		next.Revision = expectedRevision + 1
		data, err := json.MarshalIndent(next, "", "  ")
		if err != nil {
			return struct{}{}, fmt.Errorf("encode session: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return struct{}{}, fmt.Errorf("mkdir session: %w", err)
		}
		return struct{}{}, core.AtomicWrite(path, string(append(data, '\n')))
	})
	return err
}
