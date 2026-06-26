package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/integration"
)

// claudeRuntime returns an init runtime whose registry detects only claude-code.
func claudeRuntime() onboardingRuntime {
	claude := &onboardingAdapter{name: "claude-code", detected: true, scopes: []integration.Scope{integration.ScopeProject}}
	return onboardingRuntime{
		Registry:    integration.MustRegistry(claude),
		Probe:       passingProbe,
		Input:       strings.NewReader(""),
		Interactive: func() bool { return false },
	}
}

func TestInitWritesClaudeMDWhenClaudeSelected(t *testing.T) {
	root := initTestRoot(t)
	_, _, code := captureOutput(t, func() int {
		return runInitWithRuntime(cli.ParseArgs([]string{"--agent", "claude-code", "--non-interactive"}), core.DefaultInitExecutor(), claudeRuntime())
	})
	if code != core.ExitOK {
		t.Fatalf("exit=%d", code)
	}

	claudeMD := filepath.Join(root, "CLAUDE.md")
	got, err := os.ReadFile(claudeMD)
	if err != nil {
		t.Fatalf("CLAUDE.md not written: %v", err)
	}
	content := string(got)
	if !strings.Contains(content, "@AGENTS.md") {
		t.Fatalf("CLAUDE.md does not import AGENTS.md:\n%s", content)
	}
	if !strings.Contains(content, "<!-- SPECD INIT: BEGIN") || !strings.Contains(content, "<!-- SPECD INIT: END") {
		t.Fatalf("CLAUDE.md missing managed markers:\n%s", content)
	}
	// AGENTS.md stays the single source of truth.
	if _, err := os.Stat(filepath.Join(root, "AGENTS.md")); err != nil {
		t.Fatalf("AGENTS.md missing: %v", err)
	}
}

func TestInitNoClaudeMDWhenClaudeAbsent(t *testing.T) {
	t.Run("agent_none", func(t *testing.T) {
		root := initTestRoot(t)
		runtime := onboardingRuntime{Registry: integration.MustRegistry(), Probe: passingProbe, Interactive: func() bool { return false }}
		_, _, code := captureOutput(t, func() int {
			return runInitWithRuntime(cli.ParseArgs([]string{"--agent", "none", "--non-interactive"}), core.DefaultInitExecutor(), runtime)
		})
		if code != core.ExitOK {
			t.Fatalf("exit=%d", code)
		}
		if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
			t.Fatalf("CLAUDE.md created without claude-code: %v", err)
		}
	})

	t.Run("non_claude_host", func(t *testing.T) {
		root := initTestRoot(t)
		codex := &onboardingAdapter{name: "codex", detected: true, scopes: []integration.Scope{integration.ScopeProject}}
		runtime := onboardingRuntime{Registry: integration.MustRegistry(codex), Probe: passingProbe, Interactive: func() bool { return false }}
		_, _, code := captureOutput(t, func() int {
			return runInitWithRuntime(cli.ParseArgs([]string{"--agent", "codex", "--non-interactive"}), core.DefaultInitExecutor(), runtime)
		})
		if code != core.ExitOK {
			t.Fatalf("exit=%d", code)
		}
		if _, err := os.Stat(filepath.Join(root, "CLAUDE.md")); !os.IsNotExist(err) {
			t.Fatalf("CLAUDE.md created for non-claude host: %v", err)
		}
	})
}

func TestInitClaudeMDIdempotentPreservesUserContent(t *testing.T) {
	root := initTestRoot(t)
	run := func() int {
		return runInitWithRuntime(cli.ParseArgs([]string{"--agent", "claude-code", "--non-interactive"}), core.DefaultInitExecutor(), claudeRuntime())
	}
	if _, _, code := captureOutput(t, run); code != core.ExitOK {
		t.Fatalf("first init exit=%d", code)
	}

	claudeMD := filepath.Join(root, "CLAUDE.md")
	original, _ := os.ReadFile(claudeMD)
	// User content outside the managed markers must survive re-runs.
	if err := os.WriteFile(claudeMD, []byte("preamble\n"+string(original)+"postamble\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, _, code := captureOutput(t, run); code != core.ExitOK {
		t.Fatalf("second init exit=%d", code)
	}
	got, _ := os.ReadFile(claudeMD)
	if !strings.HasPrefix(string(got), "preamble\n") || !strings.HasSuffix(string(got), "postamble\n") {
		t.Fatalf("re-run lost user content outside markers:\n%s", got)
	}
	if !strings.Contains(string(got), "@AGENTS.md") {
		t.Fatalf("re-run lost managed section:\n%s", got)
	}
}

func TestInitWritesRuntimeGitignore(t *testing.T) {
	root := initTestRoot(t)
	if _, _, code := captureInitOutput(t, cli.Args{Flags: map[string]string{}}, core.DefaultInitExecutor()); code != core.ExitOK {
		t.Fatalf("init exit=%d", code)
	}

	gitignore := filepath.Join(root, ".specd", "runtime", ".gitignore")
	got, err := os.ReadFile(gitignore)
	if err != nil {
		t.Fatalf("runtime .gitignore not written: %v", err)
	}
	content := string(got)
	if !strings.Contains(content, "*") || !strings.Contains(content, "!.gitignore") {
		t.Fatalf("runtime .gitignore missing ignore rules:\n%s", content)
	}

	// Prove the policy holds against real git: runtime contents are ignored
	// while the .gitignore itself stays tracked.
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "t@example.com"},
		{"config", "user.name", "t"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// A mission-like runtime file must be ignored.
	missionFile := filepath.Join(root, ".specd", "runtime", "missions", "spec-T1-1.json")
	if err := os.MkdirAll(filepath.Dir(missionFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(missionFile, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if checkIgnored(t, root, ".specd/runtime/missions/spec-T1-1.json") != true {
		t.Fatal("runtime mission file is not git-ignored")
	}
	// The ignore file itself must stay tracked (not ignored).
	if checkIgnored(t, root, ".specd/runtime/.gitignore") != false {
		t.Fatal("runtime .gitignore is ignored; policy not visible/tracked")
	}
}

// checkIgnored reports whether git ignores rel within root.
func checkIgnored(t *testing.T, root, rel string) bool {
	t.Helper()
	cmd := exec.Command("git", "check-ignore", rel)
	cmd.Dir = root
	err := cmd.Run()
	if err == nil {
		return true // exit 0 = path is ignored
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		return false // exit 1 = not ignored
	}
	t.Fatalf("git check-ignore %s failed: %v", rel, err)
	return false
}
