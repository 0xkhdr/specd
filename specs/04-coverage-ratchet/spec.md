# Spec 04 — Coverage Ratchet Step

> Wave: **W2 (P1)** · Priority: **P1** · Source: LEVEL_UP_PLAN §1.1, §2 P1.6
> Depends on: **Specs 01–03** (the new tests land the coverage this ratchet locks in)

## 1. Problem

The coverage **floor** is a regression ratchet, not the goal, and it currently
sits ~2.5pts *under* measured:

| Scope | Floor (`coverage-check.sh`) | Measured | Stated target |
|---|---|---|---|
| overall | 70 | 72.5% | 85% |
| internal/core | 70 | 74.9% | 95% |
| internal/cmd | — (no gate) | 61.9% | — |
| internal/mcp | 70 | 86.6% | — |
| internal/testharness | 80 | 80.0% | — |

Two issues:
1. The floors lag measured, so coverage can silently *drop* before the gate
   trips.
2. The most dangerous package (`internal/cmd`, 61.9%) has **no gate at all**.

The plan's P0/P1 tests (Specs 01–03) raise measured coverage materially
(`cmd/brain.go` 0→≥80, new `internal/worker` ≥90). This spec **locks that gain
in** by ratcheting floors to measured-minus-1 and adding the missing `cmd` gate.

## 2. Solution

After Specs 01–03 land, re-measure and raise floors to **measured-minus-1**:

- `OVERALL_MIN` 70 → **78**
- `CORE_MIN` 70 → **80**
- add `CMD_MIN` → **75** (new gate for `internal/cmd`)
- keep `MCP_MIN` (89-ish), `HARNESS_MIN` (81) at measured-minus-1

`measured-minus-1` is deliberate: tight enough to catch regressions, loose
enough to absorb statement-count noise. **Never lower a floor to make a red
build pass** — this rule is already documented in `coverage-check.sh` header
and must be reinforced.

Also document the **long-term targets** (85 overall / 90→95 core) explicitly in
the script header and in `TESTING.md`, and the per-PR **no-floor-lowering** rule.

## 3. Acceptance criteria

- [ ] `scripts/coverage-check.sh` floors raised to measured-minus-1 post-Spec-03
      (overall ≥78, core ≥80, **cmd gate added ≥75**).
- [ ] A `CMD_MIN` env + check added for `internal/cmd` (previously ungated).
- [ ] Script header documents the 85/95 targets and the no-lowering rule.
- [ ] `TESTING.md` updated to reflect the new floors + targets.
- [ ] CI green at the raised floors (the floors must reflect *actual* post-test
      measured coverage — do not invent numbers; measure first).

## 4. Method (must measure, not guess)

1. Land Specs 01–03.
2. Run the same coverage command CI uses (`make cover` / `coverage-check.sh`
   path) and record per-package measured coverage.
3. Set each floor to `floor(measured) - 1`.
4. Re-run; confirm green with margin.

## 5. Non-goals

- Hitting the *final* 85/95 target in this spec — that is the cumulative result
  of the whole program. This spec ratchets one step and documents the target.
- Adding new product tests beyond what Specs 01–03 already deliver (this spec is
  the gate, not the test-writing).

## 6. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Floor set above noisy measured → flaky red | Use measured-minus-1, re-run `-count=2` before committing the floor |
| `cmd` gate too aggressive day one | Set `CMD_MIN` to actual measured-minus-1 after Spec 01/02/03 land, not an aspiration |
