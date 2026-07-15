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
	"time"
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

func testStoreEnvelope(t *testing.T, number int) ACPEnvelope {
	t.Helper()
	event := validACPEnvelope(t, ACPMessageMission, "T1", validACPMission())
	event.MessageID = fmt.Sprintf("%032x", number)
	event.Sequence = 0
	return event
}

// scaleEnvelope builds a sequence-zero mission envelope with a unique messageId
// derived from number, without a *testing.T so it is usable from benchmarks.
func scaleEnvelope(sessionID string, number int) (ACPEnvelope, error) {
	envelope, err := NewACPEnvelope(ACPMessageMission, validACPMission())
	if err != nil {
		return ACPEnvelope{}, err
	}
	envelope.MessageID = fmt.Sprintf("%032x", number)
	envelope.SessionID = sessionID
	envelope.Sequence = 0
	envelope.CreatedAt = time.Date(2026, 6, 18, 10, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	envelope.ExpiresAt = time.Date(2026, 6, 18, 11, 0, 0, 0, time.UTC).Format(time.RFC3339Nano)
	envelope.From = "brain"
	envelope.To = "pinky-worker-1"
	envelope.Spec = "example"
	envelope.Task = "T1"
	envelope.Attempt = 1
	return envelope, nil
}

// BenchmarkACPStoreWriteEvent measures per-event write cost against a session
// that already holds many events. Before the GAP-13 fix WriteEvent read, parsed
// and re-validated every prior event on each write (O(n²) over a session); now
// sequence allocation and the dup check are filename-derived, so per-write cost
// no longer scales with the parsed payloads of the backlog.
func BenchmarkACPStoreWriteEvent(b *testing.B) {
	store, err := NewACPStore(b.TempDir())
	if err != nil {
		b.Fatal(err)
	}
	sessionID := strings.Repeat("2", 32)
	const seed = 2000
	for i := 1; i <= seed; i++ {
		env, err := scaleEnvelope(sessionID, i)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := store.WriteEvent(env); err != nil {
			b.Fatal(err)
		}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		env, err := scaleEnvelope(sessionID, seed+1+i)
		if err != nil {
			b.Fatal(err)
		}
		if _, err := store.WriteEvent(env); err != nil {
			b.Fatal(err)
		}
	}
}

// TestACPStoreScaleBehaviorUnchanged is the GAP-13 regression guard: over a
// large session the filename-derived write path must preserve sequence
// contiguity, ordering, duplicate rejection, and sequence-gap detection.
func TestACPStoreScaleBehaviorUnchanged(t *testing.T) {
	store := newTestACPStore(t)
	sessionID := strings.Repeat("2", 32)
	const n = 500

	for i := 1; i <= n; i++ {
		env, err := scaleEnvelope(sessionID, i)
		if err != nil {
			t.Fatal(err)
		}
		written, err := store.WriteEvent(env)
		if err != nil {
			t.Fatalf("write %d: %v", i, err)
		}
		if written.Sequence != uint64(i) {
			t.Fatalf("write %d: sequence = %d, want %d", i, written.Sequence, i)
		}
	}

	// Ordering + contiguity across the whole session.
	events, err := store.ReadEvents(sessionID, "reader")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != n {
		t.Fatalf("read %d events, want %d", len(events), n)
	}
	for i, e := range events {
		if e.Sequence != uint64(i+1) {
			t.Fatalf("event %d has sequence %d, want %d", i, e.Sequence, i+1)
		}
	}

	// Duplicate messageId is still rejected after the backlog grows.
	dup, err := scaleEnvelope(sessionID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.WriteEvent(dup); err == nil || !strings.Contains(err.Error(), "duplicate messageId") {
		t.Fatalf("duplicate not rejected: %v", err)
	}

	// A sequence gap (manually planted) is still detected on the next read.
	gapPath, err := store.paths.EventPath(sessionID, n+2, fmt.Sprintf("%032x", n+2))
	if err != nil {
		t.Fatal(err)
	}
	gapEnv, err := scaleEnvelope(sessionID, n+2)
	if err != nil {
		t.Fatal(err)
	}
	gapEnv.Sequence = uint64(n + 2)
	raw, err := json.MarshalIndent(gapEnv, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	raw = append(raw, '\n')
	if err := writeImmutablePrivate(gapPath, raw); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReadEvents(sessionID, "reader"); err == nil || !strings.Contains(err.Error(), "gap") {
		t.Fatalf("sequence gap not detected: %v", err)
	}
}
