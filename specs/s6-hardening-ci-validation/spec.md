# S6: Hardening — CI & Static Analysis

## Purpose and Requirement Coverage

Final validation gate for the whole v0.1.0 hardening effort: confirm `make ci` passes with
S1-S5 landed, confirm zero runtime dependencies, confirm the `SHA256SUMS` release contract
still holds with `update.go` gone, fix the now-provably-broken `scripts/docs-lint.sh`, and
add a `CHANGELOG.md` entry documenting the breaking changes from S1-S3 (per the action
prompt's instruction that "migration" here means documenting breaking changes in release
notes).

Covers: **R9** (`make ci` passes), **R10** (zero deps preserved), **R11** (exit-code
contract preserved), **R12** (`SHA256SUMS` filename preserved), **R16** (no version-gated
old-command handling remains).

Depends on: S1, S2, S3, S4, S5 (this is the final gate; it validates their combined
output and does not introduce further command/doc changes of its own, except the
`docs-lint.sh` fix and the `CHANGELOG.md` entry, both of which are net-new hardening
findings surfaced during this investigation).

## Verified Current State

### `govulncheck` / `staticcheck` — already wired, contrary to a literal reading of "add"

- **Correction vs. analysis plan's constraint wording** ("add `govulncheck` to CI (already
  present)"): verified — `govulncheck` and a `golangci-lint`-based static-analysis step
  (which subsumes `staticcheck`-class checks) are **already present** in
  `.github/workflows/ci.yml`'s lint job. No Makefile target wraps them (they only run in
  GitHub Actions, not `make lint`), which is a reasonable, common split (heavy/slow
  linters in CI only) and is **not** treated as a defect here — no action required beyond
  confirming they still pass after S1-S5's deletions.

### `scripts/docs-lint.sh` — genuinely broken and unwired, a new finding

- The script hard-fails (`exit 1`, `"missing $csv"`) if
  `.specd/specs/cmd-audit/audit.csv` does not exist. Verified: **this file does not exist
  anywhere in the repository** (`find . -iname audit.csv` → no results).
- The script is **not referenced anywhere** in `Makefile` or `.github/workflows/*.yml` —
  it is currently dead, unreachable tooling. Running it standalone today
  (`bash scripts/docs-lint.sh`) fails immediately with `missing
  .specd/specs/cmd-audit/audit.csv`.
- Its logic has two independent parts:
  1. A bash dead-command scanner: reads `audit.csv`'s rows where column 10 is `"merge"` or
     `"deprecate"`, then greps `README.md`/`AGENTS.md`/`docs/*.md` for bare `specd <cmd>`
     mentions of those commands — whitelisting the migration-appendix block in
     `docs/command-reference.md` while scanning it (lines 27-31).
  2. A Python cheat-sheet consistency check: asserts `docs/command-reference.md`'s cheat
     sheet table lists exactly a hardcoded 20-command `survivors` list, and that
     `.specd/specs/CHEATSHEET.md` also lists exactly 20 commands. **Verified: neither
     `.specd/specs/CHEATSHEET.md` nor `.specd/specs/cmd-audit/` exists in the repo today**
     — this second check would also fail if it ever ran, for the same missing-file reason.
- Once S5 removes the migration-appendix delimiter comments from
  `docs/command-reference.md`, the `in_appendix` whitelist branch (part 1 above) becomes
  provably dead code (there is nothing left to whitelist).
- **This is a pre-existing gap, not something S1-S5 introduce** — the script was already
  broken and already unwired before this cleanup started. It surfaces here because
  fixing/removing genuinely dead or broken tooling falls under this release's explicit
  hardening mandate ("ensure ... no dead code").

### `CHANGELOG.md` — has an `[Unreleased]` section already, needs this cleanup's entries

- `CHANGELOG.md` already documents `[0.1.0] - 2026-06-14` as a released version and has an
  `[Unreleased]` section documenting the `boot`/`enrich` removal as a prior breaking
  change. **This cleanup's changes (S1-S3) belong in that same `[Unreleased]` section** as
  additional `### Removed (breaking)` bullets — do not create a new version header; append
  to the existing `[Unreleased]` block, consistent with the file's existing style.

### `go.mod`, `.goreleaser.yml`, `scripts/install.sh` — verified unaffected

- `go.mod` has zero `require` entries today and none of S1-S5's changes touch it. Re-verify
  after S1-S5 land (pure regression check, no expected change).
- `.goreleaser.yml`'s `checksum.name_template: SHA256SUMS` (lines 34-37) and
  `scripts/install.sh`'s checksum verification (lines 48-64) are both **independent of**
  `update.go` — they were already a self-contained two-party contract at the file level
  (the comment at `.goreleaser.yml:35` mentions `update.go` by name in a comment, which
  becomes stale once `update.go` is deleted in S1 — this comment needs a one-line edit,
  a new finding not in the original plan).

## Proposed Design and End-to-End Flow

1. After S1-S5 land, run the full validation suite (`make ci`, `GOOS=windows go build`,
   `govulncheck ./...`, the CI lint job's static analysis) and fix any regression found —
   none are expected, since S1-S5 already include their own validation waves, but this is
   the integrated final check.
2. Fix `scripts/docs-lint.sh`:
   - Remove the now-dead `in_appendix` whitelist branch (lines ~27-31) once S5's appendix
     removal lands.
   - Decide and implement one of: (a) create a minimal `.specd/specs/cmd-audit/audit.csv`
     reflecting the current (post-cleanup) command list, all non-deprecated, so the script
     can run and be wired into `make lint`; or (b) simplify the script to drop the
     CSV-driven dead-command scan entirely (since there is no more "grace period"
     deprecation model in the codebase after this cleanup — see Open Decisions) and keep
     only the cheat-sheet-consistency check, creating a minimal `.specd/specs/CHEATSHEET.md`
     if that check is kept.
   - Wire the (fixed) script into `make lint` so it's no longer orphaned tooling.
3. Edit `.goreleaser.yml:35`'s comment to drop the stale `update.go` reference (name only
   `install.sh` as the other consumer of the `SHA256SUMS` filename).
4. Add `CHANGELOG.md` entries under the existing `[Unreleased]` → `### Removed (breaking)`
   heading (or a new one if the existing heading is judged too specific to boot/enrich)
   documenting: the 13 removed legacy commands/aliases (with explicit note that `doctor`'s
   diagnostics have no replacement), the removed `specd migrate config` / `init --migrate`
   path, the removed `scripts/uninstall.sh`, and the removed `specd update` self-update
   command.
5. Run the full acceptance-criteria command list below and record results in
   `specs/progress.md`.

## Interfaces, Contracts, Data, Configuration, Dependencies

- No production code changes in this spec beyond the `docs-lint.sh` fix (a
  developer-tooling script, not shipped in the binary) and a `.goreleaser.yml` comment.
- `CHANGELOG.md` is documentation, not code — no runtime impact.

## Invariants, Security, Errors, Observability, Compatibility, Rollback

- **INV1-INV7** (see analysis plan's Repository Context) must all still hold — this spec's
  validation commands directly check INV1 (`go.mod`), INV6 (`TestRegistryMatchesHelp`), and
  INV7 (`SHA256SUMS`); INV2-INV5 are indirectly covered by `go test ./... -race -count=1`
  passing with no new failures.
- **Security**: no new surface introduced. Removing dead/broken tooling (`docs-lint.sh`'s
  current unreachable state) is a hygiene improvement, not a security fix per se.
- **Compatibility**: this spec's `CHANGELOG.md` entries are the authoritative record of
  every breaking change from S1-S3 — this is the artifact users/downstream consumers will
  actually read before upgrading.
- **Rollback**: `git revert`. The `docs-lint.sh` fix and `.goreleaser.yml` comment edit are
  low-risk, easily reverted changes with no data implications.

## Acceptance Criteria and Validation Commands

- `make ci` passes on Linux (and macOS, if available in the execution environment).
- `GOOS=windows go build ./...` succeeds.
- `go.mod` has zero `require` entries: `grep -c "^require" go.mod` returns `0` (or the
  `require` keyword appears nowhere at all, depending on `go.mod`'s exact current
  formatting — verify by reading the file, not assuming a specific grep shape).
- `go mod graph | grep -v '^[^ ]* std' | wc -l` — informational; confirm no external module
  edges appear (the exact filter pattern should be re-verified against `go mod graph`'s
  real output shape at execution time, since `go mod graph`'s format can surprise a
  first-time reader).
- `grep -rn 'legacyAlias\|DeprecatedIn\|doctor\|dispatch\|program\|validate\|schema\|replay\|diff\|serve\|watch\|mode\|migrate\|update\|uninstall' internal/ scripts/ docs/ README.md AGENTS.md TESTING.md SECURITY.md` — every remaining hit must be manually reviewed and justified (e.g. `dispatch.go`'s file itself surviving, `validate` as an English word in prose, `watch` in an unrelated context) — this is **not** expected to return zero, because 8 of the 10 functional handler files (dispatch.go, program.go, validate.go, schema.go, replay.go, diff.go, serve.go, watch.go) legitimately still exist and are named after their surviving flag (`--dispatch`, `--program`, etc.). The action prompt's own verification command (§ "Verification of 100% Requirement Coverage", item 1) as literally written will show these expected hits — do not mistake them for failures.
- `grep -rn 'v1\.0\.0\|v0\.2\.0\|v0\.3\.0' docs/ README.md AGENTS.md TESTING.md SECURITY.md` returns nothing.
- `grep -rn '\-\-version' docs/ README.md AGENTS.md TESTING.md SECURITY.md` — all hits show `0.1.0`.
- `bash scripts/docs-lint.sh` exits 0 after this spec's fix.
- `scripts/coverage-check.sh` still passes (coverage floors unaffected or improved by
  deletions — deletions reduce the denominator of untested lines too, so floors are not
  expected to regress, but this must be run, not assumed).
- `CHANGELOG.md`'s `[Unreleased]` section contains explicit bullets for: legacy command
  aliases removed (naming all 13), `doctor`'s diagnostic-capability loss, config migration
  tool removed, `uninstall.sh` removed, `update` self-update removed.

## Open Decisions and Deviations

- **New finding, not in analysis plan**: `scripts/docs-lint.sh` is currently broken
  (missing data file) and unwired from CI/Makefile. This spec surfaces two remediation
  paths (create `audit.csv`/`CHEATSHEET.md` and wire the script in, vs. simplify the script
  to drop the now-obsolete CSV-driven deprecation scan and keep only the cheat-sheet
  check) and recommends the latter as lower-risk, since the CSV-driven "grace period"
  deprecation model no longer exists in the codebase after S1 (there is no more concept of
  a command that is deprecated-but-still-functional to track). **This is presented as an
  open decision for the executor/user to confirm, not a unilateral choice** — flagged per
  the action prompt's "never hide uncertainty" rule.
- **New finding, not in analysis plan**: `.goreleaser.yml:35`'s comment names `update.go`
  by name; this becomes stale once S1 deletes that file and should be corrected to a
  one-consumer (`install.sh`) description, mirroring the `TESTING.md` fix in S4.
- **Deviation from analysis plan §8 Phase 4**: the plan's validation command for "no
  deprecated surface remains" (§ "Verification of 100% Requirement Coverage", item 1) will,
  taken completely literally, still show matches for `dispatch`, `program`, `validate`,
  `schema`, `replay`, `diff`, `serve`, `watch` — because those words are legitimate
  substrings of surviving file names and flag names (`--dispatch`, `dispatch.go`, etc.).
  This spec clarifies that the correct interpretation of "no deprecated surface remains" is
  the absence of `legacyAliases`/`DeprecatedIn`/bare-alias-invocation, not the absence of
  these words as English/identifier substrings — re-stated explicitly here so this isn't
  mistaken for an incomplete cleanup during final review.

## Version Alignment Checklist

- Final cross-repo check (superset of S4/S5's individual checks):
  `grep -rn 'v0\.2\.0\|v0\.3\.0\|v1\.0\.0\|pre-1\.0\|pre-release' internal/ docs/ README.md AGENTS.md TESTING.md SECURITY.md CHANGELOG.md` —
  expected to show only `CHANGELOG.md`'s own historical section headers (`## [0.1.0] -
  2026-06-14` is fine; that's the version number, not a stale forward-reference) and
  nothing else.
- All `--version` install examples across `README.md`, `docs/user-guide.md`, and
  `scripts/install.sh`'s usage comment show `0.1.0`.
