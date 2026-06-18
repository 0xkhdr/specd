package core

import (
	"strings"
	"testing"
	"time"
)

func TestOrchestrationSenseBuildsStableSnapshot(t *testing.T) {
	root := writePinkySpec(t)
	sessionID := strings.Repeat("6", 32)
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()
	policy := validOrchestrationPolicy()
	store, err := NewACPStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.ClaimLease(sessionID, "pinky-a", "demo", "T1", 1, time.Minute, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	first, err := SenseOrchestration(root, "demo", sessionID, policy)
	if err != nil {
		t.Fatal(err)
	}
	second, err := SenseOrchestration(root, "demo", sessionID, policy)
	if err != nil {
		t.Fatal(err)
	}
	if first.Revision != second.Revision || first.SessionExpiresAt != second.SessionExpiresAt {
		t.Fatalf("snapshot not stable:\n%#v\n%#v", first, second)
	}
	if len(first.Runnable) != 1 || first.Runnable[0].ID != "T1" {
		t.Fatalf("runnable = %#v, want T1", first.Runnable)
	}
	if len(first.ActiveLeases) != 1 || first.ActiveLeases[0].WorkerID != "pinky-a" {
		t.Fatalf("leases = %#v, want active pinky-a", first.ActiveLeases)
	}
}
