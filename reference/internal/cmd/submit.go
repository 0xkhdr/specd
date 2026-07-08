package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
	"github.com/0xkhdr/specd/internal/runner"
)

const submitUsage = "usage: specd submit <slug> [--waves w1,w2] [--dry-run] [--json]"

// submitTimeout bounds the configured submit command's wall clock.
const submitTimeout = 120 * time.Second

// RunSubmit implements `specd submit` (V7/P3.4): it validates that every gate is
// green for the spec (optionally scoped to a wave bundle), generates the
// deterministic PR summary, and streams it on stdin to the operator-configured
// `submit.command` run through the shared sandboxed exec path with a scrubbed
// env. No git/GitHub logic is embedded. A non-zero command exit is exit 1 with no
// partial state — the summary is regenerated from state, never persisted mid-run.
func RunSubmit(args cli.Args) int {
	root, slug, code, ok := requireRootAndSlug(args, submitUsage)
	if !ok {
		return code
	}
	if err := core.RequireSpec(root, slug); err != nil {
		return specdExit(err)
	}

	// Bundle gate validation: all configured gates must be green.
	ctx, pre, err := buildCheckCtx(root, slug, false)
	if err != nil {
		return specdExit(err)
	}
	violations, warnings := core.RunGates(ctx)
	violations = append(append([]core.Violation{}, pre...), violations...)
	if len(violations) > 0 {
		errLine("✗ submit blocked — %d gate violation(s); resolve them before submitting:", len(violations))
		for _, v := range violations {
			errLine("  %s: %s (%s)", v.Location, v.Message, v.Gate)
		}
		return core.ExitGate
	}

	summary := core.BuildPRSummary(ctx.State, violations, warnings, core.LinkCommits(gitRecentCommits(root)))
	body := summary.Markdown()

	if args.Bool("dry-run") {
		fmt.Print(body)
		return core.ExitOK
	}

	cfg := core.LoadConfig(root)
	command := strings.TrimSpace(cfg.Submit.Command)
	if command == "" {
		return specdExit(core.GateError("submit: no submit.command configured — set config.submit.command (e.g. `gh pr create --body-file -`) to enable submission"))
	}

	res := execSubmitCommand(root, command, cfg.Submit.Sandbox, body)
	if args.Bool("json") {
		out := map[string]interface{}{"ok": res.ExitCode == 0, "exitCode": res.ExitCode, "timedOut": res.TimedOut}
		if err := core.PrintJSON(out); err != nil {
			return specdExit(err)
		}
	} else if res.ExitCode == 0 {
		fmt.Printf("✓ submit: %s exited 0 for '%s'\n", firstWord(command), slug)
	} else {
		errLine("✗ submit: command exited %d%s\n%s", res.ExitCode, timedOutNote(res.TimedOut), strings.TrimSpace(res.Stderr))
	}
	if res.ExitCode != 0 {
		return core.ExitGate
	}
	return core.ExitOK
}

// execSubmitCommand runs the configured command through the shared sandboxed
// runner with a scrubbed env and the PR summary on stdin. An unavailable sandbox
// backend fails closed (consistent with verify/custom gates).
func execSubmitCommand(root, command, sandbox, stdin string) runner.RunResult {
	r, err := runner.SelectRunner(sandbox)
	if err != nil {
		return runner.RunResult{ExitCode: 1, Stderr: err.Error()}
	}
	shell := strings.TrimSpace(os.Getenv("SPECD_VERIFY_SHELL"))
	if shell == "" {
		shell = "sh"
	}
	return r.Run(context.Background(), runner.RunSpec{
		Root:    root,
		Shell:   shell,
		Command: command,
		Env:     core.ScrubbedEnv(),
		Timeout: submitTimeout,
		Stdin:   []byte(stdin),
	})
}

func firstWord(s string) string {
	if i := strings.IndexAny(s, " \t"); i >= 0 {
		return s[:i]
	}
	return s
}

func timedOutNote(t bool) string {
	if t {
		return " (timed out)"
	}
	return ""
}
