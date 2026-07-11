package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestBrainReportProductionScopeRejectsUndeclared(t *testing.T) {
	root := newBrainTestRoot(t, "orchestrated", "orchestration:\n  enabled: true\nsecurity:\n  profile: production\n")
	tasks := "| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | craftsman | a.go | - | printf ok | R1 |\n"
	os.WriteFile(filepath.Join(root, ".specd/specs/demo/tasks.md"), []byte(tasks), 0o644)
	os.WriteFile(filepath.Join(root, "a.go"), []byte("a"), 0o644)
	gitInitRepo(t, root)
	execGit(t, root, "add", ".")
	execGit(t, root, "commit", "-m", "tracked")
	if err := runBrain(root, []string{"start", "demo"}, nil); err != nil {
		t.Fatal(err)
	}
	if err := runBrain(root, []string{"step", "demo"}, map[string]string{"authority": ""}); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "outside.go"), []byte("x"), 0o644)
	err := runTaskComplete(root, []string{"demo", "T1"}, nil)
	if err == nil || !strings.Contains(err.Error(), "outside_scope") {
		t.Fatalf("err=%v", err)
	}
}

func execGit(t *testing.T, root string, args ...string) {
	t.Helper()
	c := exec.Command("git", append([]string{"-C", root}, args...)...)
	if out, err := c.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v %s", args, err, out)
	}
}
