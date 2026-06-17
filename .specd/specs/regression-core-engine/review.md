# T6 Review — core regression suite (T3–T5)

Verify: `go test ./internal/core/... -cover -race` → ok, **coverage 70.2%** (≥ T1 baseline 66.4%). No race, no flake across `-count=3` (T3).

## Findings (one line each)
- dag_regression_test.go:33 ✓ ok: permutation determinism exercises all 4! input orders; frontier + wave rows stable — no map-iteration leak.
- dag_regression_test.go:71 ✓ ok: incomplete-dep exclusion table covers pending/running/blocked/missing — R1.4 fully pinned.
- dag_regression_test.go:107 ✓ ok: cycle path asserted closed + CriticalPath refusal — R1.2 enforced, no source change.
- gates_regression_test.go:16 ✓ ok: PhaseReadiness block/clear across requirements/design/tasks — R2.1 at engine scope.
- gates_regression_test.go:108 ✓ ok: custom-gate order asserts core-before-custom and config order — R2.4 strengthened beyond prior count-only check.
- gates_regression_test.go:152 ✓ ok: telemetry roll-up sums stored tokens/cost verbatim — R3.3 proves no pricing/compute.
- lock_regression_test.go:24 ✓ ok: stale-lock reclaim + write re-validates state.json — R6.2 "without corruption" now asserted, not just reclaimed.
- lock_regression_test.go:68 ✓ ok: 16-way contended write validates schema on every commit and final Turn==N — R6.1/R6.3 under -race.
- runner_sandbox_test.go:14 ✓ ok: SelectRunner("none") exit=3 + stderr/stdout byte-exact — R5.2/R5.3 verbatim.

## Scope notes (not gaps)
- R2.1 full gate-block (`approve`/`task` refusing while awaiting-approval) and R3.2 `--unverified` bypass are CLI-layer — ADR-001 assigns them to regression-cli-cmd (wave 3). Engine primitives they rely on are covered here.
- No existing assertion was weakened or rewritten; T3–T5 are additive (new files + one import block).

## Verdict
Zero UNMAPPED criteria at core scope. Coverage above floor. Deterministic under race. **Pass.**
