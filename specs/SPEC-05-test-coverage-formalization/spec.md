# SPEC-05: Test Coverage Formalization

## Overview
- **Domain:** Test Coverage & Reliability (Analysis Plan Domain 6)
- **Risk Level:** Medium (tests are strong, but the coverage floor is currently a no-op)
- **Priority:** P2
- **Dependencies:** SPEC-01 (the coverage job must run) and SPEC-02 (verified feature map to target
  coverage against).

## Current State

- Test suite is ~5,819 LOC. `go test ./... -race` and `-count=2` (flaky/iteration-order catch)
  pass locally.
- **Coverage floor is broken (B2):** the `coverage-floor` CI job invokes
  `scripts/coverage-check.sh`, which **does not exist** — so the floor is unenforced today.
  SPEC-01 authors a `coverage-check.sh` with a *provisional* floor (current measured coverage) so
  CI goes green; **SPEC-05 owns the real policy** (target floor + per-package gaps).
- Existing parity/conformance tests: `internal/core/gates/parity_test.go` (handler/verb parity),
  `conformance_test.go`, `integration_polish_test.go`. Regression harnesses
  `scripts/regress-*.sh` re-run every task verify + each wave invariant but are **not wired into
  CI** (documented cadence unknown).
- **Doc gap:** no stated coverage target. `TESTING.md` is referenced by `ci.yml:232` but **does
  not exist** in the repo (confirmed this session) — the assumed file is missing.

## Target State

An enforced coverage floor at a deliberate policy target (ratcheted up from SPEC-01's provisional
value); per-package coverage gaps identified and closed where they matter (MCP contract,
help-palette schema, agent-facing surfaces); the `regress-*.sh` harnesses wired into CI or a
documented cadence; a real, accurate `TESTING.md`.

## Scope Boundaries

- **In Scope:** the coverage floor policy value and ratchet; `scripts/coverage-check.sh` policy
  (SPEC-01 creates the file — SPEC-05 sets the meaningful floor); per-package coverage measurement
  and gap-closing; wiring `regress-*.sh` into CI/cadence; authoring `TESTING.md`; MCP contract +
  help-schema tests; the `-count=2` order-dependence leg.
- **Out of Scope:** creating the missing CI scripts' *existence* (SPEC-01); the verb→handler→doc
  *map* itself (SPEC-02, consumed here); security-scanner tests (SPEC-04); anything under
  `reference/`.

## Technical Requirements

1. **Coverage policy:** set an explicit floor target and update `coverage-check.sh` to enforce it;
   ratchet from SPEC-01's provisional value. The script must fail when total (or per-package, if
   the policy is per-package) coverage drops below the floor.
2. **Per-package gaps:** produce `coverage.out`, measure per-package, and close gaps on
   agent-facing/critical surfaces — MCP contract tests (`internal/mcp/`) and help-palette schema
   tests especially. Target the verified feature map from SPEC-02.
3. **Wire regress harnesses:** add `regress-all.sh` / `regress-domains.sh` / `regress-lint.sh` to
   CI, or document a required cadence with an owner.
4. **Flaky-test discipline:** keep the `-count=2` leg; fix any order-dependent test it exposes.
5. **`TESTING.md`:** author an accurate testing guide matching what `ci.yml:232` expects (how to
   run the suite, the coverage floor, the regress harnesses, the stress jobs).

## Verification Strategy

- `go test ./... -race -count=1` and `-count=2` green; `coverage-check.sh` fails on an induced
  coverage drop below floor (prove with a temporary stub).
- `coverage.out` produced in CI; per-package numbers recorded; targeted packages meet the policy.
- `ci.yml` runs the regress harnesses (or a documented cadence exists with an owner).
- `TESTING.md` exists, is accurate, and the `ci.yml:232` reference resolves.
- No LLM in gate/DAG/report paths; no bypass flag; `reference/` untouched.

## References
- Analysis Plan: Domain 6; Recommended Spec Breakdown row SPEC-05; Appendix assumption that
  `TESTING.md` exists (now known false — SPEC-05 authors it).
- Related Specs: SPEC-01 (creates coverage-check.sh + runs the job), SPEC-02 (feature map to
  target).
- Source Files: all `*_test.go` (~5,819 LOC), `internal/core/gates/parity_test.go`,
  `conformance_test.go`, `integration_polish_test.go`, `internal/mcp/`, `scripts/regress-*.sh`,
  `scripts/coverage-check.sh` (created in SPEC-01), `.github/workflows/ci.yml`.
