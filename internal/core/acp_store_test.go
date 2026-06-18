package core

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"testing"
)

func TestACPStoreWriteImmutableEvent(t *testing.T) {
	store := newTestACPStore(t)
	event := testStoreEnvelope(t, 1)

	written, err := store.WriteEvent(event)
	if err != nil {
		t.Fatal(err)
	}
	if written.Sequence != 1 {
		t.Fatalf("sequence = %d, want 1", written.Sequence)
	}
	path, err := store.paths.EventPath(written.SessionID, written.Sequence, written.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("event permissions = %o, want 600", perm)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := ParseACPEnvelope(raw)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.MessageID != written.MessageID || parsed.Sequence != 1 {
		t.Fatalf("stored event = %#v, want sequence/message match", parsed)
	}
	if _, err := store.WriteEvent(event); err == nil || !strings.Contains(err.Error(), "duplicate messageId") {
		t.Fatalf("duplicate write error = %v, want duplicate messageId", err)
	}

	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temporary event leaked: %s", entry.Name())
		}
	}
}

func TestACPStoreConcurrentWriteGapFree(t *testing.T) {
	store := newTestACPStore(t)
	const writers = 32
	var wg sync.WaitGroup
	results := make(chan ACPEnvelope, writers)
	errs := make(chan error, writers)

	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			event := testStoreEnvelope(t, n+1)
			written, err := store.WriteEvent(event)
			if err != nil {
				errs <- err
				return
			}
			results <- written
		}(i)
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatal(err)
	}

	sequences := make([]int, 0, writers)
	for result := range results {
		sequences = append(sequences, int(result.Sequence))
	}
	sort.Ints(sequences)
	for i, sequence := range sequences {
		if sequence != i+1 {
			t.Fatalf("sequences = %v, want gap-free 1..%d", sequences, writers)
		}
	}
	events, err := store.readAllEvents(strings.Repeat("2", 32))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != writers {
		t.Fatalf("events = %d, want %d", len(events), writers)
	}
}

func TestACPStoreWritePermissionFailure(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	store := newTestACPStore(t)
	sessionDir, err := store.paths.SessionDir(strings.Repeat("2", 32))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(sessionDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sessionDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(sessionDir, 0o700)
	if _, err := store.WriteEvent(testStoreEnvelope(t, 1)); err == nil {
		t.Fatal("WriteEvent succeeded in read-only session directory")
	}
}

func TestACPStoreReadAndCursor(t *testing.T) {
	store := newTestACPStore(t)
	first, err := store.WriteEvent(testStoreEnvelope(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.WriteEvent(testStoreEnvelope(t, 2))
	if err != nil {
		t.Fatal(err)
	}

	got, err := store.ReadEvents(first.SessionID, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Sequence != 1 || got[1].Sequence != 2 {
		t.Fatalf("initial read = %#v, want sequences 1,2", got)
	}
	if err := store.SaveCursor(first.SessionID, "worker-1", first); err != nil {
		t.Fatal(err)
	}
	got, err = store.ReadEvents(first.SessionID, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MessageID != second.MessageID {
		t.Fatalf("read after cursor = %#v, want only second event", got)
	}
	if err := store.SaveCursor(first.SessionID, "worker-1", second); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCursor(first.SessionID, "worker-1", first); err == nil || !strings.Contains(err.Error(), "rollback") {
		t.Fatalf("cursor rollback error = %v, want rollback rejection", err)
	}
	cursorPath, err := store.paths.CursorPath(first.SessionID, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(cursorPath)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("cursor permissions = %o, want 600", perm)
	}
}

func TestACPStoreDuplicateReplayIsDeduplicated(t *testing.T) {
	store := newTestACPStore(t)
	first, err := store.WriteEvent(testStoreEnvelope(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	duplicate := first
	duplicate.Sequence = 2
	raw, err := json.Marshal(duplicate)
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.paths.EventPath(duplicate.SessionID, duplicate.Sequence, duplicate.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	if err := writeImmutablePrivate(path, raw); err != nil {
		t.Fatal(err)
	}

	got, err := store.ReadEvents(first.SessionID, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MessageID != first.MessageID {
		t.Fatalf("deduplicated events = %#v, want one event", got)
	}

	third, err := store.WriteEvent(testStoreEnvelope(t, 3))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCursor(first.SessionID, "worker-1", first); err != nil {
		t.Fatal(err)
	}
	got, err = store.ReadEvents(first.SessionID, "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].MessageID != third.MessageID {
		t.Fatalf("events after duplicate = %#v, want only third event", got)
	}
	if err := store.SaveCursor(first.SessionID, "worker-1", third); err != nil {
		t.Fatalf("cursor did not advance across acknowledged duplicate: %v", err)
	}
}

func TestACPStoreCorruptEventFailsClosed(t *testing.T) {
	store := newTestACPStore(t)
	event := testStoreEnvelope(t, 1)
	path, err := store.paths.EventPath(event.SessionID, 1, event.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":"1"`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEvents(event.SessionID, "worker-1"); err == nil || !strings.Contains(err.Error(), "corrupt event") {
		t.Fatalf("corrupt read error = %v, want corrupt event rejection", err)
	}
}

func TestACPStoreCursorCorruptionFailsClosed(t *testing.T) {
	store := newTestACPStore(t)
	path, err := store.paths.CursorPath(strings.Repeat("2", 32), "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"sequence":9}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEvents(strings.Repeat("2", 32), "worker-1"); err == nil || !strings.Contains(err.Error(), "corrupt cursor") {
		t.Fatalf("cursor read error = %v, want corrupt cursor rejection", err)
	}
}

func TestACPStoreCursorCannotSkipUnreconciledEvent(t *testing.T) {
	store := newTestACPStore(t)
	_, err := store.WriteEvent(testStoreEnvelope(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	second, err := store.WriteEvent(testStoreEnvelope(t, 2))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCursor(second.SessionID, "worker-1", second); err == nil ||
		!strings.Contains(err.Error(), "cannot skip") {
		t.Fatalf("cursor skip error = %v, want unreconciled rejection", err)
	}
}

func TestACPStorePermissionHonorsUmask(t *testing.T) {
	old := syscall.Umask(0o077)
	defer syscall.Umask(old)
	store := newTestACPStore(t)
	written, err := store.WriteEvent(testStoreEnvelope(t, 1))
	if err != nil {
		t.Fatal(err)
	}
	path, err := store.paths.EventPath(written.SessionID, written.Sequence, written.MessageID)
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("permissions = %o, want 600", info.Mode().Perm())
	}
}

func newTestACPStore(t *testing.T) *ACPStore {
	t.Helper()
	store, err := NewACPStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func testStoreEnvelope(t *testing.T, number int) ACPEnvelope {
	t.Helper()
	event := validACPEnvelope(t, ACPMessageMission, "T1", validACPMission())
	event.MessageID = fmt.Sprintf("%032x", number)
	event.Sequence = 0
	return event
}
