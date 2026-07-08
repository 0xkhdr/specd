package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRevertOnFail(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")

	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-m", "base")

	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasks := "| id | role | files | depends-on | verify | acceptance |\n" +
		"|---|---|---|---|---|---|\n" +
		"| ⬜ T1 | builder | tracked.txt | - | printf changed > tracked.txt; false | fails after edit |\n"
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}

	err := runVerify(root, []string{"demo", "T1"}, map[string]string{"revert-on-fail": "true"})
	if err == nil {
		t.Fatalf("runVerify succeeded, want failure")
	}
	got, err := os.ReadFile(filepath.Join(root, "tracked.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "base\n" {
		t.Fatalf("tracked.txt = %q, want base restored", got)
	}
}

// TestVerifyFailureLeavesCleanTree pins deterministic cleanup (SPEC-03 T-03-03):
// a failing verify under --revert-on-fail must restore tracked state, release
// the per-spec lock, and leak no temp artifacts (git-apply .orig/.rej, stray
// tmp files, or a leftover specd.lock).
func TestVerifyFailureLeavesCleanTree(t *testing.T) {
	root := t.TempDir()
	runGit(t, root, "init")
	runGit(t, root, "config", "user.email", "specd@example.test")
	runGit(t, root, "config", "user.name", "specd")
	if err := os.WriteFile(filepath.Join(root, "tracked.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, root, "add", "tracked.txt")
	runGit(t, root, "commit", "-m", "base")

	specDir := filepath.Join(root, ".specd", "specs", "demo")
	if err := os.MkdirAll(specDir, 0o755); err != nil {
		t.Fatal(err)
	}
	tasks := "| id | role | files | depends-on | verify | acceptance |\n" +
		"|---|---|---|---|---|---|\n" +
		"| ⬜ T1 | builder | tracked.txt | - | printf changed > tracked.txt; false | fails after edit |\n"
	if err := os.WriteFile(filepath.Join(specDir, "tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := runVerify(root, []string{"demo", "T1"}, map[string]string{"revert-on-fail": "true"}); err == nil {
		t.Fatal("runVerify succeeded, want failure")
	}

	// Tracked file restored.
	if got, _ := os.ReadFile(filepath.Join(root, "tracked.txt")); string(got) != "base\n" {
		t.Fatalf("tracked.txt = %q, want base restored", got)
	}
	// No temp/lock artifacts anywhere in the tree.
	if err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if strings.HasSuffix(name, ".orig") || strings.HasSuffix(name, ".rej") ||
			name == "specd.lock" || strings.HasPrefix(name, "tmp") {
			t.Errorf("leaked artifact after verify+revert: %s", p)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func runGit(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
