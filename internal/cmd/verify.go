package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/0xkhdr/specd/internal/cli"
	"github.com/0xkhdr/specd/internal/core"
)

func tailStr(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return fmt.Sprintf("…(%d chars truncated)…\n", len(s)-max) + s[len(s)-max:]
}

func verifyTimeoutMs() time.Duration {
	v := strings.TrimSpace(os.Getenv("SPECD_VERIFY_TIMEOUT_MS"))
	if v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			return time.Duration(n) * time.Millisecond
		}
	}
	return 600 * time.Second
}

func gitHead(cwd string) *string {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "HEAD").Output()
	if err != nil {
		return nil
	}
	s := strings.TrimSpace(string(out))
	if s == "" {
		return nil
	}
	return &s
}

func RunVerify(args cli.Args) int {
	root, err := core.RequireSpecdRoot()
	if err != nil {
		return specdExit(err)
	}
	slug := ""
	if len(args.Pos) > 0 {
		slug = args.Pos[0]
	}
	if slug == "" {
		return usageExit("usage: specd verify <slug> <id>  |  specd verify <slug> --criterion <req>.<n> --status pass|fail --evidence \"...\"")
	}
	if args.Has("criterion") {
		return recordCriterion(root, slug, args)
	}
	id := ""
	if len(args.Pos) > 1 {
		id = args.Pos[1]
	}
	if id == "" {
		return usageExit("usage: specd verify <slug> <id>")
	}

	rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		loaded, err := core.LoadSpec(root, slug)
		if err != nil {
			return specdExit(err), err
		}
		state := loaded.State
		doc := loaded.Doc

		ts, ok := state.Tasks[id]
		docTask := core.FindTask(doc, id)
		if !ok || docTask == nil {
			return specdExit(core.NotFoundError(fmt.Sprintf("task '%s' not found in spec '%s'", id, slug))), nil
		}

		command := strings.TrimSpace(docTask.Meta["verify"])
		if command == "" || strings.ToUpper(command[:min(3, len(command))]) == "N/A" {
			return specdExit(core.GateError(fmt.Sprintf("task %s: verify is '%s' (no runnable command) — read-only roles complete with `specd task %s %s --status complete --unverified --evidence \"...\"`", id, command, slug, id))), nil
		}

		timeout := verifyTimeoutMs()
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()

		startedAt := core.NowISO()
		t0 := time.Now()

		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Dir = root
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		runErr := cmd.Run()
		durationMs := time.Since(t0).Milliseconds()

		timedOut := ctx.Err() == context.DeadlineExceeded
		exitCode := 0
		if runErr != nil {
			if exitErr, ok := runErr.(*exec.ExitError); ok {
				exitCode = exitErr.ExitCode()
			} else {
				exitCode = 124
			}
		}
		if timedOut {
			exitCode = 124
		}

		rec := &core.VerificationRecord{
			Command:    command,
			ExitCode:   exitCode,
			Verified:   exitCode == 0 && !timedOut,
			TimedOut:   timedOut,
			StdoutTail: tailStr(stdout.String(), 2000),
			StderrTail: tailStr(stderr.String(), 2000),
			DurationMs: durationMs,
			RanAt:      startedAt,
			GitHead:    gitHead(root),
		}

		ts.Verification = rec
		state.Tasks[id] = ts
		if err := core.SaveState(root, slug, state); err != nil {
			return specdExit(err), err
		}

		mark := "✗ FAILED"
		if rec.Verified {
			mark = "✓ verified"
		}
		to := ""
		if timedOut {
			to = " (timed out)"
		}
		fmt.Printf("%s — %s: `%s` → exit %d%s in %dms\n", mark, id, command, exitCode, to, durationMs)
		if rec.GitHead != nil {
			fmt.Printf("  gitHead: %s\n", *rec.GitHead)
		}
		if !rec.Verified && strings.TrimSpace(rec.StderrTail) != "" {
			fmt.Printf("  stderr tail:\n%s\n", rec.StderrTail)
		}
		if rec.Verified {
			fmt.Printf("  complete with: specd task %s %s --status complete\n", slug, id)
		}
		rc := 1
		if rec.Verified {
			rc = 0
		}
		return rc, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}

var criterionRE = regexp.MustCompile(`^(\d+)\.(\d+)$`)

func recordCriterion(root, slug string, args cli.Args) int {
	key := args.Str("criterion")
	m := criterionRE.FindStringSubmatch(key)
	if m == nil {
		return usageExit(fmt.Sprintf("--criterion must be <requirement>.<n> (e.g. 1.2), got '%s'", key))
	}
	status := args.Str("status")
	if status != "pass" && status != "fail" {
		return usageExit("--status must be 'pass' or 'fail'")
	}
	evidence := args.Str("evidence")
	if evidence == "" {
		return usageExit("--evidence \"<proof>\" is required when recording a criterion")
	}
	req, _ := strconv.Atoi(m[1])
	crit, _ := strconv.Atoi(m[2])

	rc, err := core.WithSpecLock[int](root, slug, func() (int, error) {
		loaded, err := core.LoadSpec(root, slug)
		if err != nil {
			return specdExit(err), err
		}
		state := loaded.State
		reqMd := core.ReadArtifact(root, slug, "requirements.md")
		if reqMd != nil {
			nums := core.RequirementNumbers(*reqMd)
			if !nums[req] {
				return specdExit(core.GateError(fmt.Sprintf("requirement %d is not defined in requirements.md", req))), nil
			}
		}
		if state.Acceptance == nil {
			state.Acceptance = map[string]core.CriterionRecord{}
		}
		state.Acceptance[key] = core.CriterionRecord{
			Requirement: req,
			Criterion:   crit,
			Status:      status,
			Evidence:    evidence,
			RanAt:       core.NowISO(),
		}
		if err := core.SaveState(root, slug, state); err != nil {
			return specdExit(err), err
		}
		icon := "✗ fail"
		if status == "pass" {
			icon = "✓ pass"
		}
		fmt.Printf("%s — criterion %s (requirement %d) recorded.\n", icon, key, req)
		rc := 0
		if status == "fail" {
			rc = 1
		}
		return rc, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
