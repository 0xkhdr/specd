package main

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// silentRun executes run(argv) with stdout/stderr discarded so the dispatch
// surface is exercised for coverage without polluting test output.
func silentRun(argv []string) int {
	origOut, origErr := os.Stdout, os.Stderr
	devnull, _ := os.Open(os.DevNull)
	r, w, _ := os.Pipe()
	os.Stdout, os.Stderr = w, w
	done := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, r); close(done) }()
	code := run(argv)
	_ = w.Close()
	<-done
	os.Stdout, os.Stderr = origOut, origErr
	_ = devnull.Close()
	return code
}

// captureRun executes run(argv) and returns the combined stdout/stderr plus the
// exit code.
func captureRun(argv []string) (string, int) {
	origOut, origErr := os.Stdout, os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout, os.Stderr = w, w
	out := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		out <- string(b)
	}()
	code := run(argv)
	_ = w.Close()
	s := <-out
	os.Stdout, os.Stderr = origOut, origErr
	return s, code
}

func TestRunTopLevelExitCodes(t *testing.T) {
	tests := []struct {
		name string
		argv []string
		want int
	}{
		{"no_args_prints_help_usage", nil, core.ExitUsage},
		{"version_flag_ok", []string{"--version"}, core.ExitOK},
		{"version_word_ok", []string{"version"}, core.ExitOK},
		{"help_word_ok", []string{"help"}, core.ExitOK},
		{"help_flag_ok", []string{"--help"}, core.ExitOK},
		{"help_for_command_ok", []string{"help", "check"}, core.ExitOK},
		{"unknown_command_is_usage", []string{"definitely-not-a-command"}, core.ExitUsage},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := run(tt.argv); got != tt.want {
				t.Errorf("run(%v) = %d, want %d", tt.argv, got, tt.want)
			}
		})
	}
}

func TestRunHelpJSONAndErrors(t *testing.T) {
	cases := []struct {
		name string
		argv []string
		want int
	}{
		{"help_json_registry_dump", []string{"help", "--json"}, core.ExitOK},
		{"help_json_with_command", []string{"help", "check", "--json"}, core.ExitOK},
		{"help_unknown_command", []string{"help", "definitely-not-real"}, core.ExitUsage},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if got := silentRun(tt.argv); got != tt.want {
				t.Errorf("run(%v) = %d, want %d", tt.argv, got, tt.want)
			}
		})
	}
}

// TestRunDispatch drives the full run() → dispatch() → cmd.RunX path against a
// real seeded project, including the --json env-propagation branches.
func TestRunDispatch(t *testing.T) {
	t.Run("status against seeded spec", func(t *testing.T) {
		h := th.New(t)
		h.Spec("auth").Req("R", "story", "THE SYSTEM SHALL work.").Status(core.StatusRequirements).Build()
		if got := silentRun([]string{"status"}); got != core.ExitOK {
			t.Errorf("status = %d, want 0", got)
		}
	})

	t.Run("trailing --json flag enables json mode", func(t *testing.T) {
		h := th.New(t)
		h.Spec("auth").Req("R", "story", "THE SYSTEM SHALL work.").Status(core.StatusRequirements).Build()
		if got := silentRun([]string{"status", "auth", "--json"}); got != core.ExitOK {
			t.Errorf("status --json = %d, want 0", got)
		}
	})

	t.Run("leading --json token is treated as a global flag", func(t *testing.T) {
		// `specd --json status` now behaves like `specd status --json`: the
		// leading flag is stripped, the command runs, and JSON mode is on.
		h := th.New(t)
		h.Spec("auth").Req("R", "story", "THE SYSTEM SHALL work.").Status(core.StatusRequirements).Build()
		out, got := captureRun([]string{"--json", "status", "auth"})
		if got != core.ExitOK {
			t.Errorf("--json status auth = %d, want %d", got, core.ExitOK)
		}
		if !strings.Contains(out, "\"spec\": \"auth\"") {
			t.Errorf("leading --json did not produce JSON output: %q", out)
		}
	})

	t.Run("unknown spec propagates not-found", func(t *testing.T) {
		th.New(t)
		if got := silentRun([]string{"check", "ghost"}); got != core.ExitNotFound {
			t.Errorf("check ghost = %d, want %d", got, core.ExitNotFound)
		}
	})
}
