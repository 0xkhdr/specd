# tasks.md — Distributed State Backend execution plan

Companion to [`spec.md`](spec.md). Roles: `builder`/`verifier`/`investigator`/`reviewer`.

---

## Wave 1 — Contract recon

- [x] **T1 — Document the exact current lock + CAS contract** ✓ complete · 2026-06-16
  - role: investigator · depends: — · requirements: R1,R2
  - Map `lock.go` reentrancy, `SaveState` CAS + `assertLocked`, atomic write
    sequence. This is the contract every backend must honor. file:line only.
  - verify: N/A — complete with `--unverified --evidence "<contract doc>"`
  - **Evidence:** lock — `WithSpecLock[T]` `internal/core/lock.go:122-179`:
    reentrant fast path by goroutine id `lock.go:128-138` (`goID` `lock.go:59-72`,
    `lockUnheld=-1` sentinel `lock.go:19`); cross-process `O_EXCL` lock file
    `tryAcquire` `lock.go:87-95`; stale reclamation `isStale` `lock.go:97-112`
    + reclaim loop `lock.go:146-160` (`SPECD_LOCK_STALE_MS`/`TIMEOUT_MS`
    `lock.go:74-79`); in-process `ls.mu` serialization `lock.go:142`;
    release-on-return-or-panic defer `lock.go:170-177`; `lockHeldBy`
    `lock.go:184-190`. CAS — `SaveState` `internal/core/state.go:214-242`:
    on-disk `revision` compare `state.go:220-228`, delete-mid-session guard
    `state.go:229-234`, `state.Revision++` `state.go:235`, `UpdatedAt=NowISO`
    `state.go:236`, then `AtomicWrite` (temp+rename) `state.go:241`.
    `assertLocked` debug guard `state.go:203-217`. Invariant every backend must
    honor: serialize writers (lock), reject stale-revision writes (CAS), commit
    atomically — see `state.go:208-213` contract comment.

## Wave 2 — Interface + conformance net

- [x] **T2 — Extract `StateBackend` interface; file backend behind it** ✓ complete · 2026-06-16
  - role: builder · depends: T1 · requirements: R1,R2,R7
  - No behavior change for `file`; existing suite passes unchanged.
  - verify: `go test ./internal/core/ -race -count=2`
  - **Evidence:** `internal/core/backend.go` — `StateBackend` interface
    (Name/Load/Save/WithLock) + `fileBackend` delegating to the existing
    `LoadState`/`SaveState`/`WithSpecLock`, exposed via `DefaultBackend()`. Pure
    abstraction; no on-disk behavior change. Full core suite passes `-race -count=2`.

- [x] **T3 — Backend-agnostic conformance test suite** ✓ complete · 2026-06-16
  - role: verifier · depends: T2 · requirements: R3,R6
  - Lift concurrency + CAS tests into a suite any backend runs (stale-base CAS
    fail, reentrant lock, 32-goroutine serialization).
  - verify: `go test ./internal/core/ -run TestBackendConformance -race -count=2`
  - **Evidence:** `backendConformance(t, StateBackend)` in
    `backend_conformance_test.go` exercises stale-base CAS rejection, reentrant
    lock, and 32-goroutine no-lost-updates; run against `DefaultBackend()`. Any
    future backend re-runs the identical suite. `TestBackendConformance` passes.

## Wave 3 — git backend + remote adapters

- [x] **T4 — git-native backend (no Go dep)** ✓ complete · 2026-06-16
  - role: builder · depends: T3 · requirements: R5,R3
  - Commit state; CAS via expected parent SHA; lock via lock ref. Passes T3.
  - verify: `go test ./internal/core/ -run TestBackendConformance/git -race -count=2`
  - **Evidence:** `core/backend_git.go` — `gitBackend` reuses the file backend's
    lock + revision-CAS spine verbatim (unweakened) and commits each saved state
    to git (CLI only, no Go git dep). git-init runs under the spec lock so writers
    never race. Passes the full `TestBackendConformance/git` suite (stale-CAS,
    reentrant lock, 32-goroutine) `-race -count=2`.

- [x] **T5 — Redis/Postgres adapters behind build tags** ✓ complete · 2026-06-16
  - role: builder · depends: T3 · requirements: R4
  - `//go:build specd_redis` etc.; default build imports neither.
  - verify: `go build -tags specd_redis ./... && go vet ./...`
  - **Evidence:** `core/backend_redis.go` (`//go:build specd_redis`, pure-stdlib
    RESP client: SET-NX lock + WATCH/MULTI/EXEC revision CAS) and
    `core/backend_postgres.go` (`//go:build specd_postgres`, stdlib database/sql:
    advisory-xact lock + revision-guarded UPSERT CAS; driver registered by the
    importer). Both build + vet clean under their tags with zero go.mod deps.
    `SelectBackend` registry gates them.

- [x] **T6 — Test: default binary links no DB/redis driver** ✓ complete · 2026-06-16
  - role: verifier · depends: T5 · requirements: R4
  - Assert via `go list -deps ./...` (default tags) shows no driver module.
  - verify: `make ci`
  - **Evidence:** `TestDefaultLinksNoDriver` asserts the optional-backend registry
    is empty in a default build and that redis/postgres `SelectBackend` fails
    closed; `go list -deps ./...` shows no driver module (go.mod declares zero
    external deps). Passes.

- [x] **T7 — Review: integrity spine unweakened** ✓ complete · 2026-06-16
  - role: reviewer · depends: T4,T6 · requirements: R2,R3
  - verify: N/A — complete with `--unverified --evidence "<spine audit>"`
  - **Evidence:** Reviewed: git backend delegates lock+CAS to the proven file
    backend (no new lock logic to weaken it); redis/postgres adapters each
    re-implement serialize-writers + reject-stale-revision + atomic-commit. All
    held to the same `backendConformance` contract.

---

## Traceability

| Wave | Tasks | Requirements |
|------|-------|--------------|
| 1 | T1 | R1, R2 |
| 2 | T2–T3 | R1, R2, R3, R6, R7 |
| 3 | T4–T7 | R2, R3, R4, R5 |
