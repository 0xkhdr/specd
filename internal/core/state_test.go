package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestStateCAS(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	state := InitialState("demo")

	if err := SaveStateCAS(path, 0, state); err != nil {
		t.Fatalf("initial CAS save: %v", err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	if loaded.Revision != 1 {
		t.Fatalf("revision = %d, want 1", loaded.Revision)
	}

	loaded.Status = StatusDesign
	loaded.Phase = PhaseForStatus(StatusDesign)
	if err := SaveStateCAS(path, 1, loaded); err != nil {
		t.Fatalf("second CAS save: %v", err)
	}
	loaded, err = LoadState(path)
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if loaded.Revision != 2 {
		t.Fatalf("revision = %d, want 2", loaded.Revision)
	}

	if err := SaveStateCAS(path, 1, loaded); !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("stale CAS error = %v, want ErrRevisionConflict", err)
	}
}

func TestStateRejectsInvalidSchema(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":99}`), 0o644); err != nil {
		t.Fatalf("write invalid state: %v", err)
	}
	if _, err := LoadState(path); err == nil {
		t.Fatal("LoadState accepted unsupported schema")
	}
}
