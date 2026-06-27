# Spec — Context Manifest Zero-Overhead Alignment

**Priority:** P1 · **Wave:** 2 · **Domain:** phase-scoped context loading.

## Introduction

The fusion analysis makes `specd context <slug> --json` the load oracle. The current code has a shared manifest engine for `context`, `dispatch`, and Pinky missions with token estimates and targeted slices. However, its item composition is still mission-biased: it always includes the Pinky skill, does not include config/help bootstrap data, and briefing mode does not fully reflect the always-on steering constitution. This spec aligns the manifest engine with the fusion model while preserving the efficient shared engine.

## Current-state grounding

- `internal/context/manifest.go` builds `MissionContextManifest` for briefing, dispatch, and mission modes.
- `internal/cmd/context.go` adds `baseSteering` (`reasoning`, `workflow`) plus phase-specific source files in human/json output.
- Targeted slicing exists for tasks, requirements, and design sections.
- `docs/agent-integration.md` documents `contextManifest` as authoritative and the context-budget gate.
- `brain compact/clear/ledger` exists for orchestration compaction.

## Requirements

### Requirement 1 — Mode-aware required items
**User story:** As a base-mode agent, I do not want Pinky-specific context unless I am operating as a Pinky worker.

**Acceptance criteria:**
1. Briefing mode SHALL NOT include `.specd/skills/specd-pinky/SKILL.md` unless the active role or mode requires Pinky.
2. Mission mode SHALL include Pinky lifecycle skill.
3. Dispatch mode SHALL include role + phase skill but SHALL NOT force Pinky skill unless dispatching for orchestrated worker execution.

### Requirement 2 — Full steering constitution in manifest
**User story:** As an agent, I want the manifest to identify all always-on steering files without relying on memory.

**Acceptance criteria:**
1. Briefing manifests SHALL include `reasoning`, `workflow`, `product`, `tech`, and `structure` steering items with `read-full` or phase-appropriate `reference-if-needed` modes.
2. `memory.md` SHALL be included only for executing, verifying, or reflecting phases unless explicitly requested.
3. The old top-level `load` field in `specd context --json` SHALL remain for compatibility but SHOULD mirror the manifest's required load set.

### Requirement 3 — Bootstrap references without bloat
**User story:** As an agent, I need config and schema awareness but not repeated huge schema bodies.

**Acceptance criteria:**
1. Briefing manifests SHALL include a required `run-command` item for `specd fusion policy <slug> --json` when the fusion command is available.
2. They SHALL include a `run-command` or reference item for `specd help --json` only when no cached schema digest is supplied by the host.
3. Manifest output SHALL keep `estimatedTokens` based on required items and SHALL avoid inlining full help JSON.

### Requirement 4 — Budget overflow action
**User story:** As an agent, I want deterministic behavior when required context exceeds budget.

**Acceptance criteria:**
1. When `estimatedTokens > budget`, context JSON SHALL include `overBudget: true` and `budgetActions`.
2. For orchestrated sessions, recommended actions SHALL include `specd brain compact` with session when known.
3. For base mode, recommended actions SHALL include targeted slicing and asking the user before broad loading.
4. The existing context-budget gate SHALL use the same calculation.

### Requirement 5 — Targeted slice metadata
**User story:** As a host, I want to load a slice precisely, not infer it from rationale prose.

**Acceptance criteria:**
1. Manifest items in `read-targeted` mode SHALL include a `selector` object where possible (`taskID`, `requirements`, `designHeadings`, or line range).
2. Selectors SHALL be deterministic and optional for backward compatibility.
3. Tests SHALL verify task, requirement, design, and memory slice selectors.

## Design

- Extend `MissionContextItem` with optional `Selector`, `OverBudget`, and `BudgetActions` fields.
- Make `BuildContextManifest` branch on `ContextMode` for Pinky skill inclusion.
- Add steering items through the shared engine rather than only `cmd/context.go`'s parallel `load` array.
- Allow `ContextRequest` to carry cached schema/config digest hints later; initial implementation may emit stable run-command items.
- Keep old fields additive and avoid breaking existing JSON consumers.

## Out of scope

- Changing the actual model context of any host.
- Summarizing source files with an LLM.
- Removing legacy `load` from `specd context`.

## Risks

- **Manifest churn:** Additive fields and compatibility tests keep existing consumers working.
- **Under-loading:** Required steering remains explicit; phase/source artifacts remain governed by current status.
