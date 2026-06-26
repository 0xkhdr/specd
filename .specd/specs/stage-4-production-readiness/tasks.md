# Tasks — Stage 4 — Production Readiness

## Wave 1
- [ ] T1 — Audit observability, server error handling, and release gaps
  - why: We need the exact file-level surface before adding production hardening.
  - role: investigator
  - files: internal/obs/log.go, internal/mcp/server.go, internal/integration/conformance_test.go, internal/mcp/watcher.go, internal/context/manifest.go, .goreleaser.yml, SECURITY.md
  - contract: Map the current logging fields, missing-root error paths, adapter conformance coverage, context manifest flow, and release metadata gaps.
  - acceptance: A precise gap list exists for every Stage 4 item.
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 3, 4, 5

## Wave 2
- [ ] T2 — Add structured logging and JSON log format support
  - why: Requirement 1 needs production logs with stable, machine-readable fields.
  - role: builder
  - files: internal/obs/log.go, internal/obs/log_test.go, internal/mcp/server.go
  - contract: Emit structured log events with slug, session-id, phase, role, and timing; add the JSON log format path behind the documented opt-in flag/env.
  - acceptance: Log events include the required fields and JSON output is deterministic.
  - verify: go test ./internal/obs ./internal/mcp -count=1
  - depends: T1
  - requirements: 1

- [ ] T3 — Harden MCP missing-root and corrupt-root handling
  - why: Requirement 2 needs protocol-safe failure paths instead of panics or silent drops.
  - role: builder
  - files: internal/mcp/server.go, internal/mcp/server_test.go, internal/mcp/resources.go
  - contract: Audit nil-deref and missing-root paths, return structured JSON-RPC errors, and keep the server responsive after malformed input.
  - acceptance: The server continues to answer requests after a missing/corrupt root error.
  - verify: go test ./internal/mcp -run 'Test.*Server|Test.*Resources' -count=1
  - depends: T1
  - requirements: 2

## Wave 3
- [ ] T4 — Extend adapter conformance to all registry entries
  - why: Requirement 3 needs a regression guard that covers every adapter, not just a subset.
  - role: builder
  - files: internal/integration/conformance_test.go, internal/integration/registry.go, internal/integration/*_test.go
  - contract: Make the idempotency check table-driven across the entire default registry and verify the second install reports no change.
  - acceptance: Every adapter in the registry participates in the idempotency regression test.
  - verify: go test ./internal/integration -count=1
  - depends: T1
  - requirements: 3

- [ ] T5 — Scope context manifests to the active spec and finish release checks
  - why: Requirements 4 and 5 need the remaining production-readiness items closed out.
  - role: builder
  - files: internal/mcp/watcher.go, internal/context/manifest.go, internal/context/manifest_types.go, .goreleaser.yml, SECURITY.md, internal/core/commands.go
  - contract: Filter the context manifest to the pinned spec, verify release attestation/checksum settings, keep SECURITY.md contact current, and add machine-readable version output support if missing.
  - acceptance: Context delivery is spec-scoped and the release/version metadata is complete enough for CI.
  - verify: go test ./internal/context ./internal/mcp ./internal/core -count=1
  - depends: T2, T3
  - requirements: 4, 5
