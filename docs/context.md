# Context Engineering — the manifest engine contract

> Context engineering is "the real skill" (paper pp. 15–18): progressive disclosure of the
> minimum bytes a task needs, never the whole repo. specd assembles a **bounded context
> manifest** deterministically — no LLM in the path (P1). This documents the engine contract:
> the four item modes, the budget, and the heuristic estimator. Code: `internal/context/`.

## Where it is surfaced

One engine, two surfaces (they share `context.BuildManifest`, so output is identical):

- `specd context <slug> <task-id> [--json|--hud]` — the CLI manifest for a task.
- the MCP `context` tool — the same manifest for any MCP client.

## The four item modes (disclosure contract)

A manifest is a small, ordered set of items, each declaring **how much** of its source to
disclose. The contract defines four disclosure modes, cheapest first:

1. **reference** — name the path only; the agent reads it if and only if needed.
2. **read-summary** — a computed digest (heading/first-line level), not the full body.
3. **read-targeted** — only the slice that matters (e.g. the single task row, not all of `tasks.md`).
4. **read-full** — the whole file, used only when nothing smaller is faithful (e.g. the role prompt).

The shipped manifest (`context.Manifest`) realizes this as exactly **four items** per task —
`spec`, `tasks`, `task`, `role` — where `task` is the **read-targeted** slice (one row by ID) and
`role` is **read-full**. Item order is stable (sorted by kind/path/task-id) so the manifest is
byte-reproducible. The per-manifest `mode` field is derived from the task's role
(`craftsman|validator|scout`), selecting the disclosure profile for that worker.

## Budget

- Config key `context.max_tokens` (env `SPECD_CONTEXT_MAX_TOKENS`), **default 12000**, must be a
  positive integer or config validation fails.
- The context-budget gate (`internal/core/gates/contextbudget.go`, opt-in) compares the manifest's
  `estimated_tokens` against the budget and blocks when a task's context would exceed it —
  forcing the disclosure ladder down before more is loaded.

> Note: progress.md's T8.5 names `SPECD_MAX_CONTEXT_TOKENS`; the implemented name is
> `SPECD_CONTEXT_MAX_TOKENS`. The reconciliation is review-specs W5 (F10), not this doc — the
> engine's true contract is the one above.

## The heuristic estimator

Token cost is estimated without a tokenizer or any model call:

```
estimated_tokens(text) = ceil(len/4)   // ceil(len(text)/4), implemented as (len+3)/4
```

`internal/context/estimate.go` (`EstimateText` / `EstimateNoLLM`). Every item's
`estimated_tokens` is summed into the manifest total. The estimate is deterministic and
LLM-free by construction (P1, P7) — the same input always yields the same number, so the
budget gate is reproducible in CI.
