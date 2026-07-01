# S5 Tasks: Documentation Alignment — Guide Docs

Dependencies: S1, S2, S3 (describes post-removal state).
Blocks: S6 (the `docs-lint.sh` fix depends on this spec's appendix removal landing first).

---

## Wave 1 — `docs/command-reference.md` edits

- [ ] **T1.1** Reword line 3 (currently referencing "the migration appendix" as where
  deprecated commands appear) to remove the forward-reference, since the appendix is being
  deleted in this same wave.
  - Dependencies: coordinate with T1.5 (do in the same commit so the file is never in an
    inconsistent state).
  - Completion evidence: `grep -n "migration appendix" docs/command-reference.md` returns
    nothing.

- [ ] **T1.2** Edit the `specd init` cheat-sheet row (currently line 9) to drop "migration"
  from its description.
  - Dependencies: S2 (the `--migrate` flag must actually be gone).
  - Completion evidence: manual review — description no longer claims init does migration.

- [ ] **T1.3** In the "## Merged behavior homes" section, delete the "Legacy config
  conversion lives under `specd init --migrate`" bullet (currently line 65).
  - Dependencies: S2.
  - Completion evidence: `grep -n "init --migrate" docs/command-reference.md` returns
    nothing.

- [ ] **T1.4** In the same section, edit the binary-lifecycle bullet (currently line 71,
  *"Binary lifecycle operations use `scripts/install.sh` and `scripts/uninstall.sh`"*) to
  drop the `scripts/uninstall.sh` reference.
  - Dependencies: S3.
  - Completion evidence: `grep -n "uninstall.sh" docs/command-reference.md` returns
    nothing.

- [ ] **T1.5** Reword line 84 (*"...can be upgraded with the optimized init migration
  flag"*) to state legacy JSON config is still read but has no built-in conversion path.
  - Dependencies: S2.
  - Completion evidence: manual review — no reference to an "init migration flag" remains.

- [ ] **T1.6** Delete the entire "## Migration appendix" section (currently lines 94-113,
  including the `<!-- docs-lint: migration-appendix begin -->` / `<!-- docs-lint:
  migration-appendix end -->` delimiter comments) and replace it with a one-line note,
  e.g.: *"specd v0.1.0 has no deprecated commands or aliases — the command surface above is
  complete."*
  - Dependencies: S1 (all 13 commands must actually be gone for this note to be true).
  - Completion evidence: `grep -n "docs-lint: migration-appendix\|Migration appendix"
    docs/command-reference.md` returns nothing; the replacement note is present.

**Wave 1 validation:** `grep -n "specd doctor\|specd dispatch\|specd program\|specd validate\|specd schema\|specd replay\|specd diff\|specd serve\|specd watch\|specd mode\b\|specd migrate\|specd update\b\|specd uninstall\|v0\.2\.0\|v0\.3\.0\|v1\.0\.0" docs/command-reference.md` returns nothing.

---

## Wave 2 — Audit-confirmation pass (all other docs files)

For each file below, re-run the deprecated-command and old-version grep and confirm zero
hits. These were verified clean at spec-authoring time; this wave re-verifies at execution
time in case anything changed in the interim (e.g. a concurrent edit).

- [ ] **T2.1** `docs/user-guide.md` — confirm clean; confirm install example still uses
  `--version 0.1.0`.
- [ ] **T2.2** `docs/agent-integration.md` — confirm clean.
- [ ] **T2.3** `docs/contributor-guide.md` — confirm clean.
- [ ] **T2.4** `docs/validation-gates.md` — confirm clean (no `doctor` reference — this
  corrects the analysis plan's stale F8 finding).
- [ ] **T2.5** `docs/troubleshooting.md` — confirm clean (already references `init --repair`,
  not `doctor`).
- [ ] **T2.6** `docs/mcp-guide.md`, `docs/github-action.md`, `docs/open-spec-format.md`,
  `docs/spec-packs.md`, `docs/custom-gates.md`, `docs/concepts.md` — confirm clean (can be
  done as one combined grep across all six).
- [ ] **T2.7** `docs/agent-harness-baselines.md`, `docs/agent-harness-compat.md`,
  `docs/agent-harness-gap-analysis.md`, `docs/dashboard.md` — confirm clean (files not in
  the original analysis plan's inventory; included here for completeness).

Each of T2.1-T2.7: dependencies none; completion evidence is the grep command and its
(expected empty) output pasted into the commit message — no file edit expected unless a
grep surprises you, in which case stop and handle the new finding before proceeding.

**Wave 2 validation:** combined command:
```
grep -rn 'v0\.2\.0\|v0\.3\.0\|v1\.0\.0\|pre-1\.0\|pre-release' docs/*.md
grep -rln 'specd doctor\|specd dispatch\|specd program\|specd validate\|specd schema\|specd replay\|specd diff\|specd serve\|specd watch\|specd mode\b\|specd migrate\|specd update\b\|specd uninstall' docs/*.md
```
Both return nothing.

---

## Wave 3 — Handoff to S6

- [ ] **T3.1** Note in `specs/progress.md` that S5 is complete and that `scripts/docs-lint.sh`'s
  `in_appendix` whitelist logic (lines ~27-31) is now dead code, ready for S6 to remove.
  - Dependencies: T1.6.
  - Completion evidence: `specs/progress.md` updated with this handoff note.

**Wave 3 validation (gate for S6):** T3.1 done; Wave 1 and Wave 2 both green.
