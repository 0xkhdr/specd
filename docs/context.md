# Context Engineering — the manifest engine contract

> Context engineering is "the real skill" (paper pp. 15–18): progressive disclosure of the
> minimum bytes a task needs, never the whole repo. specd assembles a **bounded context
> manifest** deterministically — no LLM in the path (P1). This documents the engine contract:
> the four item kinds, item modes, the budget, the heuristic estimator, and the manifest
> schema. Code: `internal/context/manifest.go`.

---

## Where it is surfaced

One engine, three surfaces (they all call `context.BuildManifest`, so output is identical):

- `specd context <slug> <task-id> [--json|--hud]` — the CLI manifest for a task.
- `specd next <slug> --dispatch` — the manifest for the first frontier task (for orchestration).
- The MCP `context` tool — the same manifest for any MCP client.

**Parity guarantee:** the same input state always produces byte-identical output across
all three surfaces.

---

## The four core item kinds

Every manifest contains exactly **four core items** plus optional steering/memory items:

| Kind | Description | Never dropped? |
|---|---|---|
| `spec` | Path to `requirements.md` | ✅ always included |
| `tasks` | Path to `tasks.md` | ✅ always included |
| `task` | The single task row by ID | ✅ always included |
| `role` | Path to the role-prompt file | ✅ always included |
| `steering` | Constitution files (`.specd/steering/*.md`) | Dropped last |
| `memory` | Memory files (`steering/memory.md`, spec `memory.md`) | Dropped first |

Core items (spec/tasks/task/role) are **never dropped** by the budget enforcer, even
when the total exceeds `max_tokens`. Steering drops before memory is exhausted; memory
drops first.

---

## Item modes

Each item carries a `mode` field that tells the agent **how much** of the source to
disclose. The contract defines these modes, cheapest first:

1. **`reference`** — name the path only; the agent reads it if and only if needed.
2. **`read-summary`** — a computed digest (heading/first-line level), not the full body.
3. **`read-targeted`** — only the slice that matters (e.g. the single task row, not all of `tasks.md`).
4. **`read-full`** — the whole file, used only when nothing smaller is faithful (e.g. the role prompt).

Additional modes used by steering items:
- **`static-instructions`** — steering constitution files; loaded as persistent context.
- **`reference-if-needed`** — memory files; referenced but not pre-loaded.

---

## Budget enforcement

- Config key `context.max_tokens` (env `SPECD_CONTEXT_MAX_TOKENS`), **default 12000**.
- Must be a positive integer; zero or negative **disables** budget enforcement.
- When over budget: memory items drop first (last-in-first-out order), then steering.
- Core items (spec/tasks/task/role) are never dropped.
- Each dropped item appends a note to `manifest.notes` for observability.
- The optional **context-budget gate** (`internal/core/gates/contextbudget.go`, opt-in)
  turns "context stuffed with noise" into an enforceable gate failure — advisory by
  default, enforced by choice.

---

## The heuristic estimator

Token cost is estimated without a tokenizer or any model call:

```
estimated_tokens(text) = ceil(len(text) / 4)   // implemented as (len+3)/4
```

`internal/context/estimate.go` (`EstimateText` / `EstimateNoLLM`). Every item's
`estimated_tokens` is summed into the manifest total. The estimate is deterministic and
LLM-free by construction (P1, P7) — the same input always yields the same number, so the
budget gate is reproducible in CI.

For file-backed items, the estimate uses the on-disk file size: `ceil(file_size / 4)`.

---

## Manifest schema

The manifest is versioned JSON (`version: "1"`). Validation requires `mode`, `slug`,
and `task_id` to be present, and at least four items.

```json
{
  "version": "1",
  "mode": "craftsman",
  "slug": "payment-service",
  "task_id": "T1",
  "estimated_tokens": 312,
  "items": [
    {
      "kind": "memory",
      "path": ".specd/specs/payment-service/memory.md",
      "mode": "reference-if-needed",
      "estimated_tokens": 8
    },
    {
      "kind": "role",
      "path": ".specd/roles/craftsman.md",
      "estimated_tokens": 22
    },
    {
      "kind": "spec",
      "path": "specs/payment-service/requirements.md",
      "estimated_tokens": 21
    },
    {
      "kind": "steering",
      "path": ".specd/steering/memory.md",
      "mode": "reference-if-needed",
      "estimated_tokens": 6
    },
    {
      "kind": "steering",
      "path": ".specd/steering/product.md",
      "mode": "static-instructions",
      "estimated_tokens": 8
    },
    {
      "kind": "steering",
      "path": ".specd/steering/reasoning.md",
      "mode": "static-instructions",
      "estimated_tokens": 7
    },
    {
      "kind": "steering",
      "path": ".specd/steering/structure.md",
      "mode": "static-instructions",
      "estimated_tokens": 8
    },
    {
      "kind": "steering",
      "path": ".specd/steering/tech.md",
      "mode": "static-instructions",
      "estimated_tokens": 7
    },
    {
      "kind": "steering",
      "path": ".specd/steering/workflow.md",
      "mode": "static-instructions",
      "estimated_tokens": 8
    },
    {
      "kind": "task",
      "task_id": "T1",
      "estimated_tokens": 4
    },
    {
      "kind": "tasks",
      "path": "specs/payment-service/tasks.md",
      "estimated_tokens": 21
    }
  ],
  "notes": null
}
```

Items are **sorted stably** by `(kind, path, task_id)` so the manifest is byte-reproducible.

---

## `mode` field

The manifest-level `mode` field is derived from the task's `role:`:

| Task role | Manifest mode |
|---|---|
| `craftsman` | `"craftsman"` |
| `validator` | `"validator"` |
| `scout` | `"scout"` |
| *(other)* | `"craftsman"` (default) |

This selects the disclosure profile for the assigned worker.

---

## HUD rendering

`specd context <slug> <task-id> --hud` renders an operator-readable summary:

```
context manifest  payment-service / T1
mode              craftsman
items             11
estimated tokens  312 / 12000
spec              specs/payment-service/requirements.md
tasks             specs/payment-service/tasks.md
task              T1
role              .specd/roles/craftsman.md
```

---

## Known env var note

> `context.md` previously named `SPECD_MAX_CONTEXT_TOKENS` as the environment variable.
> The correct name is `SPECD_CONTEXT_MAX_TOKENS` (as implemented in
> `internal/core/config_validate.go`). The reconciliation is tracked as open finding F10.
