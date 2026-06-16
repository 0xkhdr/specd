package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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
	return time.Duration(core.EnvInt("SPECD_VERIFY_TIMEOUT_MS", 600_000, 1, 0)) * time.Millisecond
}

// scrubbedEnv is the cmd-layer alias for core.ScrubbedEnv — the shared env-scrub
// policy reused by verify execution and the custom-gate runner.
func scrubbedEnv() []string { return core.ScrubbedEnv() }

// changedFiles returns the working-tree paths that differ from HEAD (tracked
// modifications plus untracked files), sorted for deterministic evidence. It is
// best-effort: outside a git repo (or on any git error) it returns nil so the
// verify record simply omits the field.
func changedFiles(cwd string) []string {
	tracked, err := exec.Command("git", "-C", cwd, "diff", "--name-only", "HEAD").Output()
	if err != nil {
		return nil
	}
	untracked, _ := exec.Command("git", "-C", cwd, "ls-files", "--others", "--exclude-standard").Output()
	set := map[string]bool{}
	for _, blob := range [][]byte{tracked, untracked} {
		for _, line := range strings.Split(strings.TrimSpace(string(blob)), "\n") {
			if p := strings.TrimSpace(line); p != "" {
				set[p] = true
			}
		}
	}
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for p := range set {
		out = append(out, p)
	}
	sort.Strings(out)
	return out
}

// coverageRe matches Go's tooling coverage summary line, e.g.
// "coverage: 84.2% of statements". This is evidence-only; a miss yields
// "unavailable" and never fails the verify.
var coverageRe = regexp.MustCompile(`coverage:\s+(\d+(?:\.\d+)?)%`)

func parseCoverage(output string) string {
	m := coverageRe.FindAllStringSubmatch(output, -1)
	if len(m) == 0 {
		return "unavailable"
	}
	// Last reported percentage wins (final package / total line).
	return m[len(m)-1][1] + "%"
}

// revertSafety reports whether it is safe to auto-revert in cwd. It refuses
// outside a git work tree and during an in-progress merge/rebase/cherry-pick/
// bisect, where stashing could entangle or lose an operation the user is in the
// middle of. The returned reason is a human-readable explanation for the warn path.
func revertSafety(cwd string) (safe bool, reason string) {
	out, err := exec.Command("git", "-C", cwd, "rev-parse", "--is-inside-work-tree").Output()
	if err != nil || strings.TrimSpace(string(out)) != "true" {
		return false, "not a git work tree"
	}
	gitDirOut, err := exec.Command("git", "-C", cwd, "rev-parse", "--git-dir").Output()
	if err != nil {
		return false, "cannot resolve .git directory"
	}
	gitDir := strings.TrimSpace(string(gitDirOut))
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(cwd, gitDir)
	}
	inProgress := map[string]string{
		"MERGE_HEAD":       "a merge is in progress",
		"rebase-merge":     "a rebase is in progress",
		"rebase-apply":     "a rebase is in progress",
		"CHERRY_PICK_HEAD": "a cherry-pick is in progress",
		"BISECT_LOG":       "a bisect is in progress",
	}
	for marker, why := range inProgress {
		if _, err := os.Stat(filepath.Join(gitDir, marker)); err == nil {
			return false, why
		}
	}
	return true, ""
}

// stashWorkingTree stashes all changes (including untracked) and returns the
// recoverable stash reference, or ("", false) when there was nothing to stash.
func stashWorkingTree(cwd, label string) (ref string, stashed bool) {
	out, err := exec.Command("git", "-C", cwd, "stash", "push", "--include-untracked", "-m", label).CombinedOutput()
	if err != nil {
		return "", false
	}
	if strings.Contains(string(out), "No local changes to save") {
		return "", false
	}
	// Resolve the just-created stash to a stable commit hash so the reference
	// survives later stash operations that would renumber stash@{0}.
	hash, err := exec.Command("git", "-C", cwd, "rev-parse", "stash@{0}").Output()
	if err != nil {
		return "stash@{0}", true
	}
	return strings.TrimSpace(string(hash)), true
}

// maybeRevertOnFail applies the --revert-on-fail policy to a finished record. It
// only acts on a failed verify when the flag is set; passing/default runs are
// never touched. Unsafe repo states are skipped with a warning. It mutates rec
// in place (Reverted/StashRef) and prints the recovery hint.
func maybeRevertOnFail(root, slug, id string, args cli.Args, rec *core.VerificationRecord) {
	if rec.Verified || !args.Bool("revert-on-fail") {
		return
	}
	if safe, reason := revertSafety(root); !safe {
		core.Warn(fmt.Sprintf("--revert-on-fail: skipped (%s) — working tree left as-is", reason))
		return
	}
	ref, stashed := stashWorkingTree(root, fmt.Sprintf("specd revert-on-fail %s/%s", slug, id))
	if !stashed {
		core.Info("--revert-on-fail: no working-tree changes to revert")
		return
	}
	rec.Reverted = true
	rec.StashRef = ref
	fmt.Printf("  ↩ reverted working tree to a stash — recover with: git stash apply %s\n", ref)
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
	root, slug, code, ok := requireRootAndSlug(args, "usage: specd verify <slug> <id>  |  specd verify <slug> --criterion <req>.<n> --status pass|fail --evidence \"...\"")
	if !ok {
		return code
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

		if strings.ContainsRune(command, 0) {
			return specdExit(core.GateError(fmt.Sprintf("task %s: verify command contains a NUL byte — refusing to run", id))), nil
		}

		shell := strings.TrimSpace(os.Getenv("SPECD_VERIFY_SHELL"))
		if shell == "" {
			shell = "sh"
		}

		rec := runVerifyCommand(context.Background(), root, shell, command, core.NewShRunner())

		// On a failed verify, optionally stash the working tree (recoverable).
		// Never touches the tree on a passing or default run.
		maybeRevertOnFail(root, slug, id, args, rec)

		// Telemetry: count this verify run as a retry and record its elapsed time
		// via the injectable clock (deterministic under the test clock).
		tel := ensureTelemetry(&ts)
		tel.Retries++
		tel.VerifyDurationMs = core.DurationMsBetween(rec.RanAt, core.NowISO())

		// State mutation: persist the verification record under the spec lock.
		ts.Verification = rec
		state.Tasks[id] = ts
		if err := core.SaveState(root, slug, state); err != nil {
			return specdExit(err), err
		}

		// Presentation is factored out so this closure only does IO/state.
		return printVerifyResult(slug, id, rec), nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}

// printVerifyResult renders a verification record to stdout and returns the
// process exit code (OK when verified, Gate otherwise). It performs no spec-state
// IO — the caller owns persistence — mirroring the execution/presentation split
// already drawn by runVerifyCommand.
func printVerifyResult(slug, id string, rec *core.VerificationRecord) int {
	mark := "✗ FAILED"
	if rec.Verified {
		mark = "✓ verified"
	}
	to := ""
	if rec.TimedOut {
		to = " (timed out)"
	}
	fmt.Printf("%s — %s: `%s` → exit %d%s in %dms\n", mark, id, rec.Command, rec.ExitCode, to, rec.DurationMs)
	if rec.GitHead != nil {
		fmt.Printf("  gitHead: %s\n", *rec.GitHead)
	}
	if !rec.Verified && strings.TrimSpace(rec.StderrTail) != "" {
		fmt.Printf("  stderr tail:\n%s\n", rec.StderrTail)
	}
	if rec.Verified {
		fmt.Printf("  complete with: specd task %s %s --status complete\n", slug, id)
		return core.ExitOK
	}
	return core.ExitGate
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
		rc := core.ExitOK
		if status == "fail" {
			rc = core.ExitGate
		}
		return rc, nil
	})
	if err != nil {
		return specdExit(err)
	}
	return rc
}

// runVerifyCommand executes command in root with a scrubbed environment and a
// timeout, then returns the resulting VerificationRecord. Execution is delegated
// to a core.Runner (default: the shell runner, byte-identical to historical
// behaviour); this function owns policy (env scrub, timeout) and evidence
// capture (git head, changed files, coverage). It performs no state IO or
// presentation — callers handle persistence and output.
func runVerifyCommand(parent context.Context, root, shell, command string, runner core.Runner) *core.VerificationRecord {
	core.Info(fmt.Sprintf("run: %s -c %q  (cwd=%s, sandbox=%s)", shell, command, root, runner.Name()))
	startedAt := core.NowISO()

	res := runner.Run(parent, core.RunSpec{
		Root:    root,
		Shell:   shell,
		Command: command,
		Env:     scrubbedEnv(),
		Timeout: verifyTimeoutMs(),
	})

	rec := &core.VerificationRecord{
		Command:      command,
		ExitCode:     res.ExitCode,
		Verified:     res.ExitCode == 0 && !res.TimedOut,
		TimedOut:     res.TimedOut,
		StdoutTail:   tailStr(res.Stdout, 2000),
		StderrTail:   tailStr(res.Stderr, 2000),
		DurationMs:   res.DurationMs,
		RanAt:        startedAt,
		GitHead:      gitHead(root),
		ChangedFiles: changedFiles(root),
		Coverage:     parseCoverage(res.Stdout + res.Stderr),
	}
	// Only stamp Sandbox for an actually-isolating backend, so the default
	// ("none") run stays byte-identical to pre-sandbox records.
	if runner.Name() != "none" {
		rec.Sandbox = runner.Name()
	}
	return rec
}
