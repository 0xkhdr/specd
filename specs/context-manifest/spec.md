# Context Manifest Scope and Budget Spec

## Purpose
Make task context manifests precise, bounded, and explainable so agents receive enough knowledge without unnecessary token load or scope creep.

## Source Gaps
- GAP-ANALYSIS.md domain 6: context manifest scope and budget drift.
- Manifest may include broad steering/memory content not tied to task files.
- Context budget gate does not always surface why budget passed or failed.
- Task file scope and citations need stronger contract.

## Goals
- Build context from task files, dependencies, relevant steering, and cited docs only.
- Report token/byte budget estimates and truncation decisions.
- Keep manifest deterministic and stable across runs.
- Add tests for task-scoped inclusion and exclusion.

## Non-Goals
- Do not add semantic embedding or LLM ranking.
- Do not make context builder mutate state.
- Do not remove required steering files when phase requires them.

## Required Knowledge
- Context builder: `internal/context/manifest.go`.
- Context budget gate: `internal/core/gates/contextbudget.go`.
- Tasks parser: `internal/core/tasksparser.go`.
- Steering and memory files under `.specd/`.

## Functional Contract
- Manifest must cite why each included file appears: task file, dependency artifact, steering rule, role instruction, or required doc.
- Files outside task scope require explicit reason.
- Budget gate reports total bytes, estimated tokens, max budget, and largest contributors.
- Truncation preserves citation metadata and marks content truncated.

## Acceptance Criteria
- Tests prove unrelated files are excluded.
- Tests prove task-declared files and role files are included.
- Budget diagnostics are machine-readable and human-readable.
- Manifest ordering is deterministic.

## Invariants
- No LLM in context selection.
- No filesystem writes during context build.
- No hidden network access.
- Byte-stable task parsing remains intact.

## Verification
- `go test ./internal/context ./internal/core/gates ./internal/core -count=1`
- `go test ./... -count=2`

