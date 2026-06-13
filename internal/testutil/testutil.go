// Package testutil provides hermetic helpers for specd tests: temporary
// .specd/ trees, spec scaffolding, and assertions. Nothing here touches the
// network, the real git config, or process-wide state beyond t.TempDir/t.Chdir.
package testutil

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// NewTempSpecdRoot creates an isolated project root with an empty .specd/
// directory and returns its absolute path. The directory is cleaned up by the
// test framework via t.TempDir.
func NewTempSpecdRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".specd", "specs"), 0o755); err != nil {
		t.Fatalf("NewTempSpecdRoot: %v", err)
	}
	return root
}

// Chdir switches the working directory to dir for the duration of the test,
// restoring the previous directory on cleanup. Tests that call this must not be
// parallel (working directory is process-global).
func Chdir(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("Chdir getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("Chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

// NewSpec scaffolds a spec under root with an initial state.json and returns the
// spec slug. It mirrors what `specd new` produces minus the artifact templates.
func NewSpec(t *testing.T, root, slug, title string) {
	t.Helper()
	if err := os.MkdirAll(core.SpecDir(root, slug), 0o755); err != nil {
		t.Fatalf("NewSpec mkdir: %v", err)
	}
	st := core.InitialState(slug, title)
	if err := core.SaveState(root, slug, &st); err != nil {
		t.Fatalf("NewSpec SaveState: %v", err)
	}
}

// WriteArtifact writes a spec artifact (e.g. "requirements.md") with the given
// content, creating the spec directory if needed.
func WriteArtifact(t *testing.T, root, slug, name, content string) {
	t.Helper()
	p := core.ArtifactPath(root, slug, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("WriteArtifact mkdir: %v", err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteArtifact: %v", err)
	}
}

// MustReadState loads and returns the spec state, failing the test on error.
func MustReadState(t *testing.T, root, slug string) *core.State {
	t.Helper()
	st, err := core.LoadState(root, slug)
	if err != nil {
		t.Fatalf("MustReadState: %v", err)
	}
	if st == nil {
		t.Fatalf("MustReadState: no state for %q", slug)
	}
	return st
}
