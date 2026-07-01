# S4: Documentation Alignment — Root Docs

## Purpose and Requirement Coverage

Update `README.md`, `AGENTS.md`, `SECURITY.md`, and `TESTING.md` so none of them reference
the commands/scripts removed by S1-S3, and so version/support-status language matches
v0.1.0.

Covers: **R5** (scrub deprecated references), **R6** (remove `AGENTS.md` RTK section),
**R7** (README v0.1.0 alignment), **R8** (SECURITY.md v0.1.0 supported version), **R14**
(version references), **R15** (install examples pinned to 0.1.0).

Depends on: S1 (commands gone), S2 (migrate gone), S3 (`uninstall.sh` gone) — this spec's
edits describe the *post*-removal state, so land it after (or in the same PR as) S1-S3.

## Verified Current State

### `README.md`

- Install examples (`README.md:44,50,53`) already use `--version 0.1.0` (line 53) — **no
  change needed**, R15 is already satisfied here.
- `README.md:56-59` — "Uninstall" section:
  ```
  ### Uninstall
  \`\`\`bash
  curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/uninstall.sh | bash
  \`\`\`
  ```
  This references the script deleted in S3 — **must be rewritten** to manual-removal
  instructions (no script exists to curl once S3 lands).
- `README.md:61-64` — "Update" section **already** uses `install.sh --force`, not
  `specd update` — **no change needed**. This confirms the doc authors already anticipated
  `update` command removal; nothing to fix here.
- `README.md:68` — Windows note already says "reinstall with the installer command above
  instead of relying on in-place binary replacement" — **no change needed**, already
  consistent with `update.go`'s removal.
- `README.md:143` — links to `docs/troubleshooting.md` with the text *"for `doctor`
  remediation"* — must be reworded since the `doctor` command no longer exists (the link
  target itself, `docs/troubleshooting.md`, is audited separately in S5).
- Lines 14, 18, 20, 21, 35 use words like "Validation", "Dispatch", "Diff", "Watch" as
  **prose/feature names**, not CLI command invocations (verified: none are backtick-quoted
  `specd <word>` command references) — **no change needed**, these are false positives for
  a naive grep and must not be "fixed."
- Directory/doc map section (`README.md:179-208`) does not name any deprecated command
  file — **no change needed**.

### `AGENTS.md`

- **RTK section, lines 252-293**, delimited by `<!-- headroom:rtk-instructions -->` /
  `<!-- /headroom:rtk-instructions -->` comments, titled "RTK (Rust Token Killer) -
  Token-Optimized Commands". This sits *after* the file's real closing marker
  `<!-- SPECD INIT: END v1 -->` (line 249) and instructs prefixing shell commands with
  `rtk` — it is unrelated to specd's actual product surface (confirmed: no reference to it
  anywhere in `internal/core/embed_templates/`). **Delete lines 252-293 entirely** (R6).
- File tree listing, `AGENTS.md:68`: `report.go waves.go program.go update.go` — remove
  `update.go` (file deleted in S1); **keep** `program.go` (survives — shared handler for
  `status --program`, per S1's verified findings).
- File tree listing, `AGENTS.md:90`: `scripts/ ... install.sh uninstall.sh coverage-check.sh
  stress.sh ...` — remove `uninstall.sh` (file deleted in S3).
- `AGENTS.md:66` lists `dispatch.go` — **keep**, this file survives (shared handler).
- No other deprecated-command references found in `AGENTS.md` outside the RTK section and
  the two file-tree lines above (verified via targeted grep excluding lines 252-293).

### `SECURITY.md`

- `SECURITY.md:16`: *"specd is pre-1.0; only the latest tagged release receives security
  fixes."* — replace "pre-1.0" framing with v0.1.0-current framing per R8/R14, e.g.
  *"specd is currently at v0.1.0; only the latest tagged release receives security fixes."*
- `SECURITY.md:42-45`: describes `specd doctor` reporting a missing `bwrap`/container
  dependency as an **advisory finding** before a user hits `verify`'s fail-closed error.
  Verified: this advisory (`inspectSandboxAvailability`, `doctor.go:310-338`) is called
  **only** from `runDoctor` (`doctor.go:79`) — there is no other call site. **Once `doctor`
  is removed (S1), this advisory mechanism is gone, not just renamed.** This paragraph must
  be rewritten to state plainly that the pre-flight advisory no longer exists and that
  `verify --sandbox`'s fail-closed behavior is now the only signal a user gets — this is a
  real, user-facing regression in the threat model, not a cosmetic wording fix, and must
  not be silently smoothed over.
- `SECURITY.md:50-52`: *"**Self-update integrity.** `install.sh` and `specd update` fetch a
  release archive and fail closed if the `SHA256SUMS` digest does not match
  (`--no-verify` skips with a loud warning)."* — remove the `specd update` clause; only
  `install.sh` performs this check after S1 deletes `update.go`. Rewrite to: *"**Install
  integrity.** `install.sh` fetches a release archive and fails closed if the `SHA256SUMS`
  digest does not match (`--no-verify` skips with a loud warning)."*
- No other deprecated-command references in `SECURITY.md`.

### `TESTING.md`

- `TESTING.md:217`: *"`SaveState` / `migrate` sit in the 90s..."* — **do NOT touch.** This
  `migrate` refers to `internal/core/state.go:251`'s `func migrate(raw
  map[string]json.RawMessage) (State, error)` — an unrelated **state-schema migration**
  function (upgrading old `state.json` shapes), not the config-JSON-to-YAML migration
  command being removed. This is a verified false positive for any naive
  grep-and-replace of the word "migrate" and must be explicitly preserved.
- `TESTING.md:225`: *"...lives in `COVERAGE_GAPS.md`."* — verified: **`COVERAGE_GAPS.md`
  does not exist anywhere in the repository.** This is a pre-existing broken reference,
  unrelated to command deprecation, but it is a documentation-integrity defect uncovered
  during this cleanup and should be fixed under the "harden the codebase" mandate. This
  spec does not require creating the file (out of scope — no evidence of what its content
  should be); instead, rewrite the sentence to stop pointing at a file that doesn't exist
  (e.g. state that the dark-path inventory is tracked ad hoc via `// TODO` /
  `// won't test:` comments in-source, if that is in fact how gaps are tracked today, or
  simply remove the specific-file claim and describe the *policy* without naming a
  nonexistent artifact). Flag this as a new finding, not in the original analysis plan.
- `TESTING.md:244-251`, "### Windows limitation (known, documented)": entirely about
  `specd update`'s self-replacement failing on Windows (`update.go`) — **this whole
  limitation disappears once `update.go` is deleted** (S1). The subsection must be
  rewritten, not merely edited, since its entire premise (a self-update binary that can't
  replace itself on Windows) no longer exists. Windows remains build-only in CI for a
  **different**, still-valid reason: Brain/Pinky orchestration is POSIX-shell-only
  (documented in `README.md:68`: *"orchestration requires a POSIX shell (sh); not
  supported on Windows — run under WSL"*). Rewrite this subsection to state that Windows is
  build-only in CI because orchestration and `verify`'s shell execution assume a POSIX
  shell, not because of any self-update limitation.
- `TESTING.md:259-264`, the `SHA256SUMS` three-consumer list: currently names
  `.goreleaser.yml`, `internal/cmd/update.go`, and `scripts/install.sh`. Once `update.go` is
  deleted (S1), this is a **two**-consumer contract, not three. Rewrite to remove the
  `internal/cmd/update.go` bullet and adjust the framing sentence ("must stay identical
  across three consumers" → "two consumers").
- No other deprecated-command references in `TESTING.md`.

## Proposed Design and End-to-End Flow

1. `README.md`: rewrite the "Uninstall" section (56-59) to manual-removal instructions;
   reword the `docs/troubleshooting.md` link text (143) to drop "`doctor`".
2. `AGENTS.md`: delete lines 252-293 (RTK section); edit the two file-tree lines (68, 90) to
   drop `update.go` and `uninstall.sh` respectively.
3. `SECURITY.md`: reword line 16 (pre-1.0 → v0.1.0); rewrite lines 42-45 (doctor advisory
   loss, stated plainly); rewrite lines 50-52 (drop `specd update`, keep `install.sh`).
4. `TESTING.md`: leave line 217 untouched; rewrite line 225's dangling `COVERAGE_GAPS.md`
   reference; rewrite the "Windows limitation" subsection (244-251); rewrite the
   `SHA256SUMS` consumer list (259-264) from three to two consumers.

## Interfaces, Contracts, Data, Configuration, Dependencies

- Pure documentation edits. No code, schema, or CI-config changes in this spec.
- Depends on S1/S2/S3 being landed (or landing together) so these docs describe the actual
  post-cleanup state rather than describing removals ahead of the code that performs them.

## Invariants, Security, Errors, Observability, Compatibility, Rollback

- **Security-relevant documentation accuracy**: `SECURITY.md`'s threat model must not claim
  a mitigation (the `doctor` sandbox-dependency advisory) that no longer exists — leaving
  stale security documentation is itself a finding-worthy defect. This spec treats that
  correction as mandatory, not optional polish.
- **Compatibility**: none of these are code changes; no behavioral compatibility risk.
- **Rollback**: `git revert` restores prior text; no data or code risk.

## Acceptance Criteria and Validation Commands

- `grep -n 'uninstall.sh' README.md` returns nothing (Uninstall section rewritten).
- `grep -n 'specd update\b' SECURITY.md TESTING.md README.md` returns nothing.
- `grep -n 'specd doctor\|`doctor`' README.md SECURITY.md` — any remaining hits must be
  reviewed manually to confirm they're historical/explanatory (e.g. "the removed `doctor`
  command") rather than instructing a user to run it.
- `grep -n 'pre-1.0' SECURITY.md` returns nothing.
- `grep -n 'RTK\|Rust Token Killer\|headroom:rtk' AGENTS.md` returns nothing.
- `grep -n 'update.go' AGENTS.md` returns nothing.
- `grep -n 'uninstall.sh' AGENTS.md` returns nothing.
- `grep -n 'COVERAGE_GAPS.md' TESTING.md` returns nothing (or, if kept, a `test -f
  COVERAGE_GAPS.md` check must pass — this spec's recommended path is removing the
  dangling reference, not creating the file).
- `grep -n 'internal/cmd/update.go' TESTING.md` returns nothing.
- Manual review: `README.md`'s new Uninstall section gives a correct, safe manual-removal
  procedure (matches what `scripts/uninstall.sh` used to do: remove the install directory,
  the symlink, and PATH lines — described in prose, not a script).
- `--version 0.1.0` still appears in `README.md`'s install examples (unchanged, already
  correct).

## Open Decisions and Deviations

- **New finding, not in analysis plan**: `SECURITY.md`'s doctor-advisory paragraph
  (42-45) requires a substantive rewrite acknowledging a real mitigation's removal, not a
  simple word-swap. Flagged explicitly so this isn't glossed over during execution.
- **New finding, not in analysis plan**: `TESTING.md`'s `COVERAGE_GAPS.md` reference points
  at a file that has never existed in this repo (confirmed via repo-wide `find`). This
  spec's recommendation is to remove the dangling claim rather than fabricate the file's
  content; if the user/executor prefers to create `COVERAGE_GAPS.md` instead, that is an
  equally valid resolution but is out of this spec's authored scope (no source data exists
  to populate it correctly).
- **New finding, not in analysis plan**: `TESTING.md`'s "Windows limitation" subsection and
  `SHA256SUMS` three-consumer list both require content rewrites (not just deletions) since
  their premises (an existing `update.go`) disappear in S1.
- **Correction vs. analysis plan F8**: `docs/validation-gates.md` does **not** reference
  `doctor` (verified via direct grep — zero matches). F8 is stale; no action needed there
  (see S5).

## Version Alignment Checklist

- `README.md`: `--version 0.1.0` already present and correct in all install examples — verified,
  no v0.2.0/v0.3.0/v1.0.0 strings found anywhere in the file.
- `AGENTS.md`: no version strings found outside the deleted RTK section.
- `SECURITY.md`: "pre-1.0" replaced with explicit v0.1.0 language; no v0.2.0/v0.3.0/v1.0.0
  strings found.
- `TESTING.md`: no v0.2.0/v0.3.0/v1.0.0 strings found; no `--version` install examples live
  here (not applicable).
- Final check across all four files: `grep -rn 'v0\.2\.0\|v0\.3\.0\|v1\.0\.0' README.md AGENTS.md SECURITY.md TESTING.md` returns nothing.
