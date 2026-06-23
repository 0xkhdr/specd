package core

import (
	"strings"
	"testing"
	"time"
)

// acp_archive_cov_test.go covers ArchiveSession's idempotent-existing-manifest
// and retention-keep branches plus ReplaySessionEvents/CleanupArchives error
// and no-op paths.

func writeTerminalSession(t *testing.T, store *ACPStore) string {
	t.Helper()
	first := testStoreEnvelope(t, 1)
	if _, err := store.WriteEvent(first); err != nil {
		t.Fatal(err)
	}
	term := testStoreEnvelope(t, 2)
	term.Type = ACPMessageCancelled
	term.From = "pinky-1"
	term.To = "brain"
	term.Payload = mustACPPayload(t, ACPCancelledPayload{Reason: "done"})
	if _, err := store.WriteEvent(term); err != nil {
		t.Fatal(err)
	}
	return first.SessionID
}

func TestArchiveSessionBranches(t *testing.T) {
	store := newTestACPStore(t)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	defer setCoreClock(func() time.Time { return now })()

	// Bad session ID is rejected up front.
	if _, err := store.ArchiveSession("short", 0); err == nil {
		t.Error("bad session id should error")
	}
	// A session with no events cannot be archived.
	if _, err := store.ArchiveSession(strings.Repeat("9", 32), 0); err == nil {
		t.Error("session with no events should error")
	}

	sessionID := writeTerminalSession(t, store)

	// Archive with retention > 0 keeps the live session dir.
	m1, err := store.ArchiveSession(sessionID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if m1.EventCount != 2 {
		t.Fatalf("manifest = %#v", m1)
	}
	// Second archive finds the existing manifest and returns it unchanged.
	m2, err := store.ArchiveSession(sessionID, time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	if m2.SealedAt != m1.SealedAt || m2.LastSeq != m1.LastSeq {
		t.Fatalf("idempotent archive drifted: %#v vs %#v", m1, m2)
	}

	// Replay still works (from live events or the archive).
	events, err := store.ReplaySessionEvents(sessionID)
	if err != nil || len(events) != 2 {
		t.Fatalf("replay = %d events, err=%v", len(events), err)
	}
}

func TestReplaySessionEventsUnknown(t *testing.T) {
	store := newTestACPStore(t)
	if _, err := store.ReplaySessionEvents(strings.Repeat("7", 32)); err == nil {
		t.Error("replay of unknown session should error")
	}
}

func TestCleanupArchivesNoArchivesAndKeep(t *testing.T) {
	store := newTestACPStore(t)
	// No archives dir yet → nil, nil.
	removed, err := store.CleanupArchives(time.Now())
	if err != nil || removed != nil {
		t.Fatalf("empty cleanup = %v, %v", removed, err)
	}

	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	defer setCoreClock(func() time.Time { return now })()
	sessionID := writeTerminalSession(t, store)
	if _, err := store.ArchiveSession(sessionID, 0); err != nil {
		t.Fatal(err)
	}
	// Cutoff before sealedAt → archive is kept.
	removed, err = store.CleanupArchives(now.Add(-time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(removed) != 0 {
		t.Fatalf("archive should be kept, removed = %v", removed)
	}
}
