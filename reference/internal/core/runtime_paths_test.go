package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const (
	testSessionID = "0123456789abcdef0123456789abcdef"
	testMessageID = "abcdef0123456789abcdef0123456789"
)

func TestRuntimePathDerivation(t *testing.T) {
	root := t.TempDir()
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		got  func() (string, error)
		want string
	}{
		{"runtime", paths.RuntimeDir, filepath.Join(root, ".specd", "runtime")},
		{"sessions", paths.SessionsDir, filepath.Join(root, ".specd", "runtime", "sessions")},
		{"session directory", func() (string, error) { return paths.SessionDir(testSessionID) }, filepath.Join(root, ".specd", "runtime", "sessions", testSessionID)},
		{"session file", func() (string, error) { return paths.SessionPath(testSessionID) }, filepath.Join(root, ".specd", "runtime", "sessions", testSessionID, "session.json")},
		{"event", func() (string, error) { return paths.EventPath(testSessionID, 42, testMessageID) }, filepath.Join(root, ".specd", "runtime", "sessions", testSessionID, "events", "00000000000000000042-"+testMessageID+".json")},
		{"worker", func() (string, error) { return paths.WorkerDir(testSessionID, "worker-1") }, filepath.Join(root, ".specd", "runtime", "sessions", testSessionID, "workers", "worker-1")},
		{"lease", func() (string, error) { return paths.LeasePath(testSessionID, "worker-1") }, filepath.Join(root, ".specd", "runtime", "sessions", testSessionID, "workers", "worker-1", "lease.json")},
		{"cursor", func() (string, error) { return paths.CursorPath(testSessionID, "worker-1") }, filepath.Join(root, ".specd", "runtime", "sessions", testSessionID, "workers", "worker-1", "cursor.json")},
		{"artifact", func() (string, error) { return paths.ArtifactPath(testSessionID, "stdout-tail") }, filepath.Join(root, ".specd", "runtime", "sessions", testSessionID, "artifacts", "stdout-tail")},
		{"archive", func() (string, error) { return paths.ArchivePath(testSessionID) }, filepath.Join(root, ".specd", "runtime", "archives", testSessionID)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.got()
			if err != nil {
				t.Fatal(err)
			}
			if got != tt.want {
				t.Fatalf("path = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRuntimePathRejectsHostileIDs(t *testing.T) {
	root := t.TempDir()
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}

	invalidSegments := []string{
		"", ".", "..", "../escape", "nested/name", `nested\name`,
		filepath.Join(string(filepath.Separator), "absolute"),
		strings.Repeat("a", ACPRuntimeIDMaxBytes+1),
		"UPPER", "with space",
	}
	for _, value := range invalidSegments {
		t.Run("worker_"+value, func(t *testing.T) {
			if _, err := paths.WorkerDir(testSessionID, value); err == nil {
				t.Fatalf("WorkerDir accepted %q", value)
			}
		})
		t.Run("artifact_"+value, func(t *testing.T) {
			if _, err := paths.ArtifactPath(testSessionID, value); err == nil {
				t.Fatalf("ArtifactPath accepted %q", value)
			}
		})
	}

	invalidOpaqueIDs := []string{
		"", "..", "../escape", testSessionID + "0",
		"g123456789abcdef0123456789abcdef",
		filepath.Join(string(filepath.Separator), testSessionID),
	}
	for _, value := range invalidOpaqueIDs {
		t.Run("session_"+value, func(t *testing.T) {
			if _, err := paths.SessionDir(value); err == nil {
				t.Fatalf("SessionDir accepted %q", value)
			}
		})
		t.Run("message_"+value, func(t *testing.T) {
			if _, err := paths.EventPath(testSessionID, 1, value); err == nil {
				t.Fatalf("EventPath accepted %q", value)
			}
		})
	}
	if _, err := paths.EventPath(testSessionID, 0, testMessageID); err == nil {
		t.Fatal("EventPath accepted sequence zero")
	}
}

func TestRuntimePathRejectsSymlinkEscape(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	runtime := filepath.Join(root, ".specd", "runtime")
	if err := os.MkdirAll(filepath.Dir(runtime), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, runtime); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := NewACPRuntimePaths(root); err == nil {
		t.Fatal("NewACPRuntimePaths accepted symlinked runtime directory")
	}
}

func TestRuntimePathRejectsNestedSymlink(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	sessions := filepath.Join(root, ".specd", "runtime", "sessions")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(sessions, testSessionID)); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := paths.EventPath(testSessionID, 1, testMessageID); err == nil {
		t.Fatal("EventPath accepted a symlinked session directory")
	}
}

func TestRuntimePathRejectsSymlinkedSpecdDirectory(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(root, ".specd")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if _, err := NewACPRuntimePaths(root); err == nil {
		t.Fatal("NewACPRuntimePaths accepted symlinked .specd directory")
	}
}

func TestRuntimePathRootMayBeReachedThroughSymlink(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "project")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(parent, "project-link")
	if err := os.Symlink(root, alias); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	paths, err := NewACPRuntimePaths(alias)
	if err != nil {
		t.Fatal(err)
	}
	got, err := paths.RuntimeDir()
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(root, ".specd", "runtime")
	if got != want {
		t.Fatalf("RuntimeDir() = %q, want canonical %q", got, want)
	}
}
