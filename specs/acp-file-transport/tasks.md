# Tasks — ACP v1 File Transport

## Wave 2 — ACP Protocol

- [ ] T1 — Define ACP models and schema
  - why: All producers and consumers need one versioned contract.
  - role: builder
  - files: internal/core/acp.go, internal/core/schema/v1.json, internal/core/schema_test.go
  - contract: Define envelopes, payloads, message types, limits, validation, and schema generation/parity for ACP v1.
  - acceptance: Every message type round-trips; invalid versions, IDs, roles, slugs, task IDs, sizes, and payloads fail with deterministic diagnostics.
  - verify: go test ./internal/core/... -run 'TestACP|TestSchema' -count=2
  - depends: config-extension/T2
  - requirements: R2.1, R2.2, R2.3, R2.5, R2.10

## Wave 3 — Runtime Paths

- [ ] T2 — Implement runtime path and containment helpers
  - why: Untrusted IDs must never escape `.specd/runtime`.
  - role: builder
  - files: internal/core/runtime_paths.go, internal/core/runtime_paths_test.go
  - contract: Derive validated session, event, worker, lease, cursor, archive, and artifact paths with symlink-aware containment checks.
  - acceptance: Traversal, absolute paths, separators, symlink escapes, invalid slugs, and oversized IDs are rejected.
  - verify: go test ./internal/core/... -run 'TestRuntimePath' -count=2
  - depends: T1
  - requirements: R2.5

## Wave 4 — Atomic Event Store

- [ ] T3 — Implement atomic immutable event writer
  - why: Concurrent workers require durable messages with no partial visibility or overwrite.
  - role: builder
  - files: internal/core/acp_store.go, internal/core/acp_store_test.go
  - contract: Allocate session sequence under lock, write fsynced immutable 0600 event files, and reject duplicate IDs or sequence rollback.
  - acceptance: Concurrent writers produce a gap-free ordered event set with no truncation, overwrite, or temp-file leakage.
  - verify: go test ./internal/core/... -run 'TestACPStore.*(Write|Concurrent|Permission)' -race -count=2
  - depends: T1, T2
  - requirements: R2.4, R2.8, R2.9

## Wave 5 — Idempotent Delivery

- [ ] T4 — Implement idempotent event reader and cursor
  - why: At-least-once delivery must be safe across retries and restarts.
  - role: builder
  - files: internal/core/acp_store.go, internal/core/acp_cursor.go, internal/core/acp_store_test.go
  - contract: Read events in sequence order, validate each envelope, persist consumer cursors atomically, and deduplicate by message ID.
  - acceptance: Duplicate/replayed events produce one reconciliation result and corrupt/partial files fail closed.
  - verify: go test ./internal/core/... -run 'TestACPStore.*(Read|Cursor|Duplicate|Corrupt)' -count=2
  - depends: T3
  - requirements: R2.6, R2.12

## Wave 6 — Worker Leases

- [ ] T5 — Implement worker leases, heartbeat, TTL, and reclaim
  - why: Work ownership must survive crashes without accepting stale evidence.
  - role: builder
  - files: internal/core/acp_lease.go, internal/core/acp_lease_test.go
  - contract: Atomically claim, renew, expire, release, and reclaim worker leases using injected time and attempt numbers.
  - acceptance: Claim races have one winner; stale attempts and expired leases cannot submit accepted terminal reports.
  - verify: go test ./internal/core/... -run 'TestACPLease' -race -count=2
  - depends: T4
  - requirements: R2.7, R2.12

## Wave 7 — Archive and Replay

- [ ] T6 — Implement archive, replay integration, and retention cleanup
  - why: Sessions must remain auditable without unbounded runtime growth.
  - role: builder
  - files: internal/core/acp_archive.go, internal/core/replay.go, internal/cmd/replay.go, internal/core/acp_archive_test.go
  - contract: Seal terminal sessions, archive atomically, expose ordered events through replay, and delete only expired validated archives.
  - acceptance: Interrupted archive operations recover idempotently; replay order is deterministic; cleanup cannot escape the runtime root.
  - verify: go test ./internal/core/... ./internal/cmd/... -run 'TestACPArchive|TestReplay.*ACP' -count=2
  - depends: T4, T5
  - requirements: R2.11, R2.12

## Wave 8 — ACP Stress and Security

- [ ] T7 — Add ACP cross-process stress and hostile-input suite
  - why: Transport correctness is integrity-critical under adversarial input and concurrency.
  - role: verifier
  - files: scripts/stress-acp.sh, internal/core/acp_security_test.go, Makefile
  - contract: Stress concurrent writers/readers/reclaims and test traversal, symlinks, oversized payloads, stale leases, duplicate IDs, and crash artifacts.
  - acceptance: No lost messages, races, path escapes, duplicate accepted transitions, or nondeterministic ordering.
  - verify: make ci
  - depends: T6
  - requirements: R2.4, R2.5, R2.6, R2.8, R2.9, R2.10, R2.12
