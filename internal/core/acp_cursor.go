package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"
)

type ACPCursor struct {
	Version   int    `json:"version"`
	SessionID string `json:"sessionId"`
	WorkerID  string `json:"workerId"`
	Sequence  uint64 `json:"sequence"`
	MessageID string `json:"messageId,omitempty"`
	UpdatedAt string `json:"updatedAt"`
}

func (s *ACPStore) LoadCursor(sessionID, workerID string) (ACPCursor, error) {
	path, err := s.paths.CursorPath(sessionID, workerID)
	if err != nil {
		return ACPCursor{}, err
	}
	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return ACPCursor{
			Version:   1,
			SessionID: sessionID,
			WorkerID:  workerID,
			UpdatedAt: NowISO(),
		}, nil
	}
	if err != nil {
		return ACPCursor{}, fmt.Errorf("acp store: read cursor: %w", err)
	}
	var cursor ACPCursor
	if err := decodeACPStrict(raw, &cursor); err != nil {
		return ACPCursor{}, fmt.Errorf("acp store: corrupt cursor: %w", err)
	}
	if err := validateACPCursor(cursor, sessionID, workerID); err != nil {
		return ACPCursor{}, fmt.Errorf("acp store: corrupt cursor: %w", err)
	}
	return cursor, nil
}

// SaveCursor acknowledges one successfully reconciled event. Cursors may only
// move forward and must point at an existing event with a matching message ID.
func (s *ACPStore) SaveCursor(sessionID, workerID string, event ACPEnvelope) error {
	return s.withSessionLock(sessionID, func() error {
		current, err := s.LoadCursor(sessionID, workerID)
		if err != nil {
			return err
		}
		if event.SessionID != sessionID || event.Sequence == 0 || event.MessageID == "" {
			return fmt.Errorf("acp store: cursor event does not match session")
		}
		if event.Sequence < current.Sequence {
			return fmt.Errorf("acp store: cursor rollback from %d to %d", current.Sequence, event.Sequence)
		}
		if event.Sequence == current.Sequence {
			if event.MessageID == current.MessageID {
				return nil
			}
			return fmt.Errorf("acp store: cursor sequence %d has conflicting messageId", event.Sequence)
		}

		events, err := s.readAllEvents(sessionID)
		if err != nil {
			return err
		}
		seen := make(map[string]struct{}, len(events))
		targetFound := false
		for _, stored := range events {
			if stored.Sequence <= current.Sequence {
				seen[stored.MessageID] = struct{}{}
				continue
			}
			if stored.Sequence > event.Sequence {
				break
			}
			if stored.Sequence == event.Sequence {
				if stored.MessageID != event.MessageID {
					return fmt.Errorf("acp store: cursor target does not match event")
				}
				targetFound = true
				break
			}
			if _, duplicate := seen[stored.MessageID]; !duplicate {
				return fmt.Errorf("acp store: cursor cannot skip unreconciled sequence %d", stored.Sequence)
			}
		}
		if !targetFound {
			return fmt.Errorf("acp store: cursor target is unavailable")
		}

		cursor := ACPCursor{
			Version:   1,
			SessionID: sessionID,
			WorkerID:  workerID,
			Sequence:  event.Sequence,
			MessageID: event.MessageID,
			UpdatedAt: NowISO(),
		}
		encoded, err := json.MarshalIndent(cursor, "", "  ")
		if err != nil {
			return fmt.Errorf("acp store: encode cursor: %w", err)
		}
		encoded = append(encoded, '\n')
		cursorPath, err := s.paths.CursorPath(sessionID, workerID)
		if err != nil {
			return err
		}
		if err := atomicWritePrivate(cursorPath, encoded); err != nil {
			return fmt.Errorf("acp store: save cursor: %w", err)
		}
		return nil
	})
}

func validateACPCursor(cursor ACPCursor, sessionID, workerID string) error {
	if cursor.Version != 1 {
		return fmt.Errorf("acp store: unsupported cursor version %d", cursor.Version)
	}
	if cursor.SessionID != sessionID || cursor.WorkerID != workerID {
		return fmt.Errorf("acp store: cursor identity mismatch")
	}
	if cursor.Sequence == 0 && cursor.MessageID != "" {
		return fmt.Errorf("acp store: cursor messageId requires a sequence")
	}
	if cursor.Sequence > 0 {
		if err := validateACPOpaqueID("message ID", cursor.MessageID); err != nil {
			return fmt.Errorf("acp store: invalid cursor messageId")
		}
	}
	if _, err := parseACPTime("cursor updatedAt", cursor.UpdatedAt); err != nil {
		return err
	}
	return nil
}

func parseACPTime(name, value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("acp: invalid %s", name)
	}
	return parsed, nil
}
