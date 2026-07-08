# SPEC-05 Tasks: Test Coverage Formalization

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-05-01 | Measure per-package coverage | Produce `coverage.out`; record per-package percentages; identify gaps on critical/agent-facing surfaces. | A per-package coverage table exists; gap list produced. | Small | completed |
| T-05-02 | Set coverage floor policy | Decide the policy floor; update `coverage-check.sh` (from SPEC-01) to enforce it; ratchet above the provisional value. | Script fails on coverage below the policy floor (proven with a temporary stub); floor documented. | Medium | completed |
| T-05-03 | Close targeted gaps | Add tests for MCP contract (`internal/mcp/`), help-palette schema, and other flagged gaps, targeting SPEC-02's feature map. | Targeted packages meet the policy floor; new tests pass under `-race`. | Large | completed |
| T-05-04 | Wire regress harnesses | Add `regress-all.sh`/`regress-domains.sh`/`regress-lint.sh` to CI or document a required cadence + owner. | Harnesses run in CI, or a documented cadence with an owner exists. | Small | completed |
| T-05-05 | Fix order-dependence | Keep the `-count=2` leg; fix any flaky/order-dependent test it exposes. | `go test ./... -count=2` green and deterministic. | Medium | completed |
| T-05-06 | Author TESTING.md | Write an accurate testing guide (suite commands, coverage floor, regress harnesses, stress jobs) that `ci.yml:232` references. | `TESTING.md` exists, accurate; ci.yml:232 reference resolves. | Small | completed |

## Task Dependency Graph

```
T-05-01 ─→ T-05-02 ─→ T-05-03
T-05-04 (parallel)
T-05-05 (parallel)
T-05-06 (parallel, after T-05-02 for floor number)
```
Measure → set floor → close gaps is the critical path. Harness wiring, flaky fixes, and TESTING.md
run alongside (TESTING.md cites the floor from T-05-02).

## Status Notes

- **All 6 tasks completed (Wave 2).** Verified against a real git HEAD; local gates green
  (`go test ./... -race` 268 pass, `-count=2` 536 pass, coverage total **75.7%**).
  - **T-05-01** — per-package coverage measured (`coverage.out`, `-covermode=atomic`); the table is
    recorded in `TESTING.md`. Lowest package: `internal/core/verify` 51.2% (OS-path sandbox guards);
    agent-facing surfaces (MCP 88.2%, gates 83.5%, help palette) all above floor.
  - **T-05-02** — policy floor **ratcheted 74.0% → 75.0%** in `scripts/coverage-check.sh` (~0.7%
    headroom under measured 75.7%). Enforcement proven: temporarily set `FLOOR=99.0` → script exits
    1 ("total coverage 75.7% is below floor 99.0%"); reverted → exit 0. Documented in `TESTING.md`.
  - **T-05-03** — MCP tool-call marshaling contract pinned by new
    `mcp.TestSplitArgumentsContract` / `TestValueToStringFallback` (bumped `internal/mcp` 84.2% →
    88.2%); help-palette schema already covered by `cmd.TestHelpJSON`; gates by the parity/conformance
    suites. Targeted agent-facing packages meet the floor.
  - **T-05-04** — decision: regress harnesses stay **out of the CI matrix** (they exercise the
    planning `specs/` verify tables, not product behaviour — redundant per-PR cost). Documented
    cadence + owner recorded in `TESTING.md` and `scripts/README.md` (run before wave close /
    release; owner = maintainer).
  - **T-05-05** — `-count=2` leg confirmed green (536 pass); no order-dependent test exposed.
  - **T-05-06** — `TESTING.md` authored (suite commands, coverage floor + per-package table,
    regress cadence, stress jobs); linked from docs index. **`ci.yml:242`** (not `:232` as the
    acceptance text says — the line drifted) references `TESTING.md`; the reference now resolves.

  **Stale-prose correction:** the spec's "Current State" said `scripts/coverage-check.sh` "does not
  exist" and `TESTING.md` "does not exist". Both are resolved: SPEC-01 authored `coverage-check.sh`
  (provisional floor), this spec set the policy floor; `TESTING.md` is now authored here.
