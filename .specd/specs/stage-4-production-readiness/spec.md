# Stage 4 — Production Readiness

## Goal
Harden the system for real-world use: observability, graceful failures, broader conformance, spec-scoped context, and release completeness.

## Knowledge gathered
- `internal/obs/log.go` is currently minimal and needs richer structured fields.
- `internal/mcp/server.go` must keep serving JSON-RPC even when the project root is missing or corrupt.
- `internal/integration/conformance_test.go` should enforce idempotency across all adapters, not just a subset.
- `internal/mcp/watcher.go` and the context-manifest filter currently operate on the active spec, but the manifest scoping is still broader than it should be after pinning.
- `.goreleaser.yml`, `SECURITY.md`, and version reporting are called out in TASKS.md as release-readiness gaps.

## Frozen scope
- Add structured logging and JSON log formatting.
- Harden MCP error handling for missing/corrupt `.specd/` roots.
- Enforce install idempotency for every adapter.
- Scope context-manifest delivery to the active spec.
- Verify release artifact completeness and machine-readable version output.

## Requirements
1. Structured logs must include slug, session-id, phase, role, and event timing.
2. MCP must return structured errors instead of panicking when the spec root is missing or corrupt.
3. Conformance tests must cover idempotent second installs for every registry adapter.
4. Context manifests must be filtered to the pinned active spec.
5. Release config and version output must be suitable for CI and release automation.

## Non-goals
- No change to the core spec workflow.
- No new orchestration features.
- No silent downgrade of errors to success.

## Implementation constraints
- Keep logging opt-in through the documented flag/env path.
- Fail closed on malformed project state.
- Keep the adapter conformance test table-driven.
- Avoid introducing nondeterministic log ordering.

## Done criteria
- Logs are structured and useful for debugging.
- Missing or corrupt project state yields a protocol-safe error.
- Every adapter is covered by the idempotency regression guard.
- Context delivery is scoped to the active spec only.
- Release metadata is complete enough for automated publishing.
