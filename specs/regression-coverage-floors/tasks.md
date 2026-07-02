# S12 Tasks — Coverage Floor Regression

Requirement coverage: R14. Dependencies: S1–S10.

## Wave 1 — Baseline (after test-adding specs green)

- [ ] Run `make cover-check`; record measured % vs. floor per package. File:
  `scripts/coverage-check.sh`.
- **Validation:** `make cover-check`

## Wave 2 — Ratchet (depends on Wave 1)

- [ ] For each package with ≥1% headroom over its floor, raise the floor one step
  toward the `TESTING.md` target. File: `scripts/coverage-check.sh`.
- [ ] Leave thin-headroom packages (core/cmd/worker/mcp/harness) unchanged to
  avoid noise flaps.
- **Validation:** `make cover-check`

## Wave 3 — Guard (depends on Wave 2)

- [ ] Confirm CI `coverage-floor:` job passes with the raised floors.
- **Validation:** `make cover-check` (locally) → CI green

## Rollout & cleanup

- [ ] Delete stray `coverage-*.out` artifacts from the repo root if regenerated.
- **Rollback:** revert floor edits (single lines).
- **Completion evidence:** green `make cover-check` with any raised floors noted
  in PR.
