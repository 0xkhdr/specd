# 3. Core Concepts

The vocabulary `specd` runs on. Read this once and every command makes sense.

---

## The two layers: reasoning phases vs. spec statuses

specd tracks two related-but-distinct things.

**Reasoning phases** — the *thinking architecture* the agent applies (from
`steering/reasoning.md`). Six phases, never skipped:

```
PERCEIVE → ANALYZE → PLAN → EXECUTE → VERIFY → REFLECT
```

| Phase | Question it answers | Output |
|-------|---------------------|--------|
| PERCEIVE | What is being asked? What exists? | context loaded, request classified |
| ANALYZE | What must be true? | `requirements.md` (EARS) |
| PLAN (design) | How will we satisfy it? | `design.md` |
| PLAN (tasks) | What atomic units, in what order? | `tasks.md` (wave DAG) |
| EXECUTE | Build one task. | code + diff summary |
| VERIFY | Did it actually work? | evidence in `state.json` |
| REFLECT | What did we learn? | `memory.md`, `decisions.md` |

**Spec status** — the *persisted machine state* in `state.json`. The phase is **derived** from the
status; it is never stored independently and never editable by the agent.

| `status` | derived `phase` | meaning |
|----------|-----------------|---------|
| `requirements` | `analyze` | authoring/refining requirements |
| `design` | `plan` | authoring the design |
| `tasks` | `plan` | authoring the wave DAG |
| `executing` | `execute` | building tasks |
| `blocked` | `execute` | every remaining task is blocked |
| `verifying` | `verify` | all tasks complete, awaiting human acceptance |
| `complete` | `reflect` | accepted and finished |

`PERCEIVE` is the only phase with no persisted status — it is the pre-spec cognitive beat that
happens *before* `specd new` creates any state. The mapping lives in one place:
`phaseForStatus()` in `src/core/phases.ts` (the single source of truth).

---

## Gates

A **gate** is a checkpoint that work cannot cross without proof. specd has gates at three
granularities.

### 1. Validation gates (`specd check`)

Seven deterministic checks over the spec's artifacts. Exit `0` only if all pass. Covered in full in
[Validation Gates](validation-gates.md):

1. **EARS** — requirements are well-formed.
2. **Design** — all required sections present, non-empty, no TODOs.
3. **Task-schema** — every task has the seven keys, a valid role, a real verify command.
4. **DAG** — acyclic, no orphan deps, deps in earlier-or-equal waves.
5. **Evidence** — no task is `complete` without evidence.
6. **Sync** — `tasks.md` checkboxes match `state.json` statuses.
7. **Traceability** — every requirement is referenced by a task (warn); no task references a
   requirement that does not exist (fail).

### 2. The per-task evidence gate (`specd task`)

The integrity core. `--status complete` is rejected (exit `1`) unless:

- `--evidence` is present and non-empty, **and**
- every dependency of the task is already `complete`.

The agent cannot mark its own homework. *Trust is recorded, not assumed.*

### 3. Phase boundary gates (human) (`specd approve`)

Automation checks correctness; humans check intent. `specd approve` is the only sanctioned way to:

- Advance the planning ratchet: `requirements → design → tasks → executing` (each step requires the
  gate for *that* artifact to be green).
- Clear a `midreq`-raised `awaiting-approval` gate so work resumes.
- Accept the spec-level VERIFY (`verifying → complete`).

---

## The spec-level VERIFY gate

When the last task completes, specd does **not** mark the spec complete. It sets
`status: verifying`. This mirrors the per-task evidence gate at spec granularity: every task is
individually proven, but a human still confirms the feature works as a whole. Only `specd approve`
advances `verifying → complete`, and specd refuses to regress an already-accepted `complete` spec
back to `verifying`. **specd never auto-completes a spec.**

---

## Waves and the DAG

Tasks form a **directed acyclic graph** grouped into **waves**:

- **Wave 1** — tasks with zero dependencies. Runnable immediately, in parallel.
- **Wave 2** — tasks whose deps were all satisfied by Wave 1.
- **Wave N** — continue until the critical path completes.

A task is **runnable** when its status is `pending` *and* all its dependencies are `complete`. The
DAG engine (`src/core/dag.ts`) computes:

- **next runnable** (`specd next`) — the single lowest-wave, lowest-id runnable task.
- **runnable frontier** (`specd next --all`) — *every* currently-runnable task, for parallel
  dispatch.
- **critical path** (`specd waves`) — the longest dependency chain, i.e. the minimum sequential
  depth.

When nothing is runnable, the engine classifies *why*: `all-complete`, `all-blocked` (every
remaining task is `blocked`), or `waiting` (the pending frontier is gated by incomplete deps).

---

## Task statuses

A task in `state.json` is always in one of four states:

| Status | Glyph | Set by | Notes |
|--------|-------|--------|-------|
| `pending` | ○ | initial / `specd task --status pending` | not started |
| `running` | ◐ | `specd task --status running` | in progress |
| `complete` | ✓ | `specd task --status complete --evidence` | requires evidence + complete deps |
| `blocked` | ⚠ | `specd task --status blocked --reason` | requires a reason; surfaces in blockers |

The spec status is then *derived* from the set of task statuses: once any task leaves `pending` the
spec is `executing`; if every remaining task is blocked it becomes `blocked`; when all tasks complete
it becomes `verifying`.

---

## Evidence

Evidence is the currency of completion. It is a free-form non-empty string passed as `--evidence`,
and it should point at *proof*: a commit SHA, test output with pass counts, a CI link, or a manual
verification result. The reasoning steering states the hardest rule plainly:

> A builder's "done" is **NOT** evidence. A verify step or manual check must pass before any task is
> marked complete.

Evidence is stored on the task in `state.json` and annotated into `tasks.md`, so the record of *what
proved this task done* is durable and auditable.

---

## Steering — the constitution

`.specd/steering/` holds shared context that **outlives any single spec**. It is loaded as durable
context, not re-derived from a prompt each session.

| File | Scope |
|------|-------|
| `reasoning.md` | how to think through problems (the six phases) |
| `workflow.md` | phase transitions and gate rules (the lifecycle) |
| `product.md` | domain constraints and user context |
| `tech.md` | stack, patterns, conventions |
| `structure.md` | file organization and module boundaries |
| `memory.md` | learnings promoted across specs |

`reasoning.md` and `workflow.md` are **always-on**; the rest are **phase-scoped** — `specd context`
tells the agent exactly which to load for the current phase, so the context window holds signal
rather than the whole doc set. *Context is durable, not conversational. Alignment is structural, not
prompt-based.*

---

## Roles

When executing, the agent adopts one of four **roles** per task (from `.specd/roles/`). The role
constrains capability and output shape:

| Role | Capability | Writes code? |
|------|------------|--------------|
| `investigator` | locate, understand, trace | **No** (read-only) |
| `builder` | implement exactly one task | **Yes** |
| `reviewer` | defect audit of a diff | **No** (read-only) |
| `verifier` | run tests/types/build, capture evidence | **No** |

`investigator` and `reviewer` are the read-only roles — they are the only roles allowed a `verify:
N/A` in a task. See [Agent Integration](agent-integration.md) for the full role contracts.

---

## Memory and promotion

`specd memory <slug> add` records a source-attributed learning in the spec's `memory.md`. When the
same pattern recurs across enough specs, `specd memory <slug> promote` lifts it into the global
`steering/memory.md`. Promotion is **threshold-gated** (`config.json.promotionThreshold`, default
`3`) so one-off learnings don't pollute the constitution; `--force` overrides for a known-critical
pattern seen once.

---

## Mid-flight requirements (`midreq`)

Iterative refinement is first-class. New user input mid-execution is logged with
`specd midreq <slug> "<verbatim>" --impact <low|medium|high|critical>`. It always appends a turn
entry to `mid-requirements.md` and bumps the turn counter. `high`/`critical` additionally raise the
`awaiting-approval` gate, which freezes work (`next`/`task` refuse) until a human runs
`specd approve`. *Machines check correctness; humans check intent.*

---

## Decisions (ADRs)

`specd decision <slug> "<text>"` appends a numbered Architecture Decision Record (`ADR-001`,
`ADR-002`, …) to `decisions.md`, optionally `--supersedes` an earlier one. Deviations from the spec
are recorded, never silent.

---

## Determinism and durability

Two properties hold across everything above:

- **Deterministic** — every command is pure bookkeeping over files. No randomness, no model calls.
  `specd report` is assembled from `state.json` and the markdown artifacts, never from an agent's
  memory.
- **Durable & crash-safe** — every write is atomic (temp → `fsync` → `rename`). Concurrent mutation
  is guarded by a per-spec lock plus an optimistic revision compare-and-swap (see
  [Architecture §4](ARCHITECTURE.md#4-concurrency-model)).

*What happened is knowable. What remains is visible. What changed is traceable.*
