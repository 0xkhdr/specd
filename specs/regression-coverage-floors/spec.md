# S12 — Coverage Floor Regression

## 1. Purpose and requirement coverage

Guarantee statement-coverage floors are maintained (ratchet, never lowered).
Covers **R14**.

## 2. Verified current state

- Gate: `scripts/coverage-check.sh`, run by `make cover-check` and the CI
  `coverage-floor:` job.
- Verified floors (env-overridable defaults in the script):
  `OVERALL_MIN=79`, `CORE_MIN=80`, `CMD_MIN=71`, `WORKER_MIN=88`, `MCP_MIN=88`,
  `HARNESS_MIN=80`, `SPEC_MIN=99`, `CONTEXT_MIN=91`, `RUNNER_MIN=92`,
  `PACK_MIN=86`, `SCHEMA_MIN=83`.
- Long-term targets (per `TESTING.md`): 85% overall, 90%→95% for
  `internal/core`. Floors are the regression ratchet on the way there.

## 3. Proposed design and end-to-end flow

Regression = `make cover-check` stays green as new tests land, and floors only
ratchet upward. When a package's measured coverage rises with ≥1% headroom, nudge
its floor up (as the script's history documents). Never lower a floor to pass a
red build.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** the floor variable names + the eleven gated packages.
- **Dependencies:** S1–S10 (their tests raise/maintain coverage).

## 5. Invariants, security, errors, observability, compatibility, rollback

- **Invariant:** floors are monotonic (ratchet up only).
- **Rollback:** floor changes are single-line and revertible; a lowered floor
  requires PR justification.

## 6. Acceptance criteria and validation commands

- `make cover-check` passes (all eleven package floors met).
- No floor lowered without documented justification in the PR.
- After new tests, floors with ≥1% headroom are raised one step.

## 7. Open decisions and deviations

- Deviation D3: analysis plan lists floors as overall≥79, core≥80, cmd≥71,
  worker≥50 and "some packages lack floors." **Corrected:** worker floor is
  **88**, and floors exist for mcp(88), harness(80), spec(99), context(91),
  runner(92), pack(86), schema(83). The plan's F6 recommendation to "raise
  worker to 70" is already exceeded.
