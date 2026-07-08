package core

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestLoadStateMigratesV5ToV6AndSaveWritesV6(t *testing.T) {
	root := t.TempDir()
	slug := "demo"
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(statePath(root, slug), []byte(`{
  "schemaVersion": 5,
  "revision": 7,
  "spec": "demo",
  "title": "Demo",
  "status": "requirements",
  "phase": "requirements",
  "gate": "none",
  "turn": 0,
  "createdAt": "2026-01-01T00:00:00Z",
  "updatedAt": "2026-01-01T00:00:00Z",
  "tasks": {},
  "blockers": []
}`), 0o644); err != nil {
		t.Fatalf("write v5 fixture: %v", err)
	}

	state, err := LoadState(root, slug)
	if err != nil {
		t.Fatalf("LoadState v5: %v", err)
	}
	if state.SchemaVersion != SchemaVersion {
		t.Fatalf("schemaVersion after load = %d, want %d", state.SchemaVersion, SchemaVersion)
	}
	if state.Revision != 7 {
		t.Fatalf("revision after load = %d, want 7", state.Revision)
	}
	if state.Evals != nil || state.Routing != nil || state.Conductor != nil || state.Escalation != nil {
		t.Fatalf("v6 optional blocks should be absent after v5 migration: %+v", state)
	}

	oldClock := Clock
	Clock = func() time.Time { return time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC) }
	defer func() { Clock = oldClock }()

	_, err = WithSpecLock[struct{}](root, slug, func() (struct{}, error) {
		return struct{}{}, SaveState(root, slug, state)
	})
	if err != nil {
		t.Fatalf("SaveState migrated v6: %v", err)
	}
	if state.Revision != 8 {
		t.Fatalf("revision after save = %d, want 8", state.Revision)
	}

	reloaded, err := LoadState(root, slug)
	if err != nil {
		t.Fatalf("reload v6: %v", err)
	}
	if reloaded.SchemaVersion != SchemaVersion {
		t.Fatalf("reloaded schemaVersion = %d, want %d", reloaded.SchemaVersion, SchemaVersion)
	}
	if reloaded.Revision != state.Revision {
		t.Fatalf("reloaded revision = %d, want %d", reloaded.Revision, state.Revision)
	}
}

func TestLoadStateAcceptsConductorExecutionMode(t *testing.T) {
	root := t.TempDir()
	slug := "demo"
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(statePath(root, slug), []byte(`{
  "schemaVersion": 6,
  "revision": 0,
  "spec": "demo",
  "title": "Demo",
  "status": "requirements",
  "phase": "requirements",
  "gate": "none",
  "turn": 0,
  "createdAt": "2026-01-01T00:00:00Z",
  "updatedAt": "2026-01-01T00:00:00Z",
  "tasks": {},
  "blockers": [],
  "executionMode": "conductor"
}`), 0o644); err != nil {
		t.Fatalf("write v6 fixture: %v", err)
	}

	state, err := LoadState(root, slug)
	if err != nil {
		t.Fatalf("LoadState conductor: %v", err)
	}
	if got := state.EffectiveMode(); got != ModeConductor {
		t.Fatalf("EffectiveMode = %q, want %q", got, ModeConductor)
	}
}

func TestLoadStateRejectsUnknownExecutionMode(t *testing.T) {
	root := t.TempDir()
	slug := "demo"
	if err := os.MkdirAll(SpecDir(root, slug), 0o755); err != nil {
		t.Fatalf("mkdir spec dir: %v", err)
	}
	if err := os.WriteFile(statePath(root, slug), []byte(`{
  "schemaVersion": 6,
  "revision": 0,
  "spec": "demo",
  "title": "Demo",
  "status": "requirements",
  "phase": "requirements",
  "gate": "none",
  "turn": 0,
  "createdAt": "2026-01-01T00:00:00Z",
  "updatedAt": "2026-01-01T00:00:00Z",
  "tasks": {},
  "blockers": [],
  "executionMode": "mystery"
}`), 0o644); err != nil {
		t.Fatalf("write v6 fixture: %v", err)
	}

	_, err := LoadState(root, slug)
	if err == nil {
		t.Fatal("LoadState unknown executionMode = nil, want error")
	}
	if !strings.Contains(err.Error(), "executionMode") || !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("LoadState error = %v, want unknown executionMode", err)
	}
}
