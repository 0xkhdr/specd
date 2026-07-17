package scope

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDiffRenameDeleteUntracked(t *testing.T) {
	root := t.TempDir()
	git := func(a ...string) string {
		c := exec.Command("git", append([]string{"-C", root}, a...)...)
		c.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		o, e := c.CombinedOutput()
		if e != nil {
			t.Fatalf("git %v: %v %s", a, e, o)
		}
		return string(o)
	}
	git("init")
	if err := os.WriteFile(filepath.Join(root, "a"), []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "gone"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", ".")
	git("commit", "-m", "base")
	head := strings.TrimSpace(git("rev-parse", "HEAD"))
	if err := os.Rename(filepath.Join(root, "a"), filepath.Join(root, "b")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "gone")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "new"), []byte("n"), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := Derive(root, head)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"a", "b", "gone", "new"}
	if strings.Join(d.Paths, ",") != strings.Join(want, ",") {
		t.Fatalf("paths=%v", d.Paths)
	}
}
