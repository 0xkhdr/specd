package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSaveLoadState(t *testing.T) {
	dir := t.TempDir()
	slug := "test-spec"
	specPath := filepath.Join(dir, ".specd", "specs", slug)
	os.MkdirAll(specPath, 0o755)

	state := InitialState(slug, "Test Spec")
	if err := SaveState(dir, slug, &state); err != nil {
		t.Fatalf("SaveState: %v", err)
	}
	if state.Revision != 1 {
		t.Errorf("expected revision 1 after save, got %d", state.Revision)
	}

	loaded, err := LoadState(dir, slug)
	if err != nil {
		t.Fatalf("LoadState: %v", err)
	}
	if loaded.Spec != slug {
		t.Errorf("expected spec %q, got %q", slug, loaded.Spec)
	}
	if loaded.Status != StatusRequirements {
		t.Errorf("expected status requirements, got %s", loaded.Status)
	}
}

func TestCASConflict(t *testing.T) {
	dir := t.TempDir()
	slug := "test-spec"
	specPath := filepath.Join(dir, ".specd", "specs", slug)
	os.MkdirAll(specPath, 0o755)

	s1 := InitialState(slug, "Test")
	if err := SaveState(dir, slug, &s1); err != nil {
		t.Fatalf("first save: %v", err)
	}

	// Simulate a second writer that saves before us.
	s2 := InitialState(slug, "Test")
	s2.Revision = 1 // matches on-disk
	if err := SaveState(dir, slug, &s2); err != nil {
		t.Fatalf("second save: %v", err)
	}

	// Now try to save s1 again at revision 1 — should conflict (disk is now at revision 2).
	s1.Revision = 1
	err := SaveState(dir, slug, &s1)
	if err == nil {
		t.Error("expected CAS conflict error, got nil")
	}
}
