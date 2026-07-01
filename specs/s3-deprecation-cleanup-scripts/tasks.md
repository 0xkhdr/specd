# S3 Tasks: Deprecation Cleanup — Scripts

Dependencies: none (Wave 1 spec, no code deps).
Note: coordinate merge timing with S4 (see spec.md "Compatibility" note) — the doc fix for
the dangling `README.md`/`docs/command-reference.md` reference to `uninstall.sh` should
land in the same release, though the two specs can be implemented independently.

Blocks: S6 (final CI gate).

---

## Wave 1 — Delete the script

- [ ] **T1.1** Confirm no cross-reference exists before deleting:
  `grep -rn "uninstall" scripts/install.sh Makefile .github/workflows/ci.yml .goreleaser.yml`
  — expect zero output. If anything appears, stop and investigate before deleting (repo
  state may have changed since this spec was written).
  - Dependencies: none.
  - Completion evidence: empty grep output confirmed.

- [ ] **T1.2** Delete `scripts/uninstall.sh`.
  - Dependencies: T1.1.
  - Completion evidence: `test -f scripts/uninstall.sh` fails (file absent).

- [ ] **T1.3** Run `shellcheck scripts/*.sh` and confirm it still passes with the file gone.
  - Dependencies: T1.2.
  - Completion evidence: shellcheck exits 0 (or with only pre-existing findings unrelated
    to this change — compare against a pre-deletion shellcheck run if any findings appear).

**Wave 1 validation:** `make lint` (or the `shellcheck` target specifically) passes;
`git status` shows only the deletion, no unintended edits.

---

## Wave 2 — Confirm no build/CI breakage

- [ ] **T2.1** `grep -rn "uninstall" Makefile .github/workflows/*.yml .goreleaser.yml`
  returns nothing.
  - Completion evidence: empty output.

- [ ] **T2.2** Run `make ci` (or the closest available subset if the full target is heavy)
  and confirm no step fails due to the missing file.
  - Completion evidence: `make ci` (or equivalent) green.

**Wave 2 validation (gate for S6):** T2.1 and T2.2 both pass. Flag to S4/S5 owners (or
self, if doing all specs) that `README.md:58` and `docs/command-reference.md:71,110-111`
still reference the now-deleted file until those specs land.
