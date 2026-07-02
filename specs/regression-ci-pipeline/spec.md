# S13 — CI Pipeline Regression

## 1. Purpose and requirement coverage

Guarantee CI runs reliably, completely, and quickly. Covers **R15**.

## 2. Verified current state

- CI: `.github/workflows/ci.yml` jobs: `lint`, `analyze` (govulncheck +
  golangci-lint v2.1.6), `test` (ubuntu/macOS × go `1.22`/`stable`, `-race`
  + `-count=2` + `perf-gate`), `coverage-floor`, `stress`, `stress-brain-recovery`,
  `stress-checkpoint-fault`, `build` (ubuntu/macOS/windows).
- Release: `.github/workflows/release.yml` — tag-triggered, `make ci` then
  GoReleaser + SBOM.
- Local mirror: `make ci` = `lint test test-order cover-check perf-gate stress
  stress-acp stress-orchestration stress-program stress-brain-recovery
  stress-checkpoint-fault` (Makefile:94).
- Lint config: `.golangci.yml` (staticcheck, errcheck, govet, gosec, gocyclo,
  revive, etc.).

## 3. Proposed design and end-to-end flow

Regression = `make ci` is green locally and every CI job passes on PR within a
reasonable wall-clock budget. Assert parity between the local `make ci` target
and the CI job set so a passing local run predicts a passing CI run. Detect
job/target drift (e.g., a stress script added to Makefile but not CI).

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** the `make ci` target composition; the CI job list; golangci-lint
  pinned version (v2.1.6).
- **Dependencies:** S1–S12 (CI runs all of them).

## 5. Invariants, security, errors, observability, compatibility, rollback

- **Invariant:** `make ci` and CI cover the same gates (no silent divergence).
- **Compatibility:** pinned tool versions prevent toolchain drift (F9).
- **Rollback:** workflow edits are git-revertible.

## 6. Acceptance criteria and validation commands

- `make ci` passes locally.
- All CI jobs green on PR.
- `make ci` target and `ci.yml` job set stay in parity (documented check).

## 7. Open decisions and deviations

- Deviation D5: analysis plan under-lists CI stress jobs. Verified jobs include
  `stress`, `stress-brain-recovery`, `stress-checkpoint-fault` in CI, and the
  Makefile adds `stress-acp`, `stress-orchestration`, `stress-program`. Note the
  gap: three stress targets run in `make ci` but not (yet) as dedicated CI jobs —
  flag for parity.
- F9 open: add a `go mod tidy` check to guard toolchain/deps drift.
