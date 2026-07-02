# V1 Tasks — State Schema v2

Plan coverage: P1.1. Dependencies: none. Dependents: V2–V12.

## Wave 1 — Schema + migration

- [ ] Add v6 block types (`mode` enum ext, `evals`, `routing`, `conductor`,
  `escalation`) with `omitempty` to `internal/core/state.go`; bump
  SchemaVersion 5 → 6.
- [ ] Loader migration v5 → v6 (silent, idempotent) following the existing
  migration pattern; writer always emits v6.
- [ ] Extend `mode` validation: accept `conductor`, reject unknown values with
  actionable error.
- **Validation:** `go test ./internal/core/... -run 'State|Mode' -race -count=1`

## Wave 2 — Compatibility + hostile input (depends on Wave 1)

- [ ] v5 fixture round-trip tests: load → save → v6 → reload byte-equivalent
  semantics; existing state tests pass unmodified.
- [ ] Migration idempotency test (migrate twice = migrate once).
- [ ] Corruption table tests for new blocks (truncated, wrong types, oversize)
  extending existing corruption-test patterns; fail closed, no partial writes.
- [ ] CAS/lock regression: `make stress` green with v6 writer.
- **Validation:** `go test ./internal/core/... -race -count=2 && make stress`

## Rollout & cleanup

- [ ] CHANGELOG entry (additive schema v6); docs note in `docs/user-guide` state
  section; `specd migrate` docs deferred to V12.
- **Rollback:** revert schema bump; new blocks are `omitempty` so absent-block
  reads remain valid.
- **Completion evidence:** `make ci` green; v5 fixture upgrade test committed.
