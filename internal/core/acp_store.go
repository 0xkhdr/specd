package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const acpStoreLockTimeout = 5 * time.Second

type ACPStore struct {
	paths ACPRuntimePaths
}

type acpEventFile struct {
	path      string
	sequence  uint64
	messageID string
}

// acpLockEntry is a per-session-path in-process mutex with a reference count so
// the registry can drop entries once no goroutine holds or waits on them —
// otherwise the map grows one entry per session for the life of the process
// (a leak for a long-running `specd serve`/daemon).
type acpLockEntry struct {
	mu   sync.Mutex
	refs int
}

var (
	acpStoreLocksMu sync.Mutex
	acpStoreLocks   = map[string]*acpLockEntry{}
)

func NewACPStore(root string) (*ACPStore, error) {
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		return nil, err
	}
	return &ACPStore{paths: paths}, nil
}

// WriteEvent allocates the next session sequence and publishes an immutable
// event. Callers leave Sequence zero; the store is the sole sequence authority.
func (s *ACPStore) WriteEvent(envelope ACPEnvelope) (ACPEnvelope, error) {
	if envelope.Sequence != 0 {
		return ACPEnvelope{}, fmt.Errorf("acp store: sequence is allocated by the store")
	}
	if err := validateACPOpaqueID("session ID", envelope.SessionID); err != nil {
		return ACPEnvelope{}, err
	}
	if err := validateACPOpaqueID("message ID", envelope.MessageID); err != nil {
		return ACPEnvelope{}, err
	}

	var written ACPEnvelope
	err := s.withSessionLock(envelope.SessionID, func() error {
		// Sequence allocation and the duplicate-messageId check are derivable from
		// filenames alone (sequence + messageId are both encoded in the name), so we
		// avoid reading and re-validating every event payload on every write — the
		// O(n²) parse cost flagged in the production review. Full parse+validate is
		// reserved for ReadEvents consumers that actually need payloads.
		files, err := s.eventFiles(envelope.SessionID)
		if err != nil {
			return err
		}
		next, err := nextACPSequence(files)
		if err != nil {
			return err
		}
		for _, file := range files {
			if file.messageID == envelope.MessageID {
				return fmt.Errorf("acp store: duplicate messageId %s", envelope.MessageID)
			}
		}

		envelope.Sequence = next
		if err := ValidateACPEnvelope(envelope); err != nil {
			return err
		}
		raw, err := json.MarshalIndent(envelope, "", "  ")
		if err != nil {
			return fmt.Errorf("acp store: encode event: %w", err)
		}
		raw = append(raw, '\n')
		path, err := s.paths.EventPath(envelope.SessionID, envelope.Sequence, envelope.MessageID)
		if err != nil {
			return err
		}
		if err := writeImmutablePrivate(path, raw); err != nil {
			return fmt.Errorf("acp store: publish event: %w", err)
		}
		written = envelope
		return nil
	})
	return written, err
}

func (s *ACPStore) ReadEvents(sessionID, workerID string) ([]ACPEnvelope, error) {
	cursor, err := s.LoadCursor(sessionID, workerID)
	if err != nil {
		return nil, err
	}
	events, err := s.readAllEvents(sessionID)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]struct{}, len(events))
	out := make([]ACPEnvelope, 0, len(events))
	for _, event := range events {
		if _, duplicate := seen[event.MessageID]; duplicate {
			continue
		}
		seen[event.MessageID] = struct{}{}
		if event.Sequence > cursor.Sequence {
			out = append(out, event)
		}
	}
	return out, nil
}

func (s *ACPStore) readAllEvents(sessionID string) ([]ACPEnvelope, error) {
	files, err := s.eventFiles(sessionID)
	if err != nil {
		return nil, err
	}
	if _, err := nextACPSequence(files); err != nil {
		return nil, err
	}

	events := make([]ACPEnvelope, 0, len(files))
	for _, file := range files {
		raw, err := os.ReadFile(file.path)
		if err != nil {
			return nil, fmt.Errorf("acp store: read event %s: %w", filepath.Base(file.path), err)
		}
		event, err := ParseACPEnvelope(raw)
		if err != nil {
			return nil, fmt.Errorf("acp store: corrupt event %s: %w", filepath.Base(file.path), err)
		}
		if event.SessionID != sessionID || event.Sequence != file.sequence || event.MessageID != file.messageID {
			return nil, fmt.Errorf("acp store: event filename does not match envelope: %s", filepath.Base(file.path))
		}
		events = append(events, event)
	}
	return events, nil
}

func (s *ACPStore) eventFiles(sessionID string) ([]acpEventFile, error) {
	eventsDir, err := s.paths.EventsDir(sessionID)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(eventsDir)
	if errors.Is(err, os.ErrNotExist) {
		return []acpEventFile{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("acp store: read events directory: %w", err)
	}

	files := make([]acpEventFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		sequence, messageID, err := parseACPEventFilename(entry.Name())
		if err != nil {
			return nil, err
		}
		path, err := s.paths.EventPath(sessionID, sequence, messageID)
		if err != nil {
			return nil, err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return nil, fmt.Errorf("acp store: inspect event %s: %w", entry.Name(), err)
		}
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
			return nil, fmt.Errorf("acp store: insecure event file %s", entry.Name())
		}
		files = append(files, acpEventFile{path: path, sequence: sequence, messageID: messageID})
	}
	sort.Slice(files, func(i, j int) bool {
		if files[i].sequence == files[j].sequence {
			return files[i].messageID < files[j].messageID
		}
		return files[i].sequence < files[j].sequence
	})
	return files, nil
}

func nextACPSequence(files []acpEventFile) (uint64, error) {
	for i, file := range files {
		want := uint64(i + 1)
		if file.sequence != want {
			return 0, fmt.Errorf("acp store: event sequence rollback or gap: got %d, want %d", file.sequence, want)
		}
	}
	return uint64(len(files)) + 1, nil
}

func parseACPEventFilename(name string) (uint64, string, error) {
	if len(name) != acpEventSequenceWidth+1+32+len(".json") ||
		name[acpEventSequenceWidth] != '-' ||
		!strings.HasSuffix(name, ".json") {
		return 0, "", fmt.Errorf("acp store: invalid event filename %q", name)
	}
	sequence, err := strconv.ParseUint(name[:acpEventSequenceWidth], 10, 64)
	if err != nil || sequence == 0 {
		return 0, "", fmt.Errorf("acp store: invalid event filename %q", name)
	}
	messageID := name[acpEventSequenceWidth+1 : len(name)-len(".json")]
	if err := validateACPOpaqueID("message ID", messageID); err != nil {
		return 0, "", fmt.Errorf("acp store: invalid event filename %q", name)
	}
	return sequence, messageID, nil
}

func (s *ACPStore) withSessionLock(sessionID string, fn func() error) error {
	sessionDir, err := s.paths.SessionDir(sessionID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(sessionDir, 0o700); err != nil {
		return fmt.Errorf("acp store: create session directory: %w", err)
	}
	lockPath := filepath.Join(sessionDir, ".store.lock")

	acpStoreLocksMu.Lock()
	entry := acpStoreLocks[lockPath]
	if entry == nil {
		entry = &acpLockEntry{}
		acpStoreLocks[lockPath] = entry
	}
	entry.refs++
	acpStoreLocksMu.Unlock()

	entry.mu.Lock()
	defer func() {
		entry.mu.Unlock()
		acpStoreLocksMu.Lock()
		entry.refs--
		if entry.refs == 0 {
			delete(acpStoreLocks, lockPath)
		}
		acpStoreLocksMu.Unlock()
	}()

	deadline := time.Now().Add(acpStoreLockTimeout)
	for {
		lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
		if err == nil {
			if _, writeErr := fmt.Fprintf(lock, "%d %d\n", os.Getpid(), time.Now().UnixMilli()); writeErr != nil {
				lock.Close()
				os.Remove(lockPath)
				return fmt.Errorf("acp store: write session lock: %w", writeErr)
			}
			if closeErr := lock.Close(); closeErr != nil {
				os.Remove(lockPath)
				return fmt.Errorf("acp store: close session lock: %w", closeErr)
			}
			break
		}
		if !errors.Is(err, os.ErrExist) {
			return fmt.Errorf("acp store: acquire session lock: %w", err)
		}
		if isStale(lockPath) {
			if err := os.Remove(lockPath); err == nil || errors.Is(err, os.ErrNotExist) {
				continue
			}
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("acp store: session %s is locked", sessionID)
		}
		time.Sleep(retryInterval)
	}
	defer os.Remove(lockPath)
	return fn()
}

func writeImmutablePrivate(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		return fmt.Errorf("target already exists with mode %s", info.Mode())
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}

	temp, err := os.CreateTemp(dir, ".event-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		temp.Close()
		os.Remove(tempPath)
	}()
	if err := temp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Link(tempPath, path); err != nil {
		return err
	}
	if err := syncDirectory(dir); err != nil {
		os.Remove(path)
		return err
	}
	return nil
}

func atomicWritePrivate(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("refusing to replace non-regular file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temp, err := os.CreateTemp(dir, ".private-*.tmp")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		temp.Close()
		os.Remove(tempPath)
	}()
	if err := temp.Chmod(0o600); err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return syncDirectory(dir)
}

func syncDirectory(dir string) error {
	handle, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer handle.Close()
	if err := handle.Sync(); err != nil {
		return fmt.Errorf("fsync directory: %w", err)
	}
	return nil
}
