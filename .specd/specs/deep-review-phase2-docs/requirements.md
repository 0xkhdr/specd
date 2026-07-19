# Requirements — deep-review-phase2-docs

> Source: DEEP-REVIEW.md §2 finding #3, §3.1 docs drift, §3.2, §4 Phase 2.

## R1 — Single-source the command reference

- owner: 0xkhdr
- priority: must
- risk: low

- R1.1: When the docs generator runs (`go run ./tools/gendocs`), the system shall render `docs/command-reference.md` deterministically from the `specd help --json` palette.
- R1.2: When the generator output differs from the committed `docs/command-reference.md`, the system shall fail the docs lint with a non-zero exit.
- R1.3: When the palette becomes the single source of truth, the system shall no longer carry `docs/CHEATSHEET.md` as a hand-copied byte-identical duplicate.
- edge: If `specd help --json` emits an unstable ordering, the generator shall sort verbs and flags so output stays byte-stable across runs.

## R2 — CI gate list matches contributor docs

- owner: 0xkhdr
- priority: should
- risk: low

- R2.1: When a contributor reads `CLAUDE.md` or `CONTRIBUTING.md`, the system shall list every CI gate actually run (staticcheck, govulncheck, shellcheck, coverage floor, perf-gate included) or point to `scripts/ci-local.sh` mirroring CI exactly.
- R2.2: When `scripts/ci-local.sh` runs green locally, the system shall have exercised the same lint and test gates the CI pipeline runs.

## Non-goals

- No change to CI job structure (that is deep-review-phase4-ci).
- No folding of `concepts.md`/`user-guide.md` in this phase.
