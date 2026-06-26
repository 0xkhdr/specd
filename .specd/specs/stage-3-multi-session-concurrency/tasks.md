# Tasks — Stage 3 — Multi-Session Concurrency

## Wave 1
- [x] T1 — Audit active-spec selection and lock reporting paths
  - why: We need to understand where the current global selection and raw lock errors flow through the MCP stack.
  - role: investigator
  - files: internal/mcp/watcher.go, internal/mcp/server.go, internal/cli/args.go, internal/core/lock.go, internal/mcp/tools.go
  - contract: Map the active-spec resolution, watcher bootstrap, CLI argument parsing, and existing lock-error propagation points.
  - acceptance: The session-affinity and structured-lock change points are identified precisely.
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 3, 4

## Wave 2
- [x] T2 — Add per-session spec affinity to the MCP command
  - why: Requirements 1–3 need a pinned slug to remove cross-session ambiguity.
  - role: builder
  - files: internal/cli/args.go, internal/mcp/server.go, internal/mcp/watcher.go, internal/mcp/watcher_test.go
  - contract: Add `--spec`, thread the slug into watcher startup, and make active-spec selection obey the pin when present while preserving the old fallback when absent.
  - acceptance: A pinned watcher only follows one slug; an unpinned watcher behaves exactly as before.
  - verify: go test ./internal/mcp ./internal/cli -count=1
  - depends: T1
  - requirements: 1, 2, 3, 5

- [x] T3 — Add structured lock status to tool results
  - why: Requirement 4 needs hosts to distinguish a blocked session from a generic failure.
  - role: builder
  - files: internal/mcp/tools.go, internal/mcp/server.go, internal/mcp/tools_test.go
  - contract: Preserve the existing lock error path but add a machine-readable `locked` status field to tool results when a spec lock is unavailable.
  - acceptance: Tool callers can detect lock contention without parsing free-form text.
  - verify: go test ./internal/mcp -run 'Test.*Lock|Test.*Tool' -count=1
  - depends: T1
  - requirements: 4

## Wave 3
- [x] T4 — Prove multi-session behavior with concurrency tests
  - why: Requirement 5 needs regression coverage for overlapping watcher sessions and same-spec contention.
  - role: builder
  - files: internal/mcp/watcher_test.go, internal/mcp/integration_test.go, internal/core/lock_test.go
  - contract: Add deterministic tests for two concurrent MCP sessions on different specs and for a second session blocked on the same spec.
  - acceptance: Tests fail if one session can see another session's spec or if lock contention is not surfaced as structured blocking.
  - verify: go test ./internal/mcp ./internal/core -count=1
  - depends: T2, T3
  - requirements: 2, 3, 4, 5
