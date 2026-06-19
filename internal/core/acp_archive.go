package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type ACPArchiveManifest struct {
	Version    int    `json:"version"`
	SessionID  string `json:"sessionId"`
	SealedAt   string `json:"sealedAt"`
	EventCount int    `json:"eventCount"`
	LastSeq    uint64 `json:"lastSequence"`
}

const acpArchiveVersion = 1

func (s *ACPStore) ArchiveSession(sessionID string, retention time.Duration) (ACPArchiveManifest, error) {
	if err := validateACPOpaqueID("session ID", sessionID); err != nil {
		return ACPArchiveManifest{}, err
	}
	var manifest ACPArchiveManifest
	err := s.withSessionLock(sessionID, func() error {
		events, err := s.readAllEvents(sessionID)
		if err != nil {
			return err
		}
		if len(events) == 0 {
			return fmt.Errorf("acp archive: session has no events")
		}
		last := events[len(events)-1]
		if !isTerminalACPEvent(last.Type) {
			return fmt.Errorf("acp archive: session is not terminal")
		}
		manifest = ACPArchiveManifest{
			Version:    acpArchiveVersion,
			SessionID:  sessionID,
			SealedAt:   Clock().UTC().Format(time.RFC3339Nano),
			EventCount: len(events),
			LastSeq:    last.Sequence,
		}
		archiveDir, err := s.paths.ArchivePath(sessionID)
		if err != nil {
			return err
		}
		if existing, err := readACPArchiveManifest(archiveDir); err == nil {
			manifest = existing
			return nil
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
		tmp := archiveDir + ".tmp"
		if err := os.RemoveAll(tmp); err != nil {
			return fmt.Errorf("acp archive: reset temp archive: %w", err)
		}
		if err := os.MkdirAll(filepath.Join(tmp, "events"), 0o700); err != nil {
			return fmt.Errorf("acp archive: create temp archive: %w", err)
		}
		for _, event := range events {
			raw, err := json.MarshalIndent(event, "", " ")
			if err != nil {
				return fmt.Errorf("acp archive: encode event: %w", err)
			}
			name, err := ACPEventFilename(event.Sequence, event.MessageID)
			if err != nil {
				return err
			}
			if err := atomicWritePrivate(filepath.Join(tmp, "events", name), append(raw, '\n')); err != nil {
				return fmt.Errorf("acp archive: write event: %w", err)
			}
		}
		raw, err := json.MarshalIndent(manifest, "", " ")
		if err != nil {
			return fmt.Errorf("acp archive: encode manifest: %w", err)
		}
		if err := atomicWritePrivate(filepath.Join(tmp, "manifest.json"), append(raw, '\n')); err != nil {
			return fmt.Errorf("acp archive: write manifest: %w", err)
		}
		if err := os.MkdirAll(filepath.Dir(archiveDir), 0o700); err != nil {
			return fmt.Errorf("acp archive: create archives dir: %w", err)
		}
		if err := os.Rename(tmp, archiveDir); err != nil {
			return fmt.Errorf("acp archive: publish archive: %w", err)
		}
		sessionDir, err := s.paths.SessionDir(sessionID)
		if err != nil {
			return err
		}
		if retention <= 0 {
			return os.RemoveAll(sessionDir)
		}
		return nil
	})
	return manifest, err
}

func (s *ACPStore) ReplaySessionEvents(sessionID string) ([]ACPEnvelope, error) {
	events, err := s.readAllEvents(sessionID)
	if err == nil && len(events) > 0 {
		return events, nil
	}
	archiveDir, pathErr := s.paths.ArchivePath(sessionID)
	if pathErr != nil {
		return nil, pathErr
	}
	archived, archiveErr := readACPArchiveEvents(archiveDir, sessionID)
	if archiveErr == nil {
		return archived, nil
	}
	if err != nil {
		return nil, err
	}
	return nil, archiveErr
}

func (s *ACPStore) CleanupArchives(olderThan time.Time) ([]string, error) {
	archivesDir, err := s.paths.ArchivesDir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(archivesDir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("acp archive: read archives: %w", err)
	}
	var removed []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sessionID := entry.Name()
		if err := validateACPOpaqueID("session ID", sessionID); err != nil {
			continue
		}
		archiveDir, err := s.paths.ArchivePath(sessionID)
		if err != nil {
			return nil, err
		}
		manifest, err := readACPArchiveManifest(archiveDir)
		if err != nil {
			continue
		}
		sealedAt, err := parseACPTime("sealedAt", manifest.SealedAt)
		if err != nil {
			continue
		}
		if sealedAt.Before(olderThan) {
			if err := os.RemoveAll(archiveDir); err != nil {
				return nil, fmt.Errorf("acp archive: remove archive: %w", err)
			}
			removed = append(removed, sessionID)
		}
	}
	sort.Strings(removed)
	return removed, nil
}

func readACPArchiveManifest(archiveDir string) (ACPArchiveManifest, error) {
	raw, err := os.ReadFile(filepath.Join(archiveDir, "manifest.json"))
	if err != nil {
		return ACPArchiveManifest{}, err
	}
	var manifest ACPArchiveManifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return ACPArchiveManifest{}, fmt.Errorf("acp archive: corrupt manifest: %w", err)
	}
	if manifest.Version != acpArchiveVersion {
		return ACPArchiveManifest{}, fmt.Errorf("acp archive: unsupported version %d", manifest.Version)
	}
	if err := validateACPOpaqueID("session ID", manifest.SessionID); err != nil {
		return ACPArchiveManifest{}, err
	}
	return manifest, nil
}

func readACPArchiveEvents(archiveDir, sessionID string) ([]ACPEnvelope, error) {
	manifest, err := readACPArchiveManifest(archiveDir)
	if err != nil {
		return nil, err
	}
	if manifest.SessionID != sessionID {
		return nil, fmt.Errorf("acp archive: session mismatch")
	}
	entries, err := os.ReadDir(filepath.Join(archiveDir, "events"))
	if err != nil {
		return nil, fmt.Errorf("acp archive: read events: %w", err)
	}
	files := make([]acpEventFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		seq, messageID, err := parseACPEventFilename(entry.Name())
		if err != nil {
			return nil, err
		}
		files = append(files, acpEventFile{path: filepath.Join(archiveDir, "events", entry.Name()), sequence: seq, messageID: messageID})
	}
	if _, err := nextACPSequence(files); err != nil {
		return nil, err
	}
	events := make([]ACPEnvelope, 0, len(files))
	for _, file := range files {
		raw, err := os.ReadFile(file.path)
		if err != nil {
			return nil, fmt.Errorf("acp archive: read event: %w", err)
		}
		event, err := ParseACPEnvelope(raw)
		if err != nil {
			return nil, fmt.Errorf("acp archive: corrupt event: %w", err)
		}
		if event.SessionID != sessionID || event.Sequence != file.sequence || event.MessageID != file.messageID {
			return nil, fmt.Errorf("acp archive: event filename does not match envelope")
		}
		events = append(events, event)
	}
	if len(events) != manifest.EventCount || (len(events) > 0 && events[len(events)-1].Sequence != manifest.LastSeq) {
		return nil, fmt.Errorf("acp archive: manifest does not match events")
	}
	return events, nil
}

func isTerminalACPEvent(messageType ACPMessageType) bool {
	return messageType == ACPMessageEvidence || messageType == ACPMessageBlocker || messageType == ACPMessageCancelled
}
