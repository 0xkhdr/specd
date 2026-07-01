# S3: Deprecation Cleanup — Scripts

## Purpose and Requirement Coverage

Remove `scripts/uninstall.sh`, the script implementing the deprecated `specd uninstall`
replacement path, and confirm no other script or CI target depends on it.

Covers: **R3** (remove `uninstall.sh`).

## Verified Current State

- `scripts/uninstall.sh` exists (105 lines, POSIX shell). It removes
  `${HOME}/.local/share/specd` (or `~/.specd-repo` on non-Linux), the
  `~/.local/bin/specd` symlink, and `# specd` PATH lines from `.bashrc`/`.zshrc`/`.profile`
  (backing them up to `<rc>.specd.bak`).
- `scripts/install.sh` has **zero references** to `uninstall.sh`
  (`grep -n "uninstall" scripts/install.sh` → no matches, confirmed by direct grep). This
  resolves the analysis plan's F10/U3 risk item — there is no cross-reference to break.
- `Makefile` and `.github/workflows/ci.yml` have **zero references** to `uninstall.sh` by
  name. The only place it's touched is `make shellcheck`'s glob
  (`shellcheck scripts/*.sh`, `Makefile:56`) — this glob will simply match one fewer file
  once the script is deleted; no `Makefile` edit is required.
- `.goreleaser.yml` does not package or reference `uninstall.sh`.
- Documentation references to `uninstall.sh` exist in `README.md:58` (an "Uninstall"
  section with a `curl | bash` one-liner), `docs/command-reference.md:71` ("Binary
  lifecycle operations use `scripts/install.sh` and `scripts/uninstall.sh`") and
  `docs/command-reference.md:110-111` (migration appendix row). **These doc updates are
  S4/S5's responsibility, not S3's** — S3 only removes the script file itself; leaving a
  dangling doc reference to a deleted file is a defect that S4/S5 must close in the same
  release, tracked via `specs/progress.md`'s cross-spec dependency note.

## Proposed Design and End-to-End Flow

1. Delete `scripts/uninstall.sh`.
2. Re-run `shellcheck scripts/*.sh` to confirm the glob still succeeds with the file gone
   (this also incidentally re-lints every remaining script, which is a useful side effect,
   not an expansion of scope).
3. No `Makefile`/CI edits are needed (confirmed above — nothing references the file by
   name).

## Interfaces, Contracts, Data, Configuration, Dependencies

- None. This is a single, standalone file deletion with no code dependents (shell scripts,
  unlike Go, have no compiler to catch a dangling reference — the grep checks above are the
  verification substitute).

## Invariants, Security, Errors, Observability, Compatibility, Rollback

- **Security**: removing a script that mutates user shell rc files
  (`.bashrc`/`.zshrc`/`.profile`) and deletes installed binaries is a net hardening win —
  one fewer destructive, user-invoked script in the release artifact/repo.
- **Compatibility (breaking, intentional)**: users following the current `README.md`
  "Uninstall" instructions will get a 404 from the raw GitHub URL once this script is
  deleted and the doc line isn't yet updated — this is why S4 (which rewrites `README.md`'s
  Uninstall section) must land in the same release as S3, not be deferred. Document the
  package-manager/manual-removal alternative in the replacement doc text (S4 scope).
- **Rollback**: `git revert` restores the file; no data loss risk (it's a client-side
  removal script, not something that touches repository state).

## Acceptance Criteria and Validation Commands

- `scripts/uninstall.sh` does not exist: `test -f scripts/uninstall.sh && echo FAIL || echo OK`.
- `shellcheck scripts/*.sh` passes (0 issues, or only pre-existing issues unrelated to this
  deletion).
- `grep -rn "uninstall" Makefile .github/workflows/*.yml .goreleaser.yml` returns nothing.
- Cross-check (informational, not a blocker for S3 itself, but must be true before the
  overall release ships): `grep -rln "uninstall.sh" README.md docs/*.md` should return
  nothing **after S4/S5 land** — if this spec (S3) is validated in isolation before S4/S5,
  expect these hits and do not treat them as an S3 failure.

## Open Decisions and Deviations

- None — this spec matches the analysis plan (R3/F10/U3) with high confidence; the only
  addition is the explicit cross-spec dependency note above (S3 alone leaves a dangling doc
  reference until S4/S5 land — call this out in `progress.md` rather than silently assuming
  it resolves itself).

## Version Alignment Checklist

- No version strings live in `scripts/uninstall.sh`. Not applicable to this spec directly;
  the replacement doc text in S4 must not introduce a stale version reference when it
  rewrites the "Uninstall" section.
