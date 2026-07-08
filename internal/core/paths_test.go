package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFindRoot(t *testing.T) {
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(filepath.Join(root, specdDirName), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	got, err := FindRoot(nested)
	if err != nil {
		t.Fatalf("FindRoot: %v", err)
	}
	if got != root {
		t.Fatalf("FindRoot=%q want %q", got, root)
	}
	var nf NotFoundError
	if _, err := FindRoot(t.TempDir()); !errors.As(err, &nf) || nf.ExitCode() != 3 {
		t.Fatalf("missing root err=%T %[1]v", err)
	}
}

func TestSlug(t *testing.T) {
	valid := []string{"a", "a1", "spec-01", "0-a"}
	for _, slug := range valid {
		if !ValidSlug(slug) || ValidateSlug(slug) != nil {
			t.Fatalf("valid slug rejected: %q", slug)
		}
	}
	invalid := []string{"", "-", "-a", "a-", "a--b", "A", "a_b", "a.b"}
	// Path-traversal escapes must be rejected: a slug is a path component under
	// .specd/specs/<slug>/, so any `..`, absolute path, or separator that could
	// escape that directory is invalid (T-04-03, security trust boundary).
	traversal := []string{
		"..", "../x", "../../etc", "a/../b", "..\\x",
		"/etc/passwd", "/abs", "a/b", "spec/../..",
		".specd", "spec..d",
	}
	for _, slug := range append(invalid, traversal...) {
		if ValidSlug(slug) || ValidateSlug(slug) == nil {
			t.Fatalf("invalid slug accepted: %q", slug)
		}
	}
}
