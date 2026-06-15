# spec.md — Distributed Locking + Remote State Backend

**Status:** proposed
**Source:** specd-report.html §8 idea **C2** (impact: med · effort: high · moat: med)
**Date:** 2026-06-16
**Scope:** pluggable state backend behind the existing lock + CAS contract; `internal/core/state.go`, `lock.go`.

---

## 1. Objective

The advisory file lock + compare-and-swap works locally. For agents on different
machines sharing a spec, add a **pluggable state backend** (git-native, or
Redis/Postgres) honoring the **same CAS contract**, keeping the "atomic,
versioned" promise across hosts. Cloud agent platforms run subagents on separate
workers; shared, consistent state is the precondition for distributed fan-out.

> **Hard invariant:** the *shipped binary stays stdlib-only and zero-dependency*.
> The local file backend remains the default and is unchanged. Remote backends
> (Redis/Postgres) are **opt-in build tags or out-of-process adapters** — they
> must NOT pull a driver into the default build. The git-native backend uses the
> `git` CLI (already used for HEAD capture), so it adds no Go dependency. Every
> backend honors the exact same lock + revision-CAS semantics, or it is wrong.

## 2. Context

- `internal/core/lock.go` — reentrant per-spec advisory lock (goroutine-id
  keyed); `state.go` — `SaveState` revision-CAS with `assertLocked` guard,
  atomic temp+fsync+rename. Tested by `concurrency_test.go`, `lock_test.go`,
  `state_cas_test.go`.
- These are the integrity spine — the regression spec names them as invariants
  not to weaken.

## 3. Requirements (EARS)

- **R1 (H)** THE SYSTEM SHALL define a `StateBackend` interface capturing exactly
  the current contract: `Lock/Unlock` (advisory, reentrant per spec) and
  `Load/Save` with revision compare-and-swap.
- **R2 (H)** THE SYSTEM SHALL keep the local-file implementation as the default
  backend with byte-identical behavior and the existing test suite passing
  unchanged.
- **R3 (H)** WHERE a remote backend is selected, a `Save` whose base revision no
  longer matches the store SHALL fail the CAS exactly as the local backend does
  (no last-writer-wins).
- **R4 (M)** THE SHIPPED DEFAULT BINARY SHALL NOT link any database/redis driver;
  remote backends SHALL be behind build tags or an external adapter process.
- **R5 (M)** THE SYSTEM SHALL provide a git-native backend using the `git` CLI
  (commit state changes; CAS via expected parent SHA) with no Go dependency.
- **R6 (M)** THE SYSTEM SHALL run the existing concurrency/CAS test suite against
  every backend via a shared conformance test.
- **R7 (L)** Backend selection SHALL be config-driven (`state.backend`) and
  default to `file`.

## 4. Design / approach

1. **Interface extraction** — define `StateBackend` in `internal/core`; refactor
   `state.go`/`lock.go` callers to use it. The file backend is the current code
   behind the interface.
2. **Conformance suite** — lift `concurrency_test.go`/`state_cas_test.go` into a
   backend-agnostic conformance test any backend must pass.
3. **git backend** — `internal/core/backend/git.go`: serialize state to a tracked
   file, `git commit`; CAS by asserting the expected parent SHA before commit;
   advisory lock via a lock ref/branch.
4. **remote adapters** — Redis/Postgres behind build tags
   (`//go:build specd_redis`) so default builds never import them.

## 5. Non-goals

- No driver in the default binary; no change to the default `file` behavior.
- No new consensus protocol — CAS + a coordinating store is the model.
- No change to gate semantics or the evidence gate.

## 6. Acceptance criteria

- `StateBackend` interface exists; the file backend passes the existing suite
  byte-for-byte (the integrity spine is unweakened).
- A backend-agnostic conformance test runs against file + git backends; both
  enforce revision CAS (stale base ⇒ fail, no last-writer-wins).
- Default `go build` links no DB/redis driver (asserted via `go list -deps`).
- `state.backend` selects the backend, default `file`; `make ci` green.
