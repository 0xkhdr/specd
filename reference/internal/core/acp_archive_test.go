package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestACPArchiveReplayAndCleanup(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	first := testStoreEnvelope(t, 1)
	if _, err := store.WriteEvent(first); err != nil {
		t.Fatal(err)
	}
	terminal := testStoreEnvelope(t, 2)
	terminal.Type = ACPMessageCancelled
	terminal.From = "pinky-1"
	terminal.To = "brain"
	terminal.Payload = mustACPPayload(t, ACPCancelledPayload{Reason: "done"})
	if _, err := store.WriteEvent(terminal); err != nil {
		t.Fatal(err)
	}

	manifest, err := store.ArchiveSession(first.SessionID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.EventCount != 2 || manifest.LastSeq != 2 {
		t.Fatalf("manifest = %#v, want two events", manifest)
	}
	sessionDir, err := store.paths.SessionDir(first.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(sessionDir); !os.IsNotExist(err) {
		t.Fatalf("session dir still exists/error = %v", err)
	}
	events, err := store.ReplaySessionEvents(first.SessionID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 || events[0].Sequence != 1 || events[1].Type != ACPMessageCancelled {
		t.Fatalf("events = %#v, want deterministic archived replay", events)
	}
	removed, err := store.CleanupArchives(now.Add(time.Nanosecond))
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 1 || removed[0] != first.SessionID {
		t.Fatalf("removed = %#v, want archived session", removed)
	}
}

func TestACPArchiveRejectsTraversalCleanup(t *testing.T) {
	store := newTestACPStore(t)
	archives, err := store.paths.ArchivesDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(archives, "..bad"), 0o700); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CleanupArchives(time.Now()); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(archives, "..bad")); err != nil {
		t.Fatalf("invalid archive entry removed: %v", err)
	}
}

func mustACPPayload(t *testing.T, payload any) []byte {
	t.Helper()
	envelope, err := NewACPEnvelope(ACPMessageCancelled, payload)
	if err != nil {
		t.Fatal(err)
	}
	return envelope.Payload
}

func TestACPArchiveRequiresTerminalEvent(t *testing.T) {
	store := newTestACPStore(t)
	event := testStoreEnvelope(t, 1)
	if _, err := store.WriteEvent(event); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ArchiveSession(event.SessionID, 0); err == nil || !strings.Contains(err.Error(), "not terminal") {
		t.Fatalf("ArchiveSession error = %v, want not terminal", err)
	}
}
