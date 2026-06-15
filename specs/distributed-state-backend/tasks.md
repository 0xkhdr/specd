# tasks.md — Distributed State Backend execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Contract recon

- [ ] **T1 — Document the exact current lock + CAS contract**
  - role: investigator · depends: — · requirements: R1,R2
  - Map `lock.go` reentrancy, `SaveState` CAS + `assertLocked`, atomic write
    sequence. This is the contract every backend must honor. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<contract doc>"`

## Wave 2 — Interface + conformance net

- [ ] **T2 — Extract `StateBackend` interface; file backend behind it**
  - role: builder · depends: T1 · requirements: R1,R2,R7
  - No behavior change for `file`; existing suite passes unchanged.
  - verify: `go test ./internal/core/ -race -count=2`

- [ ] **T3 — Backend-agnostic conformance test suite**
  - role: verifier · depends: T2 · requirements: R3,R6
  - Lift concurrency + CAS tests into a suite any backend runs (stale-base CAS
    fail, reentrant lock, 32-goroutine serialization).
  - verify: `go test ./internal/core/ -run TestBackendConformance -race -count=2`

## Wave 3 — git backend + remote adapters

- [ ] **T4 — git-native backend (no Go dep)**
  - role: builder · depends: T3 · requirements: R5,R3
  - Commit state; CAS via expected parent SHA; lock via lock ref. Passes T3.
  - verify: `go test ./internal/core/ -run TestBackendConformance/git -race -count=2`

- [ ] **T5 — Redis/Postgres adapters behind build tags**
  - role: builder · depends: T3 · requirements: R4
  - `//go:build specd_redis` etc.; default build imports neither.
  - verify: `go build -tags specd_redis ./... && go vet ./...`

- [ ] **T6 — Test: default binary links no DB/redis driver**
  - role: verifier · depends: T5 · requirements: R4
  - Assert via `go list -deps ./...` (default tags) shows no driver module.
  - verify: `make ci`

- [ ] **T7 — Review: integrity spine unweakened**
  - role: reviewer · depends: T4,T6 · requirements: R2,R3
  - verify: N/A — complete with `--unverified --evidence "<spine audit>"`

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R2 |
| 2 | T2–T3 | R1, R2, R3, R6, R7 |
| 3 | T4–T7 | R2, R3, R4, R5 |
