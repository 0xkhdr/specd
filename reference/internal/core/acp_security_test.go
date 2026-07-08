package core

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestACPSecurityHostileInputs(t *testing.T) {
	store := newTestACPStore(t)
	if _, err := store.WriteEvent(ACPEnvelope{SessionID: "../escape"}); err == nil {
		t.Fatal("traversal session accepted")
	}
	oversized := testStoreEnvelope(t, 1)
	oversized.Payload = []byte(`{"contract":"` + strings.Repeat("x", ACPMaxEnvelopeBytes) + `"}`)
	if _, err := store.WriteEvent(oversized); err == nil {
		t.Fatal("oversized payload accepted")
	}
	sessionID := strings.Repeat("7", 32)
	sessionDir, err := store.paths.SessionDir(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(sessionDir), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), sessionDir); err != nil {
		t.Fatal(err)
	}
	event := testStoreEnvelope(t, 2)
	event.SessionID = sessionID
	if _, err := store.WriteEvent(event); err == nil {
		t.Fatal("symlinked session accepted")
	}
}

func TestACPCrossProcessStyleStressNoLostMessages(t *testing.T) {
	store := newTestACPStore(t)
	const writers = 24
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := store.WriteEvent(testStoreEnvelope(t, i+1))
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	events, err := store.ReadEvents(strings.Repeat("2", 32), "worker-1")
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != writers {
		t.Fatalf("events = %d, want %d", len(events), writers)
	}
	for i, event := range events {
		if event.Sequence != uint64(i+1) {
			t.Fatalf("sequence[%d] = %d", i, event.Sequence)
		}
	}
}

func TestACPStaleLeaseCannotReport(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()
	sessionID := strings.Repeat("8", 32)
	if _, err := store.ClaimLease(sessionID, "pinky-a", "demo", "T1", 1, time.Minute, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	now = now.Add(2 * time.Minute)
	if err := store.ValidateActiveLease(sessionID, "pinky-a", "demo", "T1", 1); err == nil {
		t.Fatal("expired lease accepted")
	}
}
