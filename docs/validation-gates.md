# 6. Validation Gates

`specd check <slug>` runs **seven** deterministic gates over a spec's artifacts. It is the
correctness backstop the planning ratchet leans on: `specd approve` will not advance a phase whose
gate is not green. Each gate is pure file inspection — no model, no network, fully reproducible.

```
$ specd check my-feature
✓ check passed — all gates green for 'my-feature'
```

Violations print as `fail  <location>: <message> (<gate>)` and set exit code `1`. Warnings print as
`warn  …` and do **not** fail the check. `--json` emits `{ ok, violations[], warnings[] }`.

| # | Gate | Severity | Source | Checks |
|---|------|----------|--------|--------|
| 1 | EARS | fail | `src/core/ears.ts` | requirements well-formed |
| 2 | Design | fail | `src/core/phases.ts` | design sections present, non-empty, no TODO |
| 3 | Task-schema | fail | `src/commands/check.ts` | seven keys, valid role, real verify |
| 4 | DAG | fail | `src/core/dag.ts` | acyclic, no orphans, wave order |
| 5 | Evidence | fail | `src/commands/check.ts` | no complete-without-evidence |
| 6 | Sync | fail | `src/commands/check.ts` | checkboxes ↔ state agree |
| 7 | Traceability | warn + fail | `src/commands/check.ts` | requirement ↔ task references |

---

## Gate 1 — EARS

Validates `requirements.md`. For every `## Requirement N` block it requires:

- A `**User story:**` line.
- At least one numbered acceptance criterion (under `**Acceptance criteria:**`).
- Every criterion matches one of the five EARS patterns (case-insensitive):

| Pattern | Grammar |
|---------|---------|
| Ubiquitous | `THE SYSTEM SHALL <response>` |
| Event-driven | `WHEN <trigger> THE SYSTEM SHALL <response>` |
| State-driven | `WHILE <state> THE SYSTEM SHALL <response>` |
| Optional-feature | `WHERE <feature> THE SYSTEM SHALL <response>` |
| Unwanted | `IF <condition> THEN THE SYSTEM SHALL <response>` |

It also fails if the file is missing, or has no `## Requirement N` sections at all.

```
fail  requirements.md:12: criterion does not match any EARS pattern: "the loader should be fast" (ears)
fail  requirements.md:3: requirement "Config loading" missing **User story:** line (ears)
```

---

## Gate 2 — Design

Validates `design.md`. Requires **all seven** mandatory H2 sections, each present, non-empty, and
free of any `TODO` marker:

```
## Overview
## Architecture
## Components and interfaces
## Data models
## Error handling
## Verification strategy
## Risks and open questions
```

```
fail  design.md: missing section: ## Data models (design)
fail  design.md:30: section 'Error handling' still contains a TODO marker (design)
```

---

## Gate 3 — Task-schema

Validates the structure of every task in `tasks.md`. The parser first enforces the **seven mandatory
keys** (`why, role, files, contract, acceptance, verify, depends`) — a missing key is a parse error
with a line number. Then the gate checks:

- **Valid role** — `role` ∈ `{investigator, builder, reviewer, verifier}`.
- **Real verify command** — `verify` may be `N/A` (or empty) **only** for read-only roles
  (`investigator`, `reviewer`). A write/run role with `verify: N/A` fails.
- **At least one task** exists.

```
fail  tasks.md:14: T3: invalid role 'maker' (task-schema)
fail  tasks.md:22: T4: verify N/A only allowed for read-only roles (got 'builder') (task-schema)
fail  tasks.md: no tasks defined (task-schema)
```

---

## Gate 4 — DAG

Validates the dependency graph built from each task's `depends` and `wave`:

- **No orphan deps** — every `depends` id must reference an existing task.
- **Acyclic** — no dependency cycle (reported as the cycle path).
- **Wave order** — a task's dependency must live in an *earlier-or-equal* wave (you cannot depend on
  a later wave).

```
fail  tasks.md: T2 depends on missing task 'T9' (dag)
fail  tasks.md: dependency cycle: T3 → T4 → T3 (dag)
fail  tasks.md: T1 depends on 'T5' which is in a later wave (dag)
```

---

## Gate 5 — Evidence

Validates `state.json`: **no task may be `complete` with empty or missing evidence.** In normal use
this is impossible to violate because `specd task --status complete` already requires `--evidence` —
this gate is the durable backstop that catches any out-of-band tampering with `state.json`.

```
fail  state.json: T1: complete without evidence (evidence)
```

---

## Gate 6 — Sync

Validates that the two truths agree. For every task, the `tasks.md` checkbox must match the
`state.json` status, and a `blocked` annotation must match a `blocked` state:

- checkbox `[x]` ⟺ `status == complete`.
- `blocked` annotation ⟺ `status == blocked`.

This is the gate that catches hand-edits to either file. The CLI's dual-write keeps them in sync, and
`reconcile()` repairs structural drift on load, but a manual edit to a checkbox will surface here.

```
fail  tasks.md:8: T1: checkbox/state drift (checkbox=[x], state=pending) (sync)
```

---

## Gate 7 — Traceability

Bidirectional link check between requirements and tasks:

- **Forward (warning):** every `## Requirement N` should be referenced by at least one task's
  `requirements:` key. Unreferenced requirements are a *warning* — they do not fail the check.
- **Backward (failure):** a task may not reference a requirement number that does not exist in
  `requirements.md`.

```
warn  requirements.md: requirement 3 not referenced by any task (traceability)
fail  tasks.md:18: T5: references requirement 7 which is not defined in requirements.md (traceability)
```

---

## How gates relate to `approve`

`specd approve` does not run all seven gates — it runs only the gate for the artifact the *current*
planning phase produces, so you can approve requirements while design and tasks are still stubs:

| Approving | Gate checked |
|-----------|--------------|
| `requirements → design` | EARS (gate 1) |
| `design → tasks` | Design (gate 2) |
| `tasks → executing` | Task-schema + DAG (gates 3–4) |

`specd check` is the full sweep you run before approving (and the one CI runs on every push). The
exit-code contract (`0` valid, `1` violations) is what both CI and driving agents branch on.
