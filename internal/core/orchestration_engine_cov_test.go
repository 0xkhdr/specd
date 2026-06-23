package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// orchestration_engine_cov_test.go drives StartOrchestrationSession's
// fail-closed branches and the corrupt-file branches of the session/lease
// loaders by injecting garbage at their runtime paths.

func TestStartOrchestrationSessionFailClosed(t *testing.T) {
	root := writePinkySpec(t)
	policy := validOrchestrationPolicy()
	now := time.Date(2026, 6, 18, 12, 0, 0, 0, time.UTC)
	defer setCoreClock(func() time.Time { return now })()

	// Invalid policy fails before any write.
	if _, err := StartOrchestrationSession(root, "demo", strings.Repeat("1", 32), "cli", OrchestrationPolicy{}); err == nil {
		t.Error("zero policy should fail closed")
	}
	// Missing spec fails (LoadSpec).
	if _, err := StartOrchestrationSession(root, "ghost", strings.Repeat("1", 32), "cli", policy); err == nil {
		t.Error("missing spec should fail")
	}

	// First start succeeds; a second with the same ID is rejected.
	id := strings.Repeat("2", 32)
	if _, err := StartOrchestrationSession(root, "demo", id, "cli", policy); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if _, err := StartOrchestrationSession(root, "demo", id, "cli", policy); err == nil {
		t.Error("duplicate session ID should be rejected")
	}
	// A different session ID for the same spec collides with the active session.
	if _, err := StartOrchestrationSession(root, "demo", strings.Repeat("3", 32), "cli", policy); err == nil {
		t.Error("second active session for a spec should be rejected")
	}
}

func TestLoadOrchestrationSessionCorrupt(t *testing.T) {
	root := t.TempDir()
	id := strings.Repeat("4", 32)
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	path, err := paths.SessionPath(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	// Corrupt JSON → decode error.
	if err := os.WriteFile(path, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrchestrationSession(root, id); err == nil {
		t.Error("corrupt session should error")
	}
	// Well-formed JSON that fails validation → validation error.
	if err := os.WriteFile(path, []byte(`{"version":1,"sessionId":"`+id+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadOrchestrationSession(root, id); err == nil {
		t.Error("invalid session should error")
	}
	// Bad session ID is rejected before any read.
	if _, err := LoadOrchestrationSession(root, "short"); err == nil {
		t.Error("bad session id should error")
	}
}

func TestLoadProgramChildLeaseCorrupt(t *testing.T) {
	root := t.TempDir()
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	leasePath, err := paths.ProgramChildLeasePath("demo")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(leasePath), 0o700); err != nil {
		t.Fatal(err)
	}
	// Corrupt lease JSON.
	if err := os.WriteFile(leasePath, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadProgramChildLease(root, "demo"); err == nil {
		t.Error("corrupt lease should error")
	}
	// Identity mismatch: lease records a different slug than its directory.
	if err := os.WriteFile(leasePath, []byte(`{"version":1,"slug":"other"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, _, err := loadProgramChildLease(root, "demo"); err == nil {
		t.Error("lease identity mismatch should error")
	}
}

func TestLoadProgramSessionCorrupt(t *testing.T) {
	root := t.TempDir()
	id := strings.Repeat("5", 32)
	paths, err := NewACPRuntimePaths(root)
	if err != nil {
		t.Fatal(err)
	}
	path, err := paths.ProgramSessionPath(id)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{bad"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProgramSession(root, id); err == nil {
		t.Error("corrupt program session should error")
	}
	if err := os.WriteFile(path, []byte(`{"version":1}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadProgramSession(root, id); err == nil {
		t.Error("invalid program session should error")
	}
}
