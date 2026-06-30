package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

// PauseProgramOrchestration marks a program session paused and propagates the
// pause to every active child orchestration session so new dispatch stops
// across the program.
func PauseProgramOrchestration(root, parentSessionID string) (ProgramSession, error) {
	session, err := updateProgramSession(root, parentSessionID, OrchestrationSessionPaused)
	if err != nil {
		return ProgramSession{}, err
	}
	if err := propagateProgramControl(root, parentSessionID, PauseOrchestration); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

// ResumeProgramOrchestration marks a program session running again and
// propagates the resume to every active child orchestration session.
func ResumeProgramOrchestration(root, parentSessionID string) (ProgramSession, error) {
	session, err := updateProgramSession(root, parentSessionID, OrchestrationSessionRunning)
	if err != nil {
		return ProgramSession{}, err
	}
	if err := propagateProgramControl(root, parentSessionID, ResumeOrchestration); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

// CancelProgramOrchestration marks a program session cancelling and
// propagates a cooperative cancel to every active child orchestration
// session.
func CancelProgramOrchestration(root, parentSessionID string) (ProgramSession, error) {
	session, err := updateProgramSession(root, parentSessionID, OrchestrationSessionCancelling)
	if err != nil {
		return ProgramSession{}, err
	}
	if err := propagateProgramControl(root, parentSessionID, CancelOrchestration); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

// LoadProgramSession reads and validates a program session by its parent
// session ID, returning errOrchestrationSessionNotFound (wrapped) if no
// session file exists.
func LoadProgramSession(root, parentSessionID string) (ProgramSession, error) {
	if err := validateACPOpaqueID("program session ID", parentSessionID); err != nil {
		return ProgramSession{}, err
	}
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return ProgramSession{}, err
	}
	path, err := paths.ProgramSessionPath(parentSessionID)
	if err != nil {
		return ProgramSession{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ProgramSession{}, fmt.Errorf("%w: %s", errOrchestrationSessionNotFound, parentSessionID)
	}
	if err != nil {
		return ProgramSession{}, fmt.Errorf("program orchestration: read session: %w", err)
	}
	var session ProgramSession
	if err := decodeACPStrict(raw, &session); err != nil {
		return ProgramSession{}, fmt.Errorf("program orchestration: corrupt session: %w", err)
	}
	if err := validateProgramSession(session); err != nil {
		return ProgramSession{}, fmt.Errorf("program orchestration: corrupt session: %w", err)
	}
	return session, nil
}

func ensureProgramSession(root, parentSessionID string) (ProgramSession, error) {
	session, err := LoadProgramSession(root, parentSessionID)
	if err == nil {
		return session, nil
	}
	if !errors.Is(err, errOrchestrationSessionNotFound) {
		return ProgramSession{}, err
	}
	now := Clock().UTC().Format(time.RFC3339Nano)
	session = ProgramSession{
		Version:         OrchestrationModelVersion,
		ParentSessionID: parentSessionID,
		Status:          OrchestrationSessionRunning,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
	if err := saveProgramSession(root, session); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

func updateProgramSession(root, parentSessionID string, status OrchestrationSessionStatus) (ProgramSession, error) {
	if !validSessionStatus(status) || status == OrchestrationSessionFailed {
		return ProgramSession{}, fmt.Errorf("program orchestration: invalid program session status %q", status)
	}
	session, err := ensureProgramSession(root, parentSessionID)
	if err != nil {
		return ProgramSession{}, err
	}
	switch session.Status {
	case OrchestrationSessionComplete, OrchestrationSessionFailed:
		return ProgramSession{}, fmt.Errorf("program orchestration: cannot update a %s session", session.Status)
	}
	session.Status = status
	session.UpdatedAt = Clock().UTC().Format(time.RFC3339Nano)
	if err := saveProgramSession(root, session); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

func markProgramSessionStatus(root, parentSessionID string, status OrchestrationSessionStatus) (ProgramSession, error) {
	if !validSessionStatus(status) {
		return ProgramSession{}, fmt.Errorf("program orchestration: invalid program session status %q", status)
	}
	session, err := ensureProgramSession(root, parentSessionID)
	if err != nil {
		return ProgramSession{}, err
	}
	session.Status = status
	session.UpdatedAt = Clock().UTC().Format(time.RFC3339Nano)
	if err := saveProgramSession(root, session); err != nil {
		return ProgramSession{}, err
	}
	return session, nil
}

func saveProgramSession(root string, session ProgramSession) error {
	if err := validateProgramSession(session); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("program orchestration: encode session: %w", err)
	}
	raw = append(raw, '\n')
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return err
	}
	path, err := paths.ProgramSessionPath(session.ParentSessionID)
	if err != nil {
		return err
	}
	if err := atomicWritePrivate(path, raw); err != nil {
		return fmt.Errorf("program orchestration: save session: %w", err)
	}
	return nil
}

func validateProgramSession(session ProgramSession) error {
	if session.Version != OrchestrationModelVersion {
		return fmt.Errorf("unsupported version %d", session.Version)
	}
	if err := validateACPOpaqueID("program session ID", session.ParentSessionID); err != nil {
		return err
	}
	if !validSessionStatus(session.Status) {
		return fmt.Errorf("unsupported status %q", session.Status)
	}
	created, err := parseACPTime("program session createdAt", session.CreatedAt)
	if err != nil {
		return err
	}
	updated, err := parseACPTime("program session updatedAt", session.UpdatedAt)
	if err != nil {
		return err
	}
	if updated.Before(created) {
		return fmt.Errorf("updatedAt precedes createdAt")
	}
	return nil
}

func propagateProgramControl(root, parentSessionID string, control func(string, string) (OrchestrationSession, error)) error {
	leases, err := LoadProgramChildLeases(root)
	if err != nil {
		return err
	}
	now := Clock().UTC()
	for _, lease := range leases {
		if lease.ParentSessionID != parentSessionID || !programChildLeaseIsActive(lease, now) {
			continue
		}
		session, err := LoadOrchestrationSession(root, lease.ChildSessionID)
		if errors.Is(err, errOrchestrationSessionNotFound) {
			continue
		}
		if err != nil {
			return err
		}
		if session.Status == OrchestrationSessionComplete || session.Status == OrchestrationSessionFailed {
			continue
		}
		if _, err := control(root, lease.ChildSessionID); err != nil {
			return err
		}
	}
	return nil
}
