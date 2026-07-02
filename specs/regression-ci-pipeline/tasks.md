# S13 Tasks — CI Pipeline Regression

Requirement coverage: R15. Dependencies: S1–S12.

## Wave 1 — Baseline (after all test specs green)

- [ ] Run `make ci` locally end-to-end; record pass/fail + wall-clock per target.
  Files: `Makefile`, `.github/workflows/ci.yml`.
- **Validation:** `make ci`

## Wave 2 — Parity & drift guard (depends on Wave 1)

- [ ] Cross-check `make ci` targets vs. `ci.yml` jobs; list any target run
  locally but absent from CI (e.g. `stress-acp`, `stress-orchestration`,
  `stress-program`). File: `.github/workflows/ci.yml`.
- [ ] Add missing stress jobs to CI *or* document why they are local-only.
- [ ] Add a `go mod tidy` verification step (guard toolchain/deps drift, F9).
- **Validation:** `make ci`

## Wave 3 — Green on PR (depends on Wave 2)

- [ ] Push branch; confirm every CI job passes.
- **Validation:** CI green on PR

## Rollout & cleanup

- [ ] Keep golangci-lint pinned (v2.1.6) to avoid lint drift.
- **Rollback:** revert workflow edits.
- **Completion evidence:** green `make ci` + green CI, parity documented.
