# S4 — Verify & Sandbox Regression

## 1. Purpose and requirement coverage

Guarantee verify execution works across shell/bwrap/container backends with
fail-closed isolation and scrubbed-env custom gates. Covers **R4** and **R11**.

## 2. Verified current state

- Runner interface + default backend: `internal/runner/runner.go` — `Runner`
  interface (`Name()`, `Run()`), `shRunner` reports `Name() == "none"`.
  Timeout maps to exit code `124`; non-exec errors also `124`.
- Isolating backends: `internal/runner/runner_sandbox.go` (bwrap/container),
  tested by `runner_sandbox_test.go`, `runner_sandbox_run_cov_test.go`.
- Policy owner (env scrub, NUL rejection, shell selection): `internal/cmd/verify.go`
  (comment in `runner.go:14-24` — runners re-derive no policy).
- Custom gates: `internal/core/customgate.go` + `runCustomGates` in
  `internal/core/gates.go`, tested by `customgate_test.go`,
  `customgate_pipeline_test.go`.
- Coverage floor `internal/runner` = **92%** (`scripts/coverage-check.sh`).

## 3. Proposed design and end-to-end flow

Tests assert: `shRunner` reproduces historical `sh -c` execution byte-for-byte;
timeout yields `TimedOut=true` + exit `124`; sandbox backends refuse (fail
closed) when their host tool (`bwrap`/container runtime) is absent rather than
silently downgrading to `none`; custom gates run with a scrubbed env and a
timeout, unisolated on host, and their sandbox identity is recorded as evidence.
Sandbox-present paths are guarded by `t.Skip` when the host tool is missing.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** `RunSpec`/`RunResult` fields; `Runner.Name()` evidence strings
  (`none`/`bwrap`/`container`); fail-closed selection contract.
- **Config:** sandbox selection flag on `verify`; env scrub list owned by
  `cmd/verify.go`.
- **Dependencies:** none for shell path; S10 consumes the isolation guarantees.

## 5. Invariants, security, errors, observability, compatibility, rollback

- **INV4** (fail-closed sandboxing): missing backend refuses, never silent
  fallback.
- **Security:** custom gates are a trust boundary — run unisolated with scrubbed
  env + timeout (documented in `SECURITY.md`); evidence records the backend.
- **Observability:** `obs.RecordDuration("verify_run_duration", …)` fires per run
  (`runner.go:57`).
- **Rollback:** additive tests + `t.Skip` guards; no behavior change.

## 6. Acceptance criteria and validation commands

- `go test ./internal/runner/... -race -count=1` passes.
- `go test ./internal/core/... -run 'CustomGate' -race -count=1` passes.
- Fail-closed: with `bwrap` absent, selecting it returns a refusal error, not a
  `none` run (asserted or `t.Skip` with rationale when tool present).
- `internal/runner` coverage ≥92% (`make cover-check`).

## 7. Open decisions and deviations

- Deviation U4: bwrap/container tools are not in CI runners; sandbox-present
  assertions are `t.Skip`-guarded and documented as manual-validation in
  `TESTING.md`. Fail-closed (tool-absent) path IS exercised in CI.
