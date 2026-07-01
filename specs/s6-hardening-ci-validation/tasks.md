# S6 Tasks: Hardening — CI & Static Analysis

Dependencies: S1, S2, S3, S4, S5 all landed (this is the final Wave-3-equivalent spec —
no other spec depends on it).
Blocks: nothing (terminal spec); gates the actual `v0.1.0` re-tag/release decision, which
is explicitly out of scope for spec authoring (stop condition).

---

## Wave 1 — Full regression run

- [ ] **T1.1** `go build ./...` and `go vet ./...`.
  - Dependencies: S1-S5 landed.
  - Completion evidence: both succeed with no output/errors.

- [ ] **T1.2** `go test ./... -race -count=1`.
  - Completion evidence: all tests pass.

- [ ] **T1.3** `make ci` (full local gate: lint, test, test-order, cover-check, perf-gate,
  stress variants).
  - Completion evidence: exits 0.

- [ ] **T1.4** `GOOS=windows go build ./...`.
  - Completion evidence: succeeds.

- [ ] **T1.5** Confirm `govulncheck`/static-analysis in `.github/workflows/ci.yml`'s lint
  job still pass (push a branch or open a draft PR if local `govulncheck` isn't installed;
  otherwise run `govulncheck ./...` directly if available locally).
  - Completion evidence: CI run green, or local `govulncheck ./...` clean.

**Wave 1 validation:** all of T1.1-T1.5 green. If anything fails, stop and diagnose —
S1-S5's own validation waves should have caught regressions already; a Wave 1 failure here
means something was missed upstream.

---

## Wave 2 — `go.mod` and release-artifact contract checks

- [ ] **T2.1** Confirm `go.mod` has zero `require` entries (read the file directly; do not
  rely solely on a grep pattern that might miss a differently-formatted require block).
  - Completion evidence: file contents pasted into commit message, showing no `require`.

- [ ] **T2.2** Confirm `.goreleaser.yml`'s `checksum.name_template` is still `SHA256SUMS`.
  - Completion evidence: `grep -n "name_template" .goreleaser.yml` shows `SHA256SUMS`.

- [ ] **T2.3** Edit `.goreleaser.yml`'s comment above the checksum block (currently
  referencing `update.go` by name, e.g. *"Must match the filename update.go::fetchChecksums
  and install.sh expect."*) to drop the `update.go` mention, since that file is deleted in
  S1 — reword to reference only `install.sh`.
  - Dependencies: S1.
  - Completion evidence: `grep -n "update.go" .goreleaser.yml` returns nothing.

- [ ] **T2.4** Confirm `scripts/install.sh` still downloads and verifies `SHA256SUMS`
  correctly (no code change expected — this task is a re-read/confirm, not an edit).
  - Completion evidence: manual confirmation, lines cited in commit message.

**Wave 2 validation:** T2.1-T2.4 all confirmed; `.goreleaser.yml` has no stale reference to
a deleted file.

---

## Wave 3 — Fix `scripts/docs-lint.sh`

- [ ] **T3.1** Decide remediation path (this is the open decision from spec.md — confirm
  with the user/team before implementing, since it's a judgment call, not a mechanical
  fix):
  - **Path A**: create `.specd/specs/cmd-audit/audit.csv` (reflecting the current, fully
    non-deprecated command list) and `.specd/specs/CHEATSHEET.md` (20 rows matching the
    `survivors` list in the script), then wire `scripts/docs-lint.sh` into `make lint`.
  - **Path B (recommended)**: simplify `scripts/docs-lint.sh` to drop the CSV-driven
    dead-command scan (obsolete now that there's no more "grace period" deprecation model
    in the codebase — commands are either present or fully deleted, no in-between), keep
    only the cheat-sheet-consistency check (creating a minimal `.specd/specs/CHEATSHEET.md`
    if that check is kept), then wire the simplified script into `make lint`.
  - Dependencies: S5 (the migration-appendix delimiter comments this script's `in_appendix`
    logic depends on must be gone first).
  - Completion evidence: decision recorded in `specs/progress.md` with rationale.

- [ ] **T3.2** Implement the chosen path from T3.1.
  - Completion evidence: `bash scripts/docs-lint.sh` exits 0.

- [ ] **T3.3** Wire the (fixed) `scripts/docs-lint.sh` into the `lint` target in `Makefile`
  (alongside `fmt-check`, `test-lint`, `shellcheck`).
  - Dependencies: T3.2.
  - Completion evidence: `make lint` runs `docs-lint.sh` and passes.

**Wave 3 validation:** `make lint` passes, including the newly-wired `docs-lint.sh` step.

---

## Wave 4 — `CHANGELOG.md` breaking-change documentation

- [ ] **T4.1** Add bullets under the existing `[Unreleased]` → `### Removed (breaking)`
  heading in `CHANGELOG.md` (append to the existing boot/enrich bullet, don't replace it),
  covering:
  - All 13 removed legacy command aliases (`doctor`, `dispatch`, `program`, `validate`,
    `schema`, `replay`, `diff`, `serve`, `watch`, `mode`, `migrate`, `update`, `uninstall`),
    naming each and its (former) survivor home.
  - Explicit note that `doctor`'s diagnostic capability (scaffold/MCP/host-registration
    health checks) has **no replacement** — this is a real capability loss, not a rename.
  - `specd migrate config` / `specd init --migrate` removed — legacy JSON config is still
    *read*, just no longer convertible via a built-in command.
  - `scripts/uninstall.sh` removed — point at the new manual-removal instructions in
    `README.md` (from S4).
  - `specd update` self-update removed — point at `scripts/install.sh --force` /
    package-manager reinstall as the replacement.
  - Dependencies: S1, S2, S3 (must accurately describe what actually landed).
  - Completion evidence: `CHANGELOG.md` diff reviewed for accuracy against the actual S1-S3
    diffs (every named command/file must actually be gone).

**Wave 4 validation:** `CHANGELOG.md`'s `[Unreleased]` section read start-to-finish makes
sense as a coherent breaking-change list a downstream user could act on.

---

## Wave 5 — Final cross-repo verification (Definition of Done gate)

- [ ] **T5.1** `grep -rn 'legacyAlias\|nextMinorVersion\|DeprecatedIn' internal/` returns
  nothing.
- [ ] **T5.2** `grep -rn 'v0\.2\.0\|v0\.3\.0\|v1\.0\.0\|pre-1\.0\|pre-release' docs/ README.md AGENTS.md TESTING.md SECURITY.md` returns nothing.
- [ ] **T5.3** `grep -rn '\-\-version' docs/ README.md AGENTS.md TESTING.md SECURITY.md` — all hits show `0.1.0`.
- [ ] **T5.4** `TestRegistryMatchesHelp`, `TestPaletteCeiling`, `TestFlagSingleOwner`,
  `TestLegacyAliasSunset` (confirm this last one no longer exists — its file was deleted in
  S1) — run `go test ./internal/... -run 'TestRegistryMatchesHelp|TestPaletteCeiling|TestFlagSingleOwner'`
  and separately confirm `grep -rn "TestLegacyAliasSunset" internal/` returns nothing.
- [ ] **T5.5** Update `specs/progress.md` with final status: all six specs' waves
  complete, all validation commands green, `docs-lint.sh` remediation path chosen and
  implemented, `CHANGELOG.md` updated.

**Wave 5 validation (Definition of Done):** T5.1-T5.5 all green. This is the last task in
the entire spec tree — per the action prompt's stop condition, do not tag `v0.1.0`, open a
PR, or push to `main` beyond this point without explicit user instruction to execute.
