package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// write_failure_cov_test.go drives the persistence error branches the happy-path
// fixtures never reach, using the established read-only-directory injection (see
// TestACPStoreWritePermissionFailure). Each case makes a save target unwritable
// and asserts the surrounding lifecycle call fails closed rather than silently
// dropping state. Skipped under root, which bypasses directory permissions.

func TestOrchestrationSaveFailsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root := writePinkySpec(t)
	sessionID := strings.Repeat("8", 32)
	policy := validOrchestrationPolicy()
	clock := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	restore := setCoreClock(func() time.Time { return clock })
	t.Cleanup(restore)
	if _, err := StartOrchestrationSession(root, "demo", sessionID, "operator", policy); err != nil {
		t.Fatalf("start: %v", err)
	}

	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	sessionPath, err := paths.SessionPath(sessionID)
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Dir(sessionPath)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(dir, 0o700)

	if _, err := PauseOrchestration(root, sessionID); err == nil {
		t.Fatal("pause should fail when the session save target is read-only")
	}
}

func TestProgramSessionSaveFailsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	parentID := strings.Repeat("a", 32)

	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	sessionsDir, err := paths.ProgramSessionsDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(sessionsDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(sessionsDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(sessionsDir, 0o700)

	if _, err := PauseProgramOrchestration(root, parentID); err == nil {
		t.Fatal("pause should fail when the program session dir is read-only")
	}
}

func TestProgramChildLeaseSaveFailsClosed(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses directory permissions")
	}
	root := t.TempDir()
	scaffoldSpec(t, root, "a", StatusExecuting)
	cfg, _ := programTestPolicy(t)
	parentID := strings.Repeat("a", 32)

	if _, err := AcquireProgramChildLease(root, parentID, "a", cfg); err != nil {
		t.Fatalf("acquire: %v", err)
	}

	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	childDir, err := paths.ProgramChildDir("a")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(childDir, 0o500); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(childDir, 0o700)

	if _, err := ReleaseProgramChildLease(root, parentID, "a"); err == nil {
		t.Fatal("release should fail when the child lease dir is read-only")
	}
}
