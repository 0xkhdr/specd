package cmd_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
	th "github.com/0xkhdr/specd/internal/testharness"
)

// seedFailingVerifySpec seeds a gate-valid spec parked at `executing` with a single
// task whose verify command always fails, so repeated `specd verify` runs drive
// the escalation fact counters.
func seedFailingVerifySpec(h *th.Harness, slug string) {
	h.Spec(slug).
		Req("Login", "As a user, I want to authenticate", "THE SYSTEM SHALL authenticate users.").
		FullDesign().
		AddTask(th.TaskSpec{ID: "T1", Title: "Implement", Verify: "false", Requirements: []int{1}, Status: core.TaskRunning}).
		Status(core.StatusExecuting).
		Build()
}

// TestReviewGate proves the V8 review gate: off by default completion is
// unaffected; required, verifying→complete is blocked until a fresh, approving
// review_report.md exists.
func TestReviewGate(t *testing.T) {
	t.Run("off_by_default", func(t *testing.T) {
		h := th.New(t)
		buildVerifyingSpec(h, "auth")
		h.RunExpect(core.ExitOK, "approve", "auth")
		h.State("auth").Status(core.StatusComplete)
	})

	t.Run("required_blocks_then_allows", func(t *testing.T) {
		t.Setenv("SPECD_REVIEW_REQUIRED", "true")
		h := th.New(t)
		buildVerifyingSpec(h, "auth")

		// No report yet → blocked.
		res := h.RunExpect(core.ExitGate, "approve", "auth")
		if !strings.Contains(res.Out(), "review gate") {
			t.Errorf("expected review-gate block, got %q", res.Out())
		}

		// Scaffold, then a revise verdict is still blocked.
		h.RunExpect(core.ExitOK, "review", "auth")
		res = h.RunExpect(core.ExitGate, "approve", "auth")
		if !strings.Contains(res.Out(), "verdict") {
			t.Errorf("expected verdict block on scaffold, got %q", res.Out())
		}

		// Flip verdict to approve → completes.
		path := core.ArtifactPath(h.Root, "auth", "review_report.md")
		body := core.ScaffoldReviewReport("auth")
		body = strings.Replace(body, "Verdict: revise", "Verdict: approve", 1)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		h.RunExpect(core.ExitOK, "approve", "auth")
		h.State("auth").Status(core.StatusComplete)
		if rv := h.State("auth").Raw().Review; rv == nil || rv.Verdict != "approve" {
			t.Fatalf("review record not persisted: %+v", rv)
		}
	})
}

// TestReviewChecklist proves the deterministic checklist extraction surface.
func TestReviewChecklist(t *testing.T) {
	h := th.New(t)
	buildVerifyingSpec(h, "auth")
	a := h.RunExpect(core.ExitOK, "review", "auth", "checklist", "--json")
	b := h.RunExpect(core.ExitOK, "review", "auth", "checklist", "--json")
	if a.Stdout != b.Stdout {
		t.Fatalf("checklist not deterministic:\n%s\n---\n%s", a.Stdout, b.Stdout)
	}
	if !strings.Contains(a.Stdout, "task T1") {
		t.Errorf("checklist missing task T1:\n%s", a.Stdout)
	}
}

// TestSecurityCheckWiring proves the `check --security` surface: off by default
// it reports the suite is off; enabled with a changed file bearing a secret it
// blocks and records the scan summary in state.
func TestSecurityCheckWiring(t *testing.T) {
	t.Run("off_reports_suite_off", func(t *testing.T) {
		h := th.New(t)
		seedFailingVerifySpec(h, "auth")
		res := h.RunExpect(core.ExitOK, "check", "auth", "--security")
		if !strings.Contains(res.Stdout, "suite off") {
			t.Errorf("expected suite-off notice, got %q", res.Stdout)
		}
	})

	t.Run("enabled_blocks_on_secret", func(t *testing.T) {
		if _, err := exec.LookPath("git"); err != nil {
			t.Skip("git unavailable")
		}
		t.Setenv("SPECD_SECURITY_SECRETS", "error")
		h := th.New(t)
		seedFailingVerifySpec(h, "auth")

		// Make the root a git repo with one committed baseline so a new file with a
		// secret shows up as a working-tree change the scanner reads.
		git := func(args ...string) {
			cmd := exec.Command("git", append([]string{"-C", h.Root}, args...)...)
			cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("git %v: %v\n%s", args, err, out)
			}
		}
		git("init")
		if err := os.WriteFile(filepath.Join(h.Root, "baseline.txt"), []byte("hello\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		git("add", "-A")
		git("commit", "-m", "baseline")
		// Now introduce a file containing an AWS-style key (uncommitted change).
		if err := os.WriteFile(filepath.Join(h.Root, "leak.txt"), []byte("aws_key = \"AKIAIOSFODNN7EXAMPLE\"\n"), 0o644); err != nil {
			t.Fatal(err)
		}

		res := h.RunExpect(core.ExitGate, "check", "auth", "--security")
		if !strings.Contains(res.Out(), "aws-access-key-id") {
			t.Errorf("expected aws key finding, got %q", res.Out())
		}
		sec := h.State("auth").Raw().Security
		if sec == nil || sec.Blocking == 0 {
			t.Fatalf("security scan not recorded/blocking: %+v", sec)
		}
	})
}

// TestSubmit proves the V7 submit surface: --dry-run prints the summary; with no
// command configured it errors; a configured command receives the summary on
// stdin and its non-zero exit fails the submit; a gate violation blocks before any
// exec (no partial state).
func TestSubmit(t *testing.T) {
	t.Run("dry_run_prints_summary", func(t *testing.T) {
		h := th.New(t)
		buildVerifyingSpec(h, "auth")
		res := h.RunExpect(core.ExitOK, "submit", "auth", "--dry-run")
		if !strings.Contains(res.Stdout, "specd — ") || !strings.Contains(res.Stdout, "Security:") {
			t.Errorf("dry-run missing summary sections:\n%s", res.Stdout)
		}
	})

	t.Run("no_command_configured", func(t *testing.T) {
		h := th.New(t)
		buildVerifyingSpec(h, "auth")
		res := h.RunExpect(core.ExitGate, "submit", "auth")
		if !strings.Contains(res.Out(), "submit.command") {
			t.Errorf("expected no-command error, got %q", res.Out())
		}
	})

	t.Run("command_receives_summary_on_stdin", func(t *testing.T) {
		h := th.New(t)
		buildVerifyingSpec(h, "auth")
		out := filepath.Join(h.Root, "pr-body.txt")
		t.Setenv("SPECD_SUBMIT_COMMAND", "cat > "+out)
		h.RunExpect(core.ExitOK, "submit", "auth")
		b, err := os.ReadFile(out)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(b), "specd — ") {
			t.Errorf("submit command did not receive the summary on stdin:\n%s", b)
		}
	})

	t.Run("nonzero_command_exit_fails", func(t *testing.T) {
		h := th.New(t)
		buildVerifyingSpec(h, "auth")
		t.Setenv("SPECD_SUBMIT_COMMAND", "exit 7")
		h.RunExpect(core.ExitGate, "submit", "auth")
	})

	t.Run("adversarial_command_gets_scrubbed_env", func(t *testing.T) {
		// A hostile submit.command trying to exfiltrate a host secret sees only the
		// scrubbed env — the secret must not leak into the child process.
		t.Setenv("MY_CLOUD_SECRET", "leak-me")
		h := th.New(t)
		buildVerifyingSpec(h, "auth")
		out := filepath.Join(h.Root, "exfil.txt")
		t.Setenv("SPECD_SUBMIT_COMMAND", "echo \"$MY_CLOUD_SECRET\" > "+out+"; cat >/dev/null")
		h.RunExpect(core.ExitOK, "submit", "auth")
		b, _ := os.ReadFile(out)
		if strings.Contains(string(b), "leak-me") {
			t.Fatalf("host secret leaked into submit command env: %q", b)
		}
	})
}

// TestEscalationOnRepeatedVerifyFail proves the V7 auto-escalation engine: two
// failed verifies with the engine enabled record a state.escalation and surface a
// conductor-handoff notice, and orchestrate resume --override clears it. Off by
// default, no escalation is ever recorded.
func TestEscalationOnRepeatedVerifyFail(t *testing.T) {
	t.Run("off_by_default", func(t *testing.T) {
		h := th.New(t)
		seedFailingVerifySpec(h, "auth")
		h.RunExpect(core.ExitGate, "verify", "auth", "T1")
		h.RunExpect(core.ExitGate, "verify", "auth", "T1")
		if esc := h.State("auth").Raw().Escalation; esc != nil {
			t.Fatalf("escalation recorded while disabled: %+v", esc)
		}
	})

	t.Run("fires_after_threshold_and_overrides", func(t *testing.T) {
		t.Setenv("SPECD_ESCALATION_ENABLED", "true")
		h := th.New(t)
		seedFailingVerifySpec(h, "auth")

		// First failure: below the default threshold (2) — no escalation yet.
		h.RunExpect(core.ExitGate, "verify", "auth", "T1")
		if esc := h.State("auth").Raw().Escalation; esc != nil {
			t.Fatalf("escalated on first failure: %+v", esc)
		}

		// Second failure: crosses the verify-fail threshold.
		res := h.RunExpect(core.ExitGate, "verify", "auth", "T1")
		if !strings.Contains(res.Out(), "escalation") {
			t.Errorf("expected escalation notice on second failure, got %q", res.Out())
		}
		esc := h.State("auth").Raw().Escalation
		if esc == nil || esc.Rule != core.RuleVerifyFail || esc.Task != "T1" {
			t.Fatalf("escalation record wrong: %+v", esc)
		}

		// Status surfaces it and recommends conductor.
		st := h.RunExpect(core.ExitOK, "orchestrate", "auth", "status")
		if !strings.Contains(st.Stdout, "escalated") || !strings.Contains(st.Stdout, "conductor") {
			t.Errorf("status should report escalation + conductor recommendation:\n%s", st.Stdout)
		}

		// resume without --override is refused (escalation is evidence).
		h.RunExpect(core.ExitGate, "orchestrate", "auth", "resume")
		if h.State("auth").Raw().Escalation == nil {
			t.Fatal("escalation cleared without --override")
		}

		// resume --override clears it.
		h.RunExpect(core.ExitOK, "orchestrate", "auth", "resume", "--override")
		if h.State("auth").Raw().Escalation != nil {
			t.Fatal("escalation not cleared after override")
		}
	})
}
