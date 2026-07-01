# S5: Documentation Alignment — Guide Docs

## Purpose and Requirement Coverage

Update `docs/*.md` so no guide references deprecated commands, the removed migration
tool, or stale version language. Verified finding: **only `docs/command-reference.md`**
needs content edits; the other 13 files in `docs/` are already clean and require an
audit-confirmation pass, not rewrites.

Covers: **R5** (scrub deprecated references from guides), **R14** (version references),
**R15** (install examples).

Depends on: S1, S2, S3 (this spec describes the post-removal command surface).

## Verified Current State

### `docs/command-reference.md` (the only file needing edits)

- **Line 3**: *"...deprecated commands appear only in the migration appendix."* — becomes
  false once the appendix is removed; must be reworded.
- **Line 9** (cheat sheet row for `specd init`): *"Scaffold `.specd/`, managed agent
  integration, repair, migration, packs, and orchestration defaults."* — the word
  "migration" refers to the `--migrate` flag removed in S2 and must be dropped from this
  description.
- **Line 62-71**, "## Merged behavior homes": mostly accurate and should be **kept** (it
  documents where functionality now lives, which remains true post-cleanup — e.g. "Dispatch
  packets live under `specd next --dispatch`"), **except**:
  - Line 65: *"Legacy config conversion lives under `specd init --migrate`."* — false after
    S2; remove this bullet entirely (there is no surviving home for config migration — it's
    gone, not moved).
  - Line 71: *"Binary lifecycle operations use `scripts/install.sh` and
    `scripts/uninstall.sh`."* — update to remove `scripts/uninstall.sh` (deleted in S3); the
    binary lifecycle bullet should describe manual removal or just `install.sh` for
    install/reinstall.
- **Line 84**: *"...legacy JSON is still read and can be upgraded with the optimized init
  migration flag."* — the "optimized init migration flag" no longer exists (S2); reword to
  state that legacy JSON is still read but no longer has a built-in conversion path (manual
  conversion only), consistent with S2's spec.
- **Lines 94-113**, "## Migration appendix" (delimited by
  `<!-- docs-lint: migration-appendix begin/end -->` comments): the full 13-row deprecation
  table. **Delete the entire section**, including both delimiter comments. Per F7's
  recommendation, replace it with a short note, e.g.: *"specd v0.1.0 has no deprecated
  commands or aliases — the command surface above is complete."* (This satisfies the
  analysis plan's F7 instruction to "add a note that v0.1.0+ has no deprecated commands.")
  - **Cross-dependency**: `scripts/docs-lint.sh` has logic keyed to these exact delimiter
    comments (`in_appendix` tracking, lines 27-31 of that script) to whitelist the appendix
    from its dead-command scan. Once the appendix and its delimiters are gone, that
    whitelist branch in `docs-lint.sh` becomes dead code. **This script fix is S6's
    responsibility** (S6 owns CI/lint tooling), not S5's — but S5 must land first since
    S6's fix is meaningless while the appendix (and its markers) still exist.
- Verified: the cheat-sheet validation logic in `docs-lint.sh` (the Python block matching
  `` ^\| `specd ([a-z-]+)` \| `` ) does **not** match migration-appendix rows (they're
  formatted as `` | `doctor` | `init --repair` | ... `` — no `specd` prefix in the cell), so
  removing the appendix does not change the cheat-sheet-vs-`survivors` validation outcome.

### All other `docs/*.md` files — audited, no changes required

Verified via direct grep for every deprecated command name as a `specd <name>` invocation,
plus every old version string (`v0.2.0`, `v0.3.0`, `v1.0.0`, `pre-1.0`, `pre-release`)
across the entire `docs/` directory:

- `docs/user-guide.md` — clean; install example (line 34) already uses `--version 0.1.0`.
- `docs/agent-integration.md` — clean.
- `docs/contributor-guide.md` — clean.
- `docs/validation-gates.md` — clean. **Correction vs. analysis plan F8**: this file does
  **not** reference `doctor` (F8 is stale; verified via direct grep, zero matches). No
  action needed.
- `docs/troubleshooting.md` — clean; already references `specd init --repair`, not
  `specd doctor` — it was already updated ahead of this cleanup. (This means `README.md`'s
  stale link text pointing here, fixed in S4/T1.2, was the only lagging piece.)
- `docs/mcp-guide.md`, `docs/github-action.md`, `docs/open-spec-format.md`,
  `docs/spec-packs.md`, `docs/custom-gates.md`, `docs/concepts.md` — clean.
- `docs/agent-harness-baselines.md`, `docs/agent-harness-compat.md`,
  `docs/agent-harness-gap-analysis.md`, `docs/dashboard.md` — **not mentioned in the
  original analysis plan at all** (new files not accounted for in the plan's file
  inventory). Verified clean of deprecated command references and old version strings.
  Included here for completeness/audit-trail, not because they need edits.

## Proposed Design and End-to-End Flow

1. Edit `docs/command-reference.md`:
   - Line 3: reword to drop the "migration appendix" forward-reference.
   - Line 9: drop "migration" from the `init` cheat-sheet description.
   - Line 65: delete the "Legacy config conversion" bullet.
   - Line 71: drop `scripts/uninstall.sh` from the binary-lifecycle bullet.
   - Line 84: reword to drop the "optimized init migration flag" mention.
   - Lines 94-113: delete the entire "Migration appendix" section and its delimiter
     comments; replace with a one-line note that v0.1.0 has no deprecated commands.
2. Re-verify the other 13 `docs/*.md` files with a final grep pass (no edits expected, but
   re-run after S1-S3 land in case anything changed in the interim).

## Interfaces, Contracts, Data, Configuration, Dependencies

- Pure documentation edits, one file's content changes plus a re-verification pass.
- **Dependency on S6**: removing the appendix delimiter comments here means
  `scripts/docs-lint.sh`'s `in_appendix` branch becomes dead code — S6 must remove that
  branch in the same release (tracked in `specs/progress.md`).

## Invariants, Security, Errors, Observability, Compatibility, Rollback

- No code/security/compatibility impact — documentation only.
- **Rollback**: `git revert`.

## Acceptance Criteria and Validation Commands

- `grep -n "migration appendix" docs/command-reference.md` returns nothing.
- `grep -n "docs-lint: migration-appendix" docs/command-reference.md` returns nothing.
- `grep -n "init --migrate\|migrate\b" docs/command-reference.md` returns nothing (except
  possibly inside the new one-line "no deprecated commands" replacement note, if it happens
  to use the word — review manually).
- `grep -n "uninstall.sh" docs/command-reference.md` returns nothing.
- `grep -rn 'v0\.2\.0\|v0\.3\.0\|v1\.0\.0\|pre-1\.0\|pre-release' docs/*.md` returns nothing.
- `grep -rln "specd doctor\|specd dispatch\|specd program\|specd validate\|specd schema\|specd replay\|specd diff\|specd serve\|specd watch\|specd mode\b\|specd migrate\|specd update\b\|specd uninstall" docs/*.md` returns nothing.
- `bash scripts/docs-lint.sh` — expected to still fail at this point (missing
  `.specd/specs/cmd-audit/audit.csv`, a pre-existing, unrelated issue tracked in S6) but
  should **not** fail due to anything related to the appendix removal — if run after S6's
  fix, it should pass cleanly.

## Open Decisions and Deviations

- **Correction vs. analysis plan F8**: `docs/validation-gates.md` does not reference
  `doctor`; no action taken there (see S4 for the same correction noted against README's
  link, which is the actual place needing an edit).
- **New finding, not in the analysis plan's file inventory**: `docs/agent-harness-baselines.md`,
  `docs/agent-harness-compat.md`, `docs/agent-harness-gap-analysis.md`, and
  `docs/dashboard.md` exist in the repo but weren't listed in the plan's §6/§8 file lists.
  Audited and found clean — no action needed, but noted here so their absence from the
  original plan isn't mistaken for an oversight.
- **Cross-spec dependency**: the `docs-lint.sh` fix this spec's appendix removal requires is
  deliberately deferred to S6, since S6 owns CI/lint tooling changes. Do not consider S5
  "done" for release purposes until S6's corresponding task also lands (tracked in
  `specs/progress.md`).

## Version Alignment Checklist

- `docs/command-reference.md`: no v0.2.0/v0.3.0/v1.0.0 strings remain after the appendix
  removal (they only existed inside the deleted table).
- All other `docs/*.md` files: verified clean of old version strings (see audit list above).
- `docs/user-guide.md`'s install example already correctly pinned to `--version 0.1.0` —
  re-verify with `grep -n "\-\-version" docs/user-guide.md` after this spec completes.
