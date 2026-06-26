# Tasks — Stage 1 — Host Integration Parity

## Wave 1
- [x] T1 — Audit current host integration surface
  - why: Establish the exact Gemini→Antigravity deltas before editing code.
  - role: investigator
  - files: internal/integration/gemini.go, internal/integration/gemini_test.go, internal/integration/registry.go, internal/integration/hostutil.go, internal/integration/conformance_test.go, README.md, AGENTS.md
  - contract: Record the exact adapter methods, config paths, install helpers, and doc references that must change; do not modify files.
  - acceptance: A complete delta map exists for detection, planning, install, inspect, verify, tests, and docs.
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 3, 5, 6

## Wave 2
- [x] T2 — Replace Gemini adapter with Antigravity adapter
  - why: Requirement 1 and 2 require a new adapter implementation and registry swap.
  - role: builder
  - files: internal/integration/gemini.go, internal/integration/gemini_test.go, internal/integration/antigravity.go, internal/integration/antigravity_test.go, internal/integration/registry.go
  - contract: Delete Gemini registration, add Antigravity detection/planning/install/inspect/verify, and point DefaultRegistry() at the new adapter; preserve the project-only scope and direct JSON write model.
  - acceptance: The registry exposes Antigravity, the adapter plans against `.agents/mcp_config.json`, and tests prove idempotent preservation of unrelated JSON keys.
  - verify: go test ./internal/integration -count=1
  - depends: T1
  - requirements: 1, 2, 3, 4, 5

- [x] T3 — Update conformance and documentation for the new host
  - why: Requirement 5 and 6 require the repo-level host matrix to match the new registry and file layout.
  - role: builder
  - files: internal/integration/conformance_test.go, README.md, AGENTS.md
  - contract: Remove Gemini-specific assertions, add Antigravity expectations, and document that `.agents/` is committed as part of the project host config.
  - acceptance: Conformance tests reflect the new registry set and the docs mention the new config path and tracking rule.
  - verify: go test ./internal/integration -run 'TestCompatibilityMatrixMatchesProjectAdapters|TestDefaultAdapterConformance' -count=1
  - depends: T2
  - requirements: 1, 5, 6
