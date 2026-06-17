# Design — Regression: State Backends (default/git/postgres/redis, locking, CAS)

## Overview
Drive all backends through the existing `backend_conformance_test.go` harness and ensure
the suite is parameterized over every implementation, with graceful skips when a service
(postgres/redis) is absent in CI. The contract is the `Backend` interface in backend.go;
the regression locks that contract behaviorally.

## Architecture
```
Backend interface (backend.go)
  ├─ backend_default.go   (filesystem)   ── always run
  ├─ backend_git.go       (git commits)  ── run if git present
  ├─ backend_postgres.go  (SQL CAS)      ── run if PG dsn set, else skip
  └─ backend_redis.go     (WATCH/MULTI)  ── run if redis url set, else skip
        shared: backend_conformance_test.go (table over all impls)
```

## Components and interfaces
- **Backend.Read/Write/CAS/Lock/Unlock** — the conformance surface.
- **conformance harness** — one test body, N backends; asserts R1-R4 per impl.
- **skip policy** — env-gated (PG_DSN, REDIS_URL); missing service => t.Skip with reason.

## Data models
Key = spec slug; value = state.json bytes (schema v4). Revision is an integer guarded by CAS.

## Error handling
Stale CAS -> conflict error. Unavailable service -> skip, not fail. Interrupted write ->
atomic rename / transaction rollback so no partial state persists.

## Verification strategy
- Conformance: `go test ./internal/core/ -run Conformance` across available backends.
- CAS races: concurrent writers, assert exactly one winner per revision.
- Git backend: assert one commit per write and `specd replay` reconstructs timeline.
- Skips are visible: CI log shows which backends ran vs skipped.

## Risks and open questions
- CI may lack postgres/redis; conformance parity is only proven where the service runs.
  Mitigation: document required services in TESTING.md and gate a nightly job. Open: should
  skipped backends fail the gate in release CI? Decision deferred to release pipeline owner.
