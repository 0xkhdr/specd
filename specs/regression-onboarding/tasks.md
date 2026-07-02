# S6 Tasks — Onboarding Regression

Requirement coverage: R6. Dependencies: S5.

## Wave 1 — Baseline (after S5 green)

- [ ] Run `make perf-gate` and record pass/fail; capture the deterministic
  receipt bytes. Files: `internal/cmd/init.go`, `init_benchmark_test.go`.
- [ ] Inventory pack/host-detection tests: `initpack_test.go`,
  `onboarding_test.go`, `internal/core/embed_drift_test.go`.
- **Validation:** `make perf-gate`

## Wave 2 — Core regression tests (depends on Wave 1)

- [ ] Idempotency: run `init` twice, assert byte-identical receipt. File:
  `internal/cmd/init_test.go` (extend).
- [ ] Embedded-pack byte stability. File: `internal/core/embed_drift_test.go`
  (extend).
- [ ] Host detection determinism given fixed env. File:
  `internal/cmd/onboarding_test.go` (extend).
- [ ] MCP registration writes a stable config shape. File:
  `internal/cmd/handshake_mcp_test.go` (extend).
- **Validation:** `go test ./internal/cmd/... -run 'Init|Onboarding|Pack|Handshake' -race -count=1`

## Wave 3 — Determinism gate (depends on Wave 2)

- [x] Ensure new tests are `-count=2` clean and folded into `perf-gate`'s
  `Deterministic` run filter where relevant.
- **Validation:** `make perf-gate`

## Rollout & cleanup

- [ ] Confirm `internal/cmd` ≥71% and `internal/pack` ≥86% (`make cover-check`).
- **Rollback:** revert test extensions.
- **Completion evidence:** green `make perf-gate` at count=2.
