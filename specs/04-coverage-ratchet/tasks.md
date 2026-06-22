# Spec 04 — Tasks (Coverage Ratchet)

> Prereq: **Specs 01–03 done & merged-to-branch** — measure *after* their tests
> land, never before.

---

## Wave A — Measure

### [ ] W2.6a — Capture post-Spec-03 measured coverage
- **Files:** n/a (record numbers in the PR description / this file)
- **Do:** Run the CI coverage path (`./scripts/coverage-check.sh` and/or
  `go test -coverprofile` per package). Record measured: overall, internal/core,
  internal/cmd, internal/worker, internal/mcp, internal/testharness.
- **Done when:** Real measured numbers recorded; `internal/cmd` and
  `internal/worker` shown ≥ their Spec-01 targets (80 / 90).

---

## Wave B — Ratchet

### [ ] W2.6b — Raise floors + add cmd gate
- **Files:** `scripts/coverage-check.sh`
- **Do:** Set `OVERALL_MIN` and `CORE_MIN` defaults to `floor(measured)-1`
  (expected ~78 / ~80). Add a `CMD_MIN` (default `floor(cmd_measured)-1`,
  expected ~75) and a check block for `internal/cmd` mirroring the existing
  per-package checks. Add a `WORKER_MIN` (≥90) if a per-package gate fits the
  script's shape. Do **not** lower any existing floor.
- **Done when:** Script enforces the new floors; `./scripts/coverage-check.sh`
  passes locally with margin under `-count=2`.

### [ ] W2.7 — Document targets + no-lowering rule
- **Files:** `scripts/coverage-check.sh` (header), `TESTING.md`
- **Do:** Update the script header note: state long-term targets (85 overall /
  90→95 core) and the explicit **no-floor-lowering** rule (floors only ratchet
  up). Update `TESTING.md`'s coverage section to match the new floors + targets.
- **Done when:** Header + `TESTING.md` consistent with the new gate.

---

## Wave C — Verify in CI

### [ ] W2.7b — Confirm CI green at raised floors
- **Files:** n/a (CI)
- **Do:** Push; confirm the coverage job passes at the new floors across the
  OS matrix. If any package is below its new floor, the fix is **more tests**,
  not a lower floor.
- **Done when:** CI coverage gate green on `level-up`; update
  `specs/progress.md` W2 + exit gate.

---

## Definition of done (Spec 04)
- [ ] Floors raised to measured-minus-1; `internal/cmd` now gated (≥75).
- [ ] Targets + no-lowering rule documented in script + `TESTING.md`.
- [ ] CI coverage gate green across the matrix.
