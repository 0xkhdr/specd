# spec.md — Automatic Rollback on Failed Verify

**Status:** proposed
**Source:** specd-report.html §8 idea **D1** (impact: high · effort: low · moat: med)
**Date:** 2026-06-16
**Scope:** `--revert-on-fail` option on `specd verify`; `internal/cmd/verify.go`.

---

## 1. Objective

Since verify records git HEAD, offer `specd verify --revert-on-fail` to
auto-restore the working tree (or stash) when a task's verify fails — leaving the
repo clean for the next attempt instead of half-applied. Failed agent attempts
leave messy trees that poison the next task; the harness has the HEAD, so it can
guarantee a clean retry surface.

> **Hard invariant:** deterministic, evidence-gated, and **safe — never destroy
> unrecoverable work**. Rollback is **opt-in** (`--revert-on-fail` / config),
> default off. It SHALL prefer a recoverable `git stash` over a hard reset so the
> reverted changes are retrievable, and SHALL refuse to act if the repo is not a
> clean-enough git state to revert safely. The evidence gate is unchanged: a
> failed verify still records a failing record.

## 2. Context

- `RunVerify` (`internal/cmd/verify.go`) runs the task command and records exit
  code + git HEAD into `VerificationRecord` (`state.go`). HEAD capture already
  shells to `git`.
- SECURITY.md / the regression spec stress not weakening the verify path.

## 3. Requirements (EARS)

- **R1 (H)** WHERE `--revert-on-fail` (or `verify.revertOnFail` config) is set
  AND a verify exits non-zero, THE SYSTEM SHALL restore the working tree to the
  state captured at the start of the verify.
- **R2 (H)** THE SYSTEM SHALL perform the restore via a recoverable `git stash`
  (not `reset --hard`) so the reverted changes remain retrievable, and SHALL
  print the stash ref in the output.
- **R3 (H)** IF the repository is in a state where a safe revert cannot be
  guaranteed (e.g. not a git repo, detached/merge in progress, or untracked
  conflicts), THE SYSTEM SHALL skip the revert and warn — never partial-revert.
- **R4 (M)** WHERE `--revert-on-fail` is unset (default), behavior SHALL be
  byte-identical to today.
- **R5 (M)** WHEN a verify passes, THE SYSTEM SHALL NOT touch the working tree
  regardless of the flag.
- **R6 (M)** THE SYSTEM SHALL record in the `VerificationRecord` that a revert
  occurred and the stash ref, so the action is auditable.

## 4. Design / approach

1. **Capture base** — before running the command, record the working-tree state
   marker (HEAD already captured; note dirty state).
2. **On non-zero exit + flag** — `git stash push -u` the changes; capture the
   stash ref; print it. The failing record is still written (evidence gate
   intact).
3. **Safety guard** — pre-check `git rev-parse`, merge/rebase in progress, repo
   presence; on any doubt, skip + warn (R3).
4. **Record** — add `Reverted bool` + `StashRef string` (`omitempty`) to the
   verify record.

## 5. Non-goals

- No `reset --hard` / destructive restore; reverts must be recoverable.
- No auto-revert by default; opt-in only.
- No change to the evidence gate — failed verify still fails.

## 6. Acceptance criteria

- With `--revert-on-fail`, a failing verify stashes the changes, prints + records
  the stash ref, and leaves a clean tree; the failing record is still written.
- A passing verify never touches the tree.
- Unsafe repo state ⇒ skip + warn, no partial revert.
- Flag unset ⇒ byte-identical to today; `make ci` green; stdlib-only.
