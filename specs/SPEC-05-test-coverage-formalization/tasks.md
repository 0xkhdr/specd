# SPEC-05 Tasks: Test Coverage Formalization

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-05-01 | Measure per-package coverage | Produce `coverage.out`; record per-package percentages; identify gaps on critical/agent-facing surfaces. | A per-package coverage table exists; gap list produced. | Small | pending |
| T-05-02 | Set coverage floor policy | Decide the policy floor; update `coverage-check.sh` (from SPEC-01) to enforce it; ratchet above the provisional value. | Script fails on coverage below the policy floor (proven with a temporary stub); floor documented. | Medium | pending |
| T-05-03 | Close targeted gaps | Add tests for MCP contract (`internal/mcp/`), help-palette schema, and other flagged gaps, targeting SPEC-02's feature map. | Targeted packages meet the policy floor; new tests pass under `-race`. | Large | pending |
| T-05-04 | Wire regress harnesses | Add `regress-all.sh`/`regress-domains.sh`/`regress-lint.sh` to CI or document a required cadence + owner. | Harnesses run in CI, or a documented cadence with an owner exists. | Small | pending |
| T-05-05 | Fix order-dependence | Keep the `-count=2` leg; fix any flaky/order-dependent test it exposes. | `go test ./... -count=2` green and deterministic. | Medium | pending |
| T-05-06 | Author TESTING.md | Write an accurate testing guide (suite commands, coverage floor, regress harnesses, stress jobs) that `ci.yml:232` references. | `TESTING.md` exists, accurate; ci.yml:232 reference resolves. | Small | pending |

## Task Dependency Graph

```
T-05-01 ─→ T-05-02 ─→ T-05-03
T-05-04 (parallel)
T-05-05 (parallel)
T-05-06 (parallel, after T-05-02 for floor number)
```
Measure → set floor → close gaps is the critical path. Harness wiring, flaky fixes, and TESTING.md
run alongside (TESTING.md cites the floor from T-05-02).
