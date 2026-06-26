# Stage 3 — Multi-Session Concurrency

## Goal
Make each MCP process session-aware so concurrent sessions do not fight over a single global active spec.

## Knowledge gathered
- `internal/mcp/watcher.go:activeSpec()` currently scans all specs and picks the most active one by status rank, then slug order.
- `internal/mcp/server.go` starts the phase watcher without any spec affinity.
- `internal/cli/args.go` does not yet expose a `--spec` flag for the `mcp` subcommand.
- `internal/core/lock.go` already serializes same-spec mutations with an advisory lock.
- TASKS.md says same-spec lock failures should expose a structured `locked` status in tool results.

## Frozen scope
- Add optional per-process spec affinity.
- Keep backward-compatible fallback behavior when no spec is pinned.
- Expose structured lock status so hosts can warn cleanly.
- Add concurrency tests for overlapping sessions.

## Requirements
1. `specd mcp` must accept an optional `--spec <slug>` pin.
2. When pinned, active-spec resolution must ignore all other specs.
3. When unpinned, the current status-rank fallback must remain.
4. Lock failures must expose structured `locked` status, not only raw error text.
5. Multi-session tests must cover different specs and same-spec contention.

## Non-goals
- No change to the underlying spec lock implementation.
- No change to worker subprocess semantics.
- No new orchestration policy.

## Implementation constraints
- Keep the no-flag behavior backward compatible.
- Thread the pinned slug through server startup and the watcher.
- Preserve deterministic status ordering when unpinned.
- Do not weaken lock enforcement.

## Done criteria
- Two MCP sessions can track different specs without cross-talk.
- A pinned session only sees its own spec's status and tools.
- Same-spec contention reports a structured lock condition.
