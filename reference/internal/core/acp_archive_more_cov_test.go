package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// acp_archive_more_cov_test.go drives the archive read error branches by
// archiving a session (retention 0 removes the live dir, forcing the archive
// path) and then corrupting the sealed manifest/events. ReplaySessionEvents
// must surface each corruption rather than return partial history.

func archiveDirForTest(t *testing.T, store *ACPStore, sessionID string) string {
	t.Helper()
	dir, err := store.paths.ArchivePath(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestReplayRejectsCorruptArchive(t *testing.T) {
	cases := []struct {
		name    string
		corrupt func(t *testing.T, archiveDir string)
	}{
		{"corrupt manifest json", func(t *testing.T, d string) {
			if err := os.WriteFile(filepath.Join(d, "manifest.json"), []byte("{not json"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"unsupported manifest version", func(t *testing.T, d string) {
			if err := os.WriteFile(filepath.Join(d, "manifest.json"), []byte(`{"version":999,"sessionId":"x"}`), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
		{"corrupt event payload", func(t *testing.T, d string) {
			entries, err := os.ReadDir(filepath.Join(d, "events"))
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) == 0 {
				t.Fatal("no archived events to corrupt")
			}
			if err := os.WriteFile(filepath.Join(d, "events", entries[0].Name()), []byte("{not json"), 0o600); err != nil {
				t.Fatal(err)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			store := newTestACPStore(t)
			now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
			defer setCoreClock(func() time.Time { return now })()
			sessionID := writeTerminalSession(t, store)
			if _, err := store.ArchiveSession(sessionID, 0); err != nil {
				t.Fatalf("archive: %v", err)
			}
			tc.corrupt(t, archiveDirForTest(t, store, sessionID))
			if _, err := store.ReplaySessionEvents(sessionID); err == nil {
				t.Fatalf("%s: replay should fail on a corrupt archive", tc.name)
			}
		})
	}
}
