# S2 — State Atomicity Regression

## 1. Purpose and requirement coverage

Guarantee state writes stay atomic and lost-update-free under cross-process
contention. Covers **R2** (CAS + advisory lock prevent lost updates).

## 2. Verified current state

- Advisory locking: `internal/core/lock.go` (`WithSpecLock`), tested by
  `lock_test.go`.
- State load/save + revision CAS: `internal/core/state.go` (`SchemaVersion = 5`,
  `state.go:16`). CAS behavior tested in `state_cas_test.go`;
  reject-on-newer-schema in `state_resume_reject_test.go`.
- Concurrency tests: `internal/core/concurrency_test.go`,
  `write_failure_cov_test.go`, `json_invariant_test.go`.
- Cross-process shell stress: `scripts/stress.sh` (16×20 short CLI writes to one
  spec) driven by `make stress`; additional ACP/program/checkpoint stress via
  `scripts/stress-*.sh`.

## 3. Proposed design and end-to-end flow

Regression tests assert: revision monotonically increments on each `SaveState`;
a stale-revision write is rejected (CAS miss); concurrent writers under
`WithSpecLock` serialize with `turn == successes` and no torn JSON; a save that
fails mid-write leaves the prior state intact (no partial file). Cross-process
guarantee is exercised by `make stress`; in-process by `-race` Go tests.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** `state.json` schema version 5; advisory-lock + revision-CAS
  contract; atomic temp-file-then-rename write path.
- **Dependencies:** none (Wave-1 root).

## 5. Invariants, security, errors, observability, compatibility, rollback

- **INV2** (atomic writes under lock with revision bump).
- **INV5** (backward-compatible state.json: newer schema rejected, older
  migrated).
- **Errors:** CAS miss returns a typed conflict, not a silent overwrite.
- **Compatibility:** on-disk format for v5 is frozen; any bump needs a migration.
- **Rollback:** additive tests; revert by deleting new `*_test.go`.

## 6. Acceptance criteria and validation commands

- `go test ./internal/core/... -run 'CAS|Lock|Concurrency|Invariant' -race -count=1`
  passes.
- `make stress` passes (turn == successes, no torn writes).
- `go test ./internal/core/... -count=2` stable.

## 7. Open decisions and deviations

- Deviation D3: analysis plan states `internal/worker` floor 50% and no
  `internal/core`-specific stress harness beyond `stress.sh`; repo actually ships
  `stress-acp/orchestration/program/brain-recovery/checkpoint-fault.sh`. These
  are folded into the atomicity picture where they touch shared state.
