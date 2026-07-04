package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAtomicWriteCreatesParentAndWrites0644(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "state.json")

	if err := AtomicWrite(path, "first"); err != nil {
		t.Fatalf("AtomicWrite() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "first" {
		t.Fatalf("content = %q, want first", data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if got := info.Mode().Perm(); got != 0o644 {
		t.Fatalf("mode = %o, want 0644", got)
	}
}

func TestAtomicWriteReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if err := AtomicWrite(path, "new"); err != nil {
		t.Fatalf("AtomicWrite() error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "new" {
		t.Fatalf("content = %q, want new", data)
	}
}

func TestAtomicWriteCleansTempFileOnError(t *testing.T) {
	dir := t.TempDir()

	if err := AtomicWrite(dir, "cannot replace directory"); err == nil {
		t.Fatal("AtomicWrite() error = nil, want error")
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir() error = %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") && strings.HasSuffix(entry.Name(), ".tmp") {
			t.Fatalf("temp file was not cleaned up: %s", entry.Name())
		}
	}
}

func TestAtomicWriteReadOrNull(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing")
	if got := ReadOrNull(path); got != nil {
		t.Fatalf("ReadOrNull(missing) = %q, want nil", *got)
	}

	if err := AtomicWrite(path, "present"); err != nil {
		t.Fatalf("AtomicWrite() error = %v", err)
	}
	got := ReadOrNull(path)
	if got == nil || *got != "present" {
		t.Fatalf("ReadOrNull() = %v, want present", got)
	}
}

func TestAtomicWriteAppendFileAppendsAndCreatesParent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs", "verify.log")

	if err := AppendFile(path, "one\n"); err != nil {
		t.Fatalf("AppendFile(first) error = %v", err)
	}
	if err := AppendFile(path, "two\n"); err != nil {
		t.Fatalf("AppendFile(second) error = %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "one\ntwo\n" {
		t.Fatalf("content = %q, want appended lines", data)
	}
}
