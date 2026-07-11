package context

import (
	"os"
	"path/filepath"
	"testing"
)

// TestResolveSource (R2.2/R2.3): a required context source resolves beneath the
// declared repository base; traversal, absolute escape, symlink escape, and
// missing/unreadable targets fail closed with the item's identity attached.
func TestResolverResolvesSafely(t *testing.T) {
	root := t.TempDir()
	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	rel, err := ResolveSource(root, ".specd/specs/demo/tasks.md")
	if err != nil {
		t.Fatalf("valid source rejected: %v", err)
	}
	if rel != ".specd/specs/demo/tasks.md" {
		t.Fatalf("rel = %q", rel)
	}

	for _, bad := range []string{"../escape.md", "/etc/passwd", ".specd/specs/demo/nope.md"} {
		_, err := ResolveSource(root, bad)
		if err == nil {
			t.Fatalf("ResolveSource accepted unsafe/missing %q", bad)
		}
		re, ok := err.(ResolveError)
		if !ok || re.Source != bad {
			t.Fatalf("error must carry item identity, got %#v", err)
		}
	}

	// A symlink inside the tree pointing outside the repo base is refused.
	outside := t.TempDir()
	secret := filepath.Join(outside, "secret.md")
	if err := os.WriteFile(secret, []byte("s"), 0o644); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(specDir, "link.md")
	if err := os.Symlink(secret, link); err != nil {
		t.Skip("symlinks unsupported on this platform")
	}
	if _, err := ResolveSource(root, ".specd/specs/demo/link.md"); err == nil {
		t.Fatal("symlink escaping the repo base must be refused")
	}
}
