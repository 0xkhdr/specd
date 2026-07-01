package cmd_test

import (
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/cmd"
	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

func TestHandshakeCommand(t *testing.T) {
	t.Run("bootstrap_json_includes_runtime_contract", func(t *testing.T) {
		h := th.New(t)
		res := h.RunExpect(core.ExitOK, "handshake", "bootstrap", "--json")
		for _, want := range []string{"\"version\"", "\"load\"", "\"config\""} {
			if !strings.Contains(res.Stdout, want) {
				t.Fatalf("handshake bootstrap missing %s:\n%s", want, res.Stdout)
			}
		}
	})

	t.Run("policy_human_output_mentions_digest", func(t *testing.T) {
		h := th.New(t)
		res := h.RunExpect(core.ExitOK, "handshake", "policy")
		if !strings.Contains(res.Stdout, "handshake policy:") || !strings.Contains(res.Stdout, "digest=") {
			t.Fatalf("unexpected handshake policy output:\n%s", res.Stdout)
		}
	})

	t.Run("policy_json_can_scope_to_spec", func(t *testing.T) {
		h := th.New(t)
		h.Spec("auth").
			Req("Login", "As a user, I want login", "THE SYSTEM SHALL authenticate users.").
			FullDesign().
			AddTask(th.TaskSpec{ID: "T1", Verify: "true", Requirements: []int{1}}).
			Status(core.StatusExecuting).
			Build()
		res := h.RunExpect(core.ExitOK, "handshake", "policy", "auth", "--json")
		if !strings.Contains(res.Stdout, "\"slug\": \"auth\"") || !strings.Contains(res.Stdout, "\"recommendedCommandFamily\"") {
			t.Fatalf("unexpected scoped handshake policy:\n%s", res.Stdout)
		}
	})

	t.Run("bootstrap_human_output_points_to_json", func(t *testing.T) {
		h := th.New(t)
		res := h.RunExpect(core.ExitOK, "handshake", "bootstrap")
		if !strings.Contains(res.Stdout, "run with --json") {
			t.Fatalf("unexpected handshake bootstrap output:\n%s", res.Stdout)
		}
	})

	t.Run("unknown_subcommand_is_usage", func(t *testing.T) {
		h := th.New(t)
		h.RunExpect(core.ExitUsage, "handshake", "wat")
	})
}

func TestMCPConfigCommand(t *testing.T) {
	t.Run("known_host_prints_config", func(t *testing.T) {
		out := th.CaptureStdout(t, func() {
			if got := cmd.RunMCP(cli.ParseArgs([]string{"--config", "claude-code", "--root", "/repo"})); got != core.ExitOK {
				t.Fatalf("RunMCP config = %d, want %d", got, core.ExitOK)
			}
		})
		if !strings.Contains(out, "# Paste into:") || !strings.Contains(out, "/repo") {
			t.Fatalf("unexpected mcp config output:\n%s", out)
		}
	})

	t.Run("unknown_host_is_usage", func(t *testing.T) {
		stderr := th.CaptureStderr(t, func() {
			if got := cmd.RunMCP(cli.ParseArgs([]string{"--config", "nope"})); got != core.ExitUsage {
				t.Fatalf("RunMCP unknown config = %d, want %d", got, core.ExitUsage)
			}
		})
		if !strings.Contains(stderr, "unknown host") {
			t.Fatalf("stderr missing unknown host: %q", stderr)
		}
	})

	t.Run("bad_root_is_usage", func(t *testing.T) {
		stderr := th.CaptureStderr(t, func() {
			if got := cmd.RunMCP(cli.ParseArgs([]string{"--root", "/definitely/not/here"})); got != core.ExitUsage {
				t.Fatalf("RunMCP bad root = %d, want %d", got, core.ExitUsage)
			}
		})
		if !strings.Contains(stderr, "cannot use --root") {
			t.Fatalf("stderr missing root error: %q", stderr)
		}
	})

	t.Run("invalid_pinned_spec_is_usage", func(t *testing.T) {
		stderr := th.CaptureStderr(t, func() {
			if got := cmd.RunMCP(cli.ParseArgs([]string{"--spec", "../bad", "--http", "127.0.0.1:0"})); got != core.ExitUsage {
				t.Fatalf("RunMCP invalid spec = %d, want %d", got, core.ExitUsage)
			}
		})
		if !strings.Contains(stderr, "invalid slug") {
			t.Fatalf("stderr missing slug error: %q", stderr)
		}
	})
}
