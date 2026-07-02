# S4 Tasks — Verify & Sandbox Regression

Requirement coverage: R4, R11. Dependencies: none (Wave-1 root).

## Wave 1 — Baseline

- [ ] Record current runner behavior: exit code on success, non-zero, and
  timeout (expect `124`). Files: `internal/runner/runner_test.go`.
- [ ] Detect host sandbox availability (`bwrap`, container runtime) to gate
  skips. File: `internal/runner/runner_sandbox_test.go` (extend).
- **Validation:** `go test ./internal/runner/... -race -count=1`

## Wave 2 — Core regression tests (depends on Wave 1)

- [ ] `shRunner` byte-for-byte reproduction: stdout/stderr/exit match a direct
  `sh -c`. File: `internal/runner/runner_test.go`.
- [ ] Timeout path: assert `TimedOut=true` and `ExitCode==124`. File:
  `internal/runner/runner_test.go`.
- [ ] Fail-closed: selecting an unavailable backend returns a refusal, never a
  `none` fallback. File: `internal/runner/runner_sandbox_test.go`.
- [ ] Custom gates: run with scrubbed env + timeout; assert env leakage blocked.
  File: `internal/core/customgate_test.go` (extend).
- **Validation:** `go test ./internal/runner/... -race -count=1 && go test ./internal/core/... -run CustomGate -race -count=1`

## Wave 3 — Sandbox-present (manual/skip-guarded) (depends on Wave 2)

- [ ] Guarded bwrap/container execution test (`t.Skip` when tool absent) with a
  rationale message. File: `internal/runner/runner_sandbox_run_cov_test.go`.
- [ ] Document manual sandbox validation steps in `TESTING.md`.
- **Validation:** `go test ./internal/runner/... -count=2`

## Rollout & cleanup

- [ ] Confirm `internal/runner` ≥92% (`make cover-check`).
- **Rollback:** revert extensions; skip guards keep CI green.
- **Completion evidence:** green runner + custom-gate tests; documented skips.
