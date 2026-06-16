---
name: specd-tasks
description: Author tasks.md as a wave DAG for a specd spec. Load when entering the tasks (PLAN) phase. Covers the seven mandatory task keys, waves, the deps-in-earlier-or-equal-wave rule, acyclicity, requirement traceability, and the `task-schema` + `dag` gates `specd check` enforces.
---

# specd tasks

Phase PLAN (tasks): decompose the approved design into an ordered wave DAG. Load
`.specd/specs/<slug>/design.md`. Run `specd context <slug>` for the briefing, then
`specd waves <slug>` to inspect the graph you author.

## The seven mandatory keys (every task)

Each task is a checkbox item carrying all seven keys, or it is not a task:

1. `why` — the reason this task exists (the requirement/design need it serves).
2. `role` — `investigator` | `builder` | `reviewer` | `verifier`.
3. `files` — the concrete files it will touch.
4. `contract` — exactly what it must do.
5. `acceptance` — the observable done-condition.
6. `verify` — the deterministic command that proves it (`verify: N/A` is allowed
   **only** for read-only roles: investigator, reviewer).
7. `depends` — task IDs it depends on (`—` if none).

Also tag `requirements:` with the requirement numbers each task covers — every
requirement must be referenced by at least one task (traceability), and every
referenced number must exist in `requirements.md`.

## Waves and the DAG

- Tasks are grouped into **waves** = dependency batches that can run concurrently.
- A task's `depends` must point to tasks in an **earlier-or-equal** wave. A
  later-wave dependency fails the gate.
- The graph must be **acyclic** — no dependency cycles.
- `depends` must reference task IDs that exist.

## The gates `specd check` enforces here

- `task-schema` — every task has the seven keys; `verify` rules respected; at least
  one task defined.
- `dag` — no missing deps, no cycles, no later-wave dependency.
- `traceability` — requirements ↔ tasks both directions (severity configurable in
  `config.json`; defaults to warn).

## Exit and advance

```
specd check <slug>      # task-schema + dag gates green (exit 0)
specd waves <slug>      # eyeball the wave DAG / critical path
specd approve <slug>    # human approves tasks → advances to executing
```

Then load `specd-execute` when the approve advances you into the executing phase.
