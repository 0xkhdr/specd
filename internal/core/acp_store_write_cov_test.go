package core

import (
	"os"
	"path/filepath"
	"testing"
)

// acp_store_write_cov_test.go covers the immutability/regular-file guards in
// writeImmutablePrivate and atomicWritePrivate.

func TestWriteImmutablePrivateRejectsRewrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "event.json")

	if err := writeImmutablePrivate(path, []byte("first")); err != nil {
		t.Fatalf("first write: %v", err)
	}
	// An immutable target must never be rewritten.
	if err := writeImmutablePrivate(path, []byte("second")); err == nil {
		t.Fatal("rewriting an immutable file should error")
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "first" {
		t.Fatalf("content drifted: %q err=%v", got, err)
	}
}

func TestAtomicWritePrivateRejectsNonRegular(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	// A directory at the path is non-regular → refused.
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := atomicWritePrivate(path, []byte("data")); err == nil {
		t.Fatal("replacing a directory should error")
	}

	// Replacing a regular file is fine and overwrites atomically.
	reg := filepath.Join(dir, "regular")
	if err := atomicWritePrivate(reg, []byte("v1")); err != nil {
		t.Fatal(err)
	}
	if err := atomicWritePrivate(reg, []byte("v2")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(reg)
	if err != nil || string(got) != "v2" {
		t.Fatalf("overwrite failed: %q err=%v", got, err)
	}
}
