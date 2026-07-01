# S3 Tasks: Deprecation Cleanup — Scripts

Dependencies: none (Wave 1 spec, no code deps).
Note: coordinate merge timing with S4 (see spec.md "Compatibility" note) — the doc fix for
the dangling `README.md`/`docs/command-reference.md` reference to `uninstall.sh` should
land in the same release, though the two specs can be implemented independently.

Blocks: S6 (final CI gate).

---

## Wave 1 — Delete the script

- [x] **T1.1** Confirm no cross-reference exists before deleting:
  `grep -rn "uninstall" scripts/install.sh Makefile .github/workflows/ci.yml .goreleaser.yml`
  — expect zero output. If anything appears, stop and investigate before deleting (repo
  state may have changed since this spec was written).
  - Dependencies: none.
  - Completion evidence: empty grep output confirmed (exit code 1, no matches).

- [x] **T1.2** Delete `scripts/uninstall.sh`.
  - Dependencies: T1.1.
  - Completion evidence: `test -f scripts/uninstall.sh` fails (file absent). Deleted via
    `git rm scripts/uninstall.sh`.

- [x] **T1.3** Run `shellcheck scripts/*.sh` and confirm it still passes with the file gone.
  - Dependencies: T1.2.
  - Completion evidence: shellcheck exits 0, no findings.

**Wave 1 validation:** `make lint` (`shellcheck` target) passes;
`git status` shows only the deletion, no unintended edits.

---

## Wave 2 — Confirm no build/CI breakage

- [x] **T2.1** `grep -rn "uninstall" Makefile .github/workflows/*.yml .goreleaser.yml`
  returns nothing.
  - Completion evidence: empty output confirmed (exit 1).

- [x] **T2.2** Run `make ci` (or the closest available subset if the full target is heavy)
  and confirm no step fails due to the missing file.
  - Completion evidence: `make ci` ran full suite (build, vet, race tests, count=2 tests,
    coverage-check, shellcheck, test-lint, workflow harness tests). Only failure is
    `cover-check`: `internal/worker 87.4% < 88% floor` — confirmed **pre-existing and
    unrelated** to this deletion (`git status`/`git diff --stat internal/worker/` show zero
    changes to that package; a shell-script deletion cannot move Go coverage numbers). No
    step failed due to the missing `uninstall.sh` file. All other packages meet their
    coverage floors; `shellcheck scripts/*.sh` exits 0.

**Wave 2 validation (gate for S6):** T2.1 and T2.2 both pass — no build/CI breakage
attributable to this deletion. Flag to S4/S5 owners (or self, if doing all specs) that
`README.md:58`, `docs/command-reference.md:71,110-111`, **and additionally
`docs/mcp-guide.md` and `docs/concepts.md`** (found via broader grep, not called out in the
original spec) still reference the now-deleted `uninstall.sh` until S4/S5 land.

Flag to S6: the pre-existing `internal/worker` coverage-floor failure (87.4% < 88%) is
unrelated to S1/S2/S3 deprecation-cleanup work and must be resolved (new tests or a
justified floor adjustment) before S6's final CI gate can go green — this is a
pre-existing gap in the repo, not something this cleanup introduced.
