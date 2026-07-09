# Spec Progress

This file is the wave dispatch board for gap-closure specs derived from `GAP-ANALYSIS.md`.

## Status Legend
- `todo`: not started.
- `doing`: current implementation wave.
- `done`: completed with passing verify evidence.
- `blocked`: cannot proceed; record blocker in Notes.

## Wave Board

| wave | status | specs | intent | dispatch rule |
|---:|---|---|---|---|
| 1 | done | `agent-workflow-mcp`, `init-host-scaffold`, `cli-contracts`, `evidence-security`, `orchestration-workers`, `context-manifest`, `project-diagnostics-config`, `concurrency-isolation` | Build foundations and close P0 safety/truth gaps. | Completed all `wave=1` no-deps tasks. |
| 2 | done | same | Integrate docs, scaffold, diagnostics, and cross-path behavior. | Verified ESG-T3/T4 and OWR-T3. |
| 3 | todo | same | Validate whole repo, race tests, flake checks, docs lint. | Dispatch after wave 2 tasks in same spec are `done`. |

## Current Recommended Dispatch

Start with wave 1 in this order:

1. `evidence-security` wave 1
2. `orchestration-workers` wave 1
3. `agent-workflow-mcp` wave 1
4. `cli-contracts` wave 1
5. `init-host-scaffold` wave 1
6. `context-manifest` wave 1
7. `project-diagnostics-config` wave 1
8. `concurrency-isolation` wave 1

Reason: evidence and worker rigor protect completion semantics before broader UX/docs changes.

## Completion Rules

- Mark task `done` only after its `verify` command exits 0 and output is recorded in implementation notes or commit/PR body.
- Mark wave `done` only when every task in that wave for selected spec is `done`.
- If verify fails, do not mark done. Record exact failing command and first failing test.
- If task requires files beyond `files`, record scope expansion in notes before editing.
- Never edit `reference/`.

## Notes

Add dated notes below as waves are dispatched.

- 2026-07-09: Wave 1 complete. Verified AWM-T1/T2, IHS-T1/T2, CCH-T1/T2, ESG-T1/T2, OWR-T1/T2, CMS-T1/T2, PDC-T1/T2, and CWI-T1/T2 with their declared commands.
- 2026-07-09: Wave 2 complete. Verified ESG-T3/T4 and OWR-T3 with their declared commands.
- 2026-07-09: Wave 3 complete. Verified PDC-T5 with `go test ./... -count=1`.
