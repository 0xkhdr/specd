# Spec — Single-Flight MCP Server Documentation (A9)

**Priority:** P2 · **Wave:** 3 · **Domain:** API-contract clarity.

## Introduction

`transport_http.go` wraps dispatch in a single process-wide `sync.Mutex`
(`internal/mcp/transport_http.go:61`), serializing **all** `/rpc` and `/sse`
handling to one in-flight request. This is likely intentional — it preserves the
determinism invariant and matches the single-agent local-first model — but it is
an undocumented throughput ceiling that could be mistaken for a bug or
load-tested as if concurrent.

This spec documents the single-flight design in code and in `mcp-guide.md` so the
contract is explicit.

## Current-state grounding

- `internal/mcp/transport_http.go:61` — `var mu sync.Mutex`; `:63` `mu.Lock()`
  around dispatch.
- Determinism invariant: orchestration decisions pure; single-flight preserves
  ordering.
- `docs/mcp-guide.md` — MCP transport docs; no concurrency note today.

## Requirements

### Requirement 1 — Code comment on the single-flight mutex
**User story:** As a contributor, I want the mutex's intent documented at the
call site, so I do not mistake it for a missing-optimization bug.

**Acceptance criteria:**
1. A comment at `transport_http.go` mutex SHALL state the server is intentionally
   single-flight to preserve determinism / single-agent model.
2. The comment SHALL note it is a deliberate throughput ceiling, not a bug.

### Requirement 2 — Document in mcp-guide
**User story:** As an MCP host author, I want the throughput contract stated, so
I do not load-test it as concurrent.

**Acceptance criteria:**
1. `docs/mcp-guide.md` SHALL state the server processes one in-flight request at
   a time across `/rpc` and `/sse`.
2. The doc SHALL state the rationale (determinism, local-first single-agent).

## Design

- Add the explanatory comment at the mutex declaration / lock site.
- Add a short "Concurrency model" subsection to `mcp-guide.md`.
- No behavior change.

## Out of scope

- Making the server concurrent (would threaten the determinism invariant).
- Per-method locking.

## Risks

- None (documentation-only).
