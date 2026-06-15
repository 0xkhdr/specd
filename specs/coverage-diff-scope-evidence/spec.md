# spec.md — Coverage & Diff-scope Evidence

**Status:** proposed
**Source:** specd-report.html §8 idea **B2** (impact: high · effort: med · moat: med)
**Date:** 2026-06-16
**Scope:** extend `VerificationRecord` + `internal/cmd/verify.go`; new scope gate.

---

## 1. Objective

Have `verify` also capture a coverage delta and assert the task's `files:`
contract — flagging when a builder touched files outside its declared scope.
Scope creep and untested branches are where agents quietly go wrong; the harness
is the right place to catch both, deterministically. The verify record already
holds the git HEAD — add the changed-file set and an optional coverage figure.

> **Hard invariant:** deterministic, stdlib-only, evidence-gated. Changed-file
> detection uses `git` (already a dependency of the verify flow via HEAD
> capture); coverage is an **optional** captured number, never a hard floor
> baked into the binary. The scope assertion is a gate, not a silent mutation.

## 2. Context

- `VerificationRecord` (`internal/core/state.go`) records exit code, output
  tail, and git HEAD today.
- Each task declares a `files:` contract (`tasksparser.go`); the report (B2 in
  the source) notes the record "already holds HEAD; add changed-file set +
  coverage."
- `RunVerify` (`internal/cmd/verify.go`) runs the task's command and writes the
  record.

## 3. Requirements (EARS)

- **R1 (H)** WHEN `specd verify <slug> <id>` runs for a `builder` task, THE
  SYSTEM SHALL capture the set of files changed since the task started (via
  `git` against the recorded base) into the `VerificationRecord`.
- **R2 (H)** WHEN a captured changed file lies outside the task's declared
  `files:` contract (glob-matched), THE SYSTEM SHALL emit a **scope** violation
  via a new gate, configurable `warn`/`error` (default `warn`).
- **R3 (M)** WHERE the verify command emits coverage in a recognized format
  (e.g. Go `-coverprofile` total), THE SYSTEM SHALL record the coverage figure
  into the record; otherwise it SHALL record "unavailable" without failing.
- **R4 (M)** THE SYSTEM SHALL NOT block completion on coverage value (no
  hard-coded floor in the binary); coverage is evidence, not a gate, in this
  spec.
- **R5 (M)** WHERE a task's `files:` is `*` or unset, the scope gate SHALL be a
  no-op for that task (no false positives).
- **R6 (L)** `specd report` SHALL display per-task changed-file count and
  coverage where captured.

## 4. Design / approach

1. **Extend the record** — add `ChangedFiles []string` and `Coverage *string`
   to `VerificationRecord` (`state.go`); JSON back-compat via `omitempty`.
2. **Capture** — in `RunVerify`, after the command runs, diff against the base
   HEAD (`git diff --name-only`) for the changed set; optionally parse a
   coverage total from output/`-coverprofile`.
3. **Scope gate** — `GateScope(CheckCtx)` matches each changed file against the
   owning task's `files:` globs (`path/filepath.Match`); out-of-scope ⇒
   warn/error per config. `*`/unset ⇒ skip.
4. **Surface** — extend report rendering.

## 5. Non-goals

- No coverage floor enforcement in the binary (that stays a CI script concern).
- No language-specific coverage tooling bundled — parse what the command emits.
- No change to the evidence gate's completion contract beyond the new record
  fields.

## 6. Acceptance criteria

- A builder that edits a file outside its `files:` contract produces a scope
  violation (warn by default, error when configured).
- The verify record contains the changed-file set; coverage is recorded when the
  command emits it, "unavailable" otherwise — never a failure.
- `files: *`/unset ⇒ no scope violations.
- Report shows changed-file count + coverage; `make ci` green; stdlib-only;
  `VerificationRecord` JSON stays backward compatible.
