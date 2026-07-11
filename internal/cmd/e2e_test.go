package cmd

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xkhdr/specd/internal/core"
)

// TestLifecycleE2E drives init→new→check→approve→next→verify→report through a
// freshly built binary in a temp repo and asserts an on-disk side effect at
// every step. This is the evidence-integrity harness ADR-8 requires: a verb is
// done only when a running binary exercises it and leaves a trace.
func TestLifecycleE2E(t *testing.T) {
	if testing.Short() {
		t.Skip("builds the binary; skipped in -short")
	}
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	bin := filepath.Join(t.TempDir(), "specd")
	build := exec.Command("go", "build", "-o", bin, ".")
	build.Dir = repoRoot
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build specd: %v\n%s", err, out)
	}

	repo := t.TempDir()
	mustGit(t, repo, "init")
	mustGit(t, repo, "commit", "--allow-empty", "-m", "root", "--no-gpg-sign")

	run := func(args ...string) (string, int) {
		t.Helper()
		cmd := exec.Command(bin, args...)
		cmd.Dir = repo
		out, err := cmd.CombinedOutput()
		code := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else if err != nil {
			t.Fatalf("run %v: %v", args, err)
		}
		return string(out), code
	}
	exists := func(rel string) bool {
		_, err := os.Stat(filepath.Join(repo, rel))
		return err == nil
	}

	if _, code := run("init"); code != 0 || !exists(".specd") {
		t.Fatalf("init: code=%d specd-dir=%v", code, exists(".specd"))
	}
	if _, code := run("new", "demo"); code != 0 || !exists(".specd/specs/demo/state.json") {
		t.Fatalf("new: code=%d state=%v", code, exists(".specd/specs/demo/state.json"))
	}
	// Swap in a trivially-passing verify so the verify step exercises the real
	// runner without depending on a toolchain inside the temp repo.
	tasks := "# Tasks\n\n| id | role | files | depends-on | verify | acceptance |\n|---|---|---|---|---|---|\n| T1 | scout | spec.md | - | true | ok |\n"
	if err := os.WriteFile(filepath.Join(repo, ".specd/specs/demo/tasks.md"), []byte(tasks), 0o644); err != nil {
		t.Fatal(err)
	}
	// Author real requirements + design: the EARS and design-stub gates (W4)
	// refuse to check/approve an unedited scaffold stub.
	writeReal := func(name, body string) {
		if err := os.WriteFile(filepath.Join(repo, ".specd/specs/demo", name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeReal("requirements.md", "# Requirements — demo\n\n- **R1** When a user runs check, the system shall validate the spec.\n")
	writeReal("design.md", "# Design — demo\n\n## Modules\nThe check module runs gates.\n\n## On-disk contracts\nstate.json holds status.\n\n## Invariants\nOutput is deterministic.\n")

	if _, code := run("check", "demo"); code != 0 {
		t.Fatalf("check: code=%d", code)
	}
	if _, code := run("approve", "demo", "requirements"); code != 0 {
		t.Fatal("approve requirements failed")
	}
	if _, code := run("approve", "demo", "design"); code != 0 {
		t.Fatal("approve failed")
	}
	if out, _ := run("status", "demo"); !strings.Contains(out, "demo") {
		t.Fatalf("status missing spec: %s", out)
	}
	if out, code := run("next", "demo"); code != 0 || !strings.Contains(out, "T1") {
		t.Fatalf("next: code=%d out=%s", code, out)
	}
	if _, code := run("verify", "demo", "T1"); code != 0 || !exists(".specd/specs/demo/evidence.jsonl") {
		t.Fatalf("verify: code=%d evidence=%v", code, exists(".specd/specs/demo/evidence.jsonl"))
	}
	if out, code := run("report", "demo"); code != 0 || strings.TrimSpace(out) == "" {
		t.Fatalf("report: code=%d out=%q", code, out)
	}

	// R3.1: the approval record names the gate approved and the artifact
	// revision it approved, stamped with the provenance triple.
	state, err := core.LoadState(filepath.Join(repo, ".specd/specs/demo/state.json"))
	if err != nil {
		t.Fatalf("load state: %v", err)
	}
	var appr core.Record
	if err := json.Unmarshal(state.Records["approval:requirements"], &appr); err != nil {
		t.Fatalf("approval record: %v", err)
	}
	if appr.Gate != "requirements" {
		t.Fatalf("approval record missing gate: %+v", appr)
	}
	if appr.Timestamp == "" || appr.Actor == "" || appr.GitHead == "" {
		t.Fatalf("approval record not stamped: %+v", appr)
	}

	// R3.2/R3.3: the evidence ledger is append-only; a second verify appends a
	// line rather than rewriting, and every line pins to a commit (this repo
	// has a HEAD, so no record carries the "unknown" sentinel).
	if _, code := run("verify", "demo", "T1"); code != 0 {
		t.Fatalf("second verify failed")
	}
	ledger, err := os.ReadFile(filepath.Join(repo, ".specd/specs/demo/evidence.jsonl"))
	if err != nil {
		t.Fatalf("read ledger: %v", err)
	}
	if lines := strings.Count(strings.TrimSpace(string(ledger)), "\n") + 1; lines < 2 {
		t.Fatalf("ledger not append-only: %d lines\n%s", lines, ledger)
	}
	if strings.Contains(string(ledger), `"git_head":"unknown"`) {
		t.Fatalf("evidence carries unresolved head:\n%s", ledger)
	}

	// P4.2 close-the-loop: task complete writes the ✅ marker and state.json
	// status atomically, and the Sync gate stays green because they agree.
	if out, code := run("task", "complete", "demo", "T1"); code != 0 || !strings.Contains(out, "completed") {
		t.Fatalf("task complete: code=%d out=%q", code, out)
	}
	final, err := core.LoadState(filepath.Join(repo, ".specd/specs/demo/state.json"))
	if err != nil {
		t.Fatalf("reload state: %v", err)
	}
	if final.TaskStatus["T1"] != core.TaskComplete {
		t.Fatalf("state.json task status not recorded: %+v", final.TaskStatus)
	}
	if _, code := run("check", "demo"); code != 0 {
		t.Fatalf("check after complete (sync gate red?): code=%d", code)
	}

	// Fail-closed dispatch: unknown verb must exit 2, never 0.
	if _, code := run("bogusverb"); code != 2 {
		t.Fatalf("unknown verb exit = %d, want 2", code)
	}
}

func mustGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
