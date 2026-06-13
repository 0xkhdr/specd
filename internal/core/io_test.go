package core

import (
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWrite(t *testing.T) {
	t.Run("creates_parent_dirs_and_writes", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		path := filepath.Join(dir, "nested", "deep", "file.txt")

		// Act
		if err := AtomicWrite(path, "hello"); err != nil {
			t.Fatalf("AtomicWrite: %v", err)
		}

		// Assert
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		if string(got) != "hello" {
			t.Errorf("content = %q, want %q", got, "hello")
		}
	})

	t.Run("overwrites_atomically_leaving_no_temp_files", func(t *testing.T) {
		// Arrange
		dir := t.TempDir()
		path := filepath.Join(dir, "f.txt")
		if err := AtomicWrite(path, "v1"); err != nil {
			t.Fatal(err)
		}

		// Act
		if err := AtomicWrite(path, "v2"); err != nil {
			t.Fatal(err)
		}

		// Assert: content replaced and no leftover *.tmp turds in the dir.
		got, _ := os.ReadFile(path)
		if string(got) != "v2" {
			t.Errorf("content = %q, want v2", got)
		}
		entries, _ := os.ReadDir(dir)
		if len(entries) != 1 {
			t.Errorf("dir has %d entries, want 1 (no temp leftovers)", len(entries))
		}
	})
}

func TestReadOrDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	if got := ReadOrDefault(path, "fallback"); got != "fallback" {
		t.Errorf("missing file: got %q, want fallback", got)
	}
	_ = os.WriteFile(path, []byte("real"), 0o644)
	if got := ReadOrDefault(path, "fallback"); got != "real" {
		t.Errorf("present file: got %q, want real", got)
	}
}

func TestReadOrNull(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "x")
	if got := ReadOrNull(path); got != nil {
		t.Errorf("missing file: got %v, want nil", got)
	}
	_ = os.WriteFile(path, []byte("real"), 0o644)
	if got := ReadOrNull(path); got == nil || *got != "real" {
		t.Errorf("present file: got %v, want real", got)
	}
}

func TestAppendFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	if err := AppendFile(path, "a"); err != nil {
		t.Fatal(err)
	}
	if err := AppendFile(path, "b"); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "ab" {
		t.Errorf("content = %q, want ab", got)
	}
}
