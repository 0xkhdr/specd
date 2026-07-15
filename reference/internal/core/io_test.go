package core

import (
	"os"
	"path/filepath"
	"syscall"
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

	t.Run("writes_0644_not_temp_0600", func(t *testing.T) {
		// Arrange: pin umask 022 so the expected 0644 survives intact.
		old := syscall.Umask(0o022)
		defer syscall.Umask(old)
		dir := t.TempDir()
		path := filepath.Join(dir, "state.json")

		// Act
		if err := AtomicWrite(path, "{}"); err != nil {
			t.Fatal(err)
		}

		// Assert: not the CreateTemp default of 0600.
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if perm := fi.Mode().Perm(); perm != 0o644 {
			t.Errorf("perm = %o, want 0644", perm)
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

func TestAtomicWriteFailureModes(t *testing.T) {
	t.Run("parent_is_a_file_mkdirall_fails", func(t *testing.T) {
		// A regular file standing where a parent dir must be makes MkdirAll fail;
		// the error must propagate, not be swallowed.
		dir := t.TempDir()
		blocker := filepath.Join(dir, "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		err := AtomicWrite(filepath.Join(blocker, "child", "f.txt"), "data")
		if err == nil {
			t.Fatal("expected error when parent path is a file, got nil")
		}
	})

	t.Run("unwritable_dir_leaves_no_temp_file", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root bypasses directory permissions")
		}
		dir := t.TempDir()
		target := filepath.Join(dir, "ro")
		if err := os.Mkdir(target, 0o555); err != nil {
			t.Fatal(err)
		}
		defer os.Chmod(target, 0o755) // allow cleanup
		err := AtomicWrite(filepath.Join(target, "f.txt"), "data")
		if err == nil {
			t.Fatal("expected error writing into read-only dir, got nil")
		}
		// No temp turds left behind in the read-only dir.
		entries, _ := os.ReadDir(target)
		if len(entries) != 0 {
			t.Errorf("read-only dir has %d leftover entries, want 0", len(entries))
		}
	})
}

func TestAppendFileFailureModes(t *testing.T) {
	t.Run("parent_is_a_file", func(t *testing.T) {
		dir := t.TempDir()
		blocker := filepath.Join(dir, "blocker")
		if err := os.WriteFile(blocker, []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := AppendFile(filepath.Join(blocker, "child", "log"), "data"); err == nil {
			t.Fatal("expected MkdirAll error, got nil")
		}
	})

	t.Run("target_is_a_directory", func(t *testing.T) {
		// OpenFile on a directory for write fails; the error must propagate.
		dir := t.TempDir()
		target := filepath.Join(dir, "adir")
		if err := os.Mkdir(target, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := AppendFile(target, "data"); err == nil {
			t.Fatal("expected error appending to a directory, got nil")
		}
	})
}
