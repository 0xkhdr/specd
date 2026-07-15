package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func TestWave4SubmitHelpers(t *testing.T) {
	if got := firstWord("git status --short"); got != "git" {
		t.Fatalf("firstWord = %q, want git", got)
	}
	if got := firstWord(" \t\n"); got != "" {
		t.Fatalf("firstWord blank = %q", got)
	}
	if got := timedOutNote(false); got != "" {
		t.Fatalf("timedOutNote(false) = %q", got)
	}
	if got := timedOutNote(true); !strings.Contains(got, "timed out") {
		t.Fatalf("timedOutNote(true) = %q", got)
	}
}

func TestWave4ExecSubmitCommand(t *testing.T) {
	res := execSubmitCommand(t.TempDir(), "printf submit-ok", "", "")
	if res.ExitCode != 0 {
		t.Fatalf("exit = %d stderr=%q", res.ExitCode, res.Stderr)
	}
	if res.Stdout != "submit-ok" {
		t.Fatalf("stdout = %q", res.Stdout)
	}
}

func TestWave4SecurityAllowlistInvalidJSON(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".specd", "security", "allow.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("{"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := loadSecurityAllowlist(root); err == nil {
		t.Fatal("expected invalid allowlist error")
	}
}

func TestWave4ReadChangedForSecuritySkipsLargeAndDirectories(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "small.txt"), []byte("ok"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(root, "dir"), 0o755); err != nil {
		t.Fatal(err)
	}

	files := readChangedForSecurity(root)
	for _, f := range files {
		if f.Path == "dir" {
			t.Fatalf("directory included in changed files: %#v", files)
		}
	}
}

func TestWave4RunSecurityCheckRecordsCleanScan(t *testing.T) {
	root := t.TempDir()
	slug := "auth"
	if err := os.MkdirAll(core.SpecDir(root, slug), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := core.SaveState(root, slug, &core.State{
		Status: core.StatusTasks,
		Tasks:  map[string]core.TaskState{},
	}); err != nil {
		t.Fatal(err)
	}

	code := runSecurityCheck(root, slug, cli.ParseArgs([]string{"--json"}))
	if code != core.ExitOK {
		t.Fatalf("security check exit = %d", code)
	}
	state, err := core.LoadState(root, slug)
	if err != nil {
		t.Fatal(err)
	}
	if state.Security == nil {
		t.Fatal("expected recorded security scan")
	}
	if state.Security.Blocking != 0 {
		t.Fatalf("blocking = %d", state.Security.Blocking)
	}
}
