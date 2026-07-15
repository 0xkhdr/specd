package core

import (
	"strings"
	"testing"
	"time"
)

func seedResumableSession(t *testing.T, root, id string, status OrchestrationSessionStatus, updated time.Time) {
	t.Helper()
	sess := validOrchestrationSessionForTest(t)
	sess.SessionID = id
	sess.Status = status
	sess.CreatedAt = updated.Format(time.RFC3339Nano)
	sess.UpdatedAt = updated.Format(time.RFC3339Nano)
	if err := saveOrchestrationSession(root, sess); err != nil {
		t.Fatalf("seed session %s: %v", id, err)
	}
}

func TestListResumableSessionsFiltersAndOrders(t *testing.T) {
	root := t.TempDir()
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return now })
	defer restore()

	seedResumableSession(t, root, strings.Repeat("a", 32), OrchestrationSessionRunning, now.Add(-1*time.Minute))
	seedResumableSession(t, root, strings.Repeat("b", 32), OrchestrationSessionPaused, now.Add(-2*time.Minute))
	seedResumableSession(t, root, strings.Repeat("c", 32), OrchestrationSessionComplete, now.Add(-30*time.Second))

	all, err := ListResumableSessions(root, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 2 {
		t.Fatalf("want 2 resumable (running+paused), got %d: %#v", len(all), all)
	}
	// Sorted most-recently-updated first: running (-1m) before paused (-2m).
	if all[0].SessionID != strings.Repeat("a", 32) || all[1].SessionID != strings.Repeat("b", 32) {
		t.Fatalf("unexpected order: %#v", all)
	}
	if all[1].PausedSince == "" {
		t.Fatal("paused session should carry pausedSince")
	}

	// Age filter excludes the 2-minute-old paused session.
	fresh, err := ListResumableSessions(root, 90*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if len(fresh) != 1 || fresh[0].SessionID != strings.Repeat("a", 32) {
		t.Fatalf("max-age filter wrong: %#v", fresh)
	}
}

func TestListResumableSessionsEmpty(t *testing.T) {
	root := t.TempDir()
	out, err := ListResumableSessions(root, 0)
	if err != nil {
		t.Fatalf("empty root should not error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("want empty slice, got %#v", out)
	}
}
