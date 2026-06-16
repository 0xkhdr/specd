# tasks.md — Automatic Rollback on Failed Verify execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Verify-path recon

- [x] **T1 — Map RunVerify exit handling + git usage** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R1,R4
  - Report where exit code is evaluated, where the record is written, and the
    existing git HEAD-capture call. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<verify-path map>"`
  - **Evidence:** exit eval — `runVerifyCommand` `internal/cmd/verify.go:232-243`
    (`*exec.ExitError ⇒ ExitCode()` `verify.go:235`, non-exit/timeout ⇒ 124
    `verify.go:238`/`:242`); `Verified = exitCode==0 && !timedOut`
    `verify.go:248`. Record write — `ts.Verification = rec` + `SaveState`
    `verify.go:107-111`, inside `WithSpecLock` opened at `verify.go:76`. Existing
    git call — `gitHead(cwd)` `verify.go:48-58` (`git -C cwd rev-parse HEAD`),
    invoked at `verify.go:254` to stamp `VerificationRecord.GitHead`
    (`state.go:61`). The revert hook attaches after the non-zero exit at
    `verify.go:104` (record built) and before/around `SaveState`: pre-check repo
    safety, `git stash` to a recoverable ref on failure, and record
    `Reverted`/`StashRef` on the record — never `reset --hard`.

## Wave 2 — Safe revert

- [x] **T2 — Repo-safety pre-check (skip+warn on unsafe state)** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R3
  - Detect non-git / merge-rebase-in-progress; refuse to revert.
  - verify: `go test ./internal/cmd/ -run TestRevertSafetyGuard -race -count=1`
  - **Evidence:** `revertSafety(cwd)` in `verify.go` refuses outside a git work
    tree and on MERGE_HEAD/rebase-merge/rebase-apply/CHERRY_PICK_HEAD/BISECT_LOG,
    returning a human reason for the warn path. `TestRevertSafetyGuard` passes.

- [x] **T3 — `--revert-on-fail` recoverable stash on non-zero exit** ✓ complete · 2026-06-16
  - role: builder · depends: T2 · requirements: R1,R2,R4,R5
  - `git stash push -u`; print stash ref; pass/default untouched.
  - verify: `go test ./internal/cmd/ -run TestRevertOnFail -race -count=1`
  - **Evidence:** `maybeRevertOnFail` only acts on a failed verify when the flag
    is set; `stashWorkingTree` runs `git stash push --include-untracked` and
    resolves a stable commit hash; prints `git stash apply <ref>` hint. Passing
    and default runs never touch the tree (asserted). `TestRevertOnFail` passes.

- [x] **T4 — Record `Reverted`/`StashRef` in VerificationRecord** ✓ complete · 2026-06-16
  - role: builder · depends: T3 · requirements: R6
  - verify: `go test ./internal/core/ -run TestRevertRecord -race -count=2`
  - **Evidence:** `VerificationRecord.Reverted bool` + `StashRef string` (omitempty)
    + schema mirror; stamped by `maybeRevertOnFail`. `TestRevertRecord` passes.

## Wave 3 — Regression + review

- [ ] **T5 — Test: flag unset is byte-identical; pass never touches tree**
  - role: verifier · depends: T3 · requirements: R4,R5
  - verify: `go test ./... -run 'TestRevertDefaultRegression|TestRevertPassNoop' -race -count=2`

- [ ] **T6 — Review: no reset --hard, evidence gate intact**
  - role: reviewer · depends: T4,T5 · requirements: R2
  - verify: N/A — complete with `--unverified --evidence "<recoverable-only audit>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R4 |
| 2 | T2–T4 | R1, R2, R3, R4, R5, R6 |
| 3 | T5–T6 | R2, R4, R5 |
