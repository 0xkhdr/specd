# Tasks — Context Manifest Zero-Overhead Alignment

## Wave 1 — Manifest item shape
- [ ] T1 — Add selector and budget fields
  - why: hosts need precise slice and overflow behavior (Req 4,5)
  - role: builder
  - files: internal/context/manifest_types.go, internal/cmd/context.go
  - contract: add optional `selector`, `overBudget`, and `budgetActions` fields to manifest JSON without removing existing fields.
  - acceptance: old tests still pass; new fields appear only when populated.
  - verify: go test ./internal/context/ ./internal/cmd/ -run Context
  - depends: —
  - requirements: 4,5

- [ ] T2 — Emit targeted slice selectors
  - why: eliminate prose parsing for `read-targeted` (Req 5)
  - role: builder
  - files: internal/context/manifest.go, internal/context/slice.go, internal/context/manifest_test.go
  - contract: populate selector for task row, covered requirements, design headings, and memory window when available.
  - acceptance: tests assert selector contents for each slice type.
  - verify: go test ./internal/context/ -run "Manifest|Slice"
  - depends: T1
  - requirements: 5

## Wave 2 — Mode-aware required items
- [ ] T3 — Remove unconditional Pinky skill from briefing
  - why: base-mode context must not pay Pinky overhead (Req 1)
  - role: builder
  - files: internal/context/manifest.go, internal/context/manifest_test.go, internal/cmd/context_manifest_cmd_test.go
  - contract: include Pinky skill only for mission mode or explicit Pinky/orchestrated worker context; briefing gets role + phase skill.
  - acceptance: base `specd context --json` has no Pinky skill; Brain/Pinky missions still include it.
  - verify: go test ./internal/context/ ./internal/cmd/ -run Context
  - depends: T1
  - requirements: 1

- [ ] T4 — Add steering constitution items
  - why: manifest must be the load oracle (Req 2)
  - role: builder
  - files: internal/context/manifest.go, internal/cmd/context.go
  - contract: add reasoning/workflow/product/tech/structure steering items with phase-appropriate modes; include memory only execute/verify/reflect; keep legacy `load` compatible.
  - acceptance: context JSON manifest includes all required steering; memory absent during requirements/design/tasks unless requested.
  - verify: go test ./internal/cmd/ -run "Context|BuildBrief"
  - depends: T3
  - requirements: 2

## Wave 3 — Fusion-aware budget actions
- [ ] T5 — Add fusion policy/help run-command references
  - why: config and schema awareness without repeated bloat (Req 3)
  - role: builder
  - files: internal/context/manifest.go, internal/cmd/context.go
  - contract: include `specd fusion policy <slug> --json` when fusion is registered; include `specd help --json` reference/run-command only when no schema digest hint is present.
  - acceptance: manifest contains commands as `run-command` items with rationales; estimated tokens remain bounded.
  - verify: go test ./internal/context/ ./internal/cmd/ -run Context
  - depends: T4, session-bootstrap/T3
  - requirements: 3

- [ ] T6 — Over-budget recommendations
  - why: agents need deterministic overflow handling (Req 4)
  - role: builder
  - files: internal/context/manifest.go, internal/cmd/context.go, internal/cmd/check.go
  - contract: compute `overBudget`; emit action strings for base vs orchestrated/session-known cases; ensure context-budget gate consumes same estimate.
  - acceptance: over-budget fixture reports actions; check gate names heaviest items and agrees with manifest.
  - verify: go test ./internal/context/ ./internal/cmd/ -run "Budget|Context"
  - depends: T1
  - requirements: 4

- [ ] T7 — Docs update for load oracle
  - why: agents must trust manifest over ad-hoc loading (Req 1,2,3,4,5)
  - role: builder
  - files: docs/agent-integration.md, internal/core/embed_templates/AGENTS.md
  - contract: document mode-aware Pinky loading, selectors, over-budget actions, and fusion policy references.
  - acceptance: docs state `contextManifest` is exclusive load list after bootstrap.
  - verify: N/A
  - depends: T6
  - requirements: 1,2,3,4,5
