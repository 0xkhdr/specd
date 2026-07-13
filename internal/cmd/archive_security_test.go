package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

func TestArchiveCLIRejectsTraversal(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(core.SpecdDir(root), "specs", "next"), 0o755); err != nil {
		t.Fatal(err)
	}
	flags := map[string]string{"successor": "next", "owner": "team", "evidence": "artifact:x"}
	for _, slug := range []string{"../escape", "a/b", "."} {
		if err := runArchive(root, []string{slug}, flags); err == nil {
			t.Fatalf("accepted archive traversal %q", slug)
		}
	}
	for _, successor := range []string{"../escape", "a/b", "."} {
		bad := map[string]string{"successor": successor, "owner": "team", "evidence": "artifact:x"}
		if err := runArchive(root, []string{"old"}, bad); err == nil {
			t.Fatalf("accepted successor traversal %q", successor)
		}
	}
}
