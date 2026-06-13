# specd — Core Philosophy & Principles

> *Deterministic process. Non-deterministic agent. Evidence-gated truth.*

---

## 1. The Foundational Split

**The agent reasons. The harness enforces.**

`specd` is not a model. It is not an agent. It is a **deterministic state machine** that any coding agent drives via shell commands. The agent provides creativity, analysis, and code generation. The harness provides process integrity, validation, and durable state.

This split exists because:
- Agents hallucinate, forget, and drift.
- A checkbox flip, a DAG-cycle check, or a state transition must never hallucinate.
- The **process** is guaranteed even when the **agent** varies.

---

## 2. Specs Are the Source of Truth

The agent does not hold the plan in its context window. The plan lives in `.specd/` as durable, versioned, human-readable artifacts.

| Artifact | Purpose |
|----------|---------|
| `requirements.md` | EARS-formatted user stories & acceptance criteria |
| `design.md` | Architecture, sequence diagrams, file structure |
| `tasks.md` | Wave DAG of executable tasks with mandatory schema |
| `decisions.md` | Numbered ADRs (Architecture Decision Records) |
| `memory.md` | Source-attributed learnings |
| `mid-requirements.md` | In-flight requirement changes with impact |
| `state.json` | **CLI-owned machine truth** — never hand-edit |

**Principle:** Markdown is authored truth for *intent*. `state.json` is machine truth for *status*. The CLI keeps them in sync. Drift is a gate failure.

---

## 3. Evidence Gates Every State Change

Nothing moves forward without proof.

```
requirements → [approve + EARS valid] → design
    design   → [approve + structure valid] → tasks
    tasks    → [approve + DAG valid] → executing
 executing  → [evidence per task] → complete
```

- `specd check` runs 7 validation gates before any approval.
- `specd task <id> --status complete --evidence "..."` is the only way to mark done.
- Evidence must be non-empty: commit SHAs, test output, CI links.
- The agent cannot mark its own homework.

**Principle:** *Trust is not assumed. It is recorded.*

---

## 4. Waves, Not Lines

Work is a **DAG of concurrent batches**, not a todo list.

- **Wave 1:** Tasks with zero dependencies (run in parallel)
- **Wave 2:** Tasks whose deps were satisfied by Wave 1
- **Wave N:** Continue until the critical path is complete

`specd next` returns the single next runnable task with focused context. This prevents context window pollution and keeps the agent's attention narrow.

**Principle:** *Concurrency is explicit. Dependencies are validated. The critical path is visible.*

---

## 5. Agent-Agnostic by Design

Any agent that can run a shell command can drive `specd`. No API. No plugin. No MCP.

The integration mechanism is `AGENTS.md` — a prompt file at the repo root that teaches the host agent to:

1. Load `.specd/steering/*` at session start
2. Follow the workflow phases strictly
3. Mutate state **only** through `specd` CLI commands
4. Adopt role prompts per task type (investigator, builder, reviewer, verifier)
5. Never claim completion without passing `--evidence`

**Principle:** *The harness is portable. The process is universal. The agent is interchangeable.*

---

## 6. Human Gates at Phase Boundaries

Automation handles validation. Humans handle intent.

`specd approve` advances the planning phase:
- requirements → design
- design → tasks
- tasks → executing

Mid-flight changes (`specd midreq`) with high/critical impact gate execution until a human clears them.

**Principle:** *Machines check correctness. Humans check intent.*

---

## 7. Deterministic Reporting

`specd report` produces a deterministic snapshot (markdown or single-file HTML) of the entire spec state. It is generated from `state.json` and the markdown artifacts — never from the agent's memory.

**Principle:** *What happened is knowable. What remains is visible. What changed is traceable.*

---

## 8. Steering as Constitution

The `.specd/steering/` directory holds shared context that outlives any single spec:

| File | Scope |
|------|-------|
| `reasoning.md` | How to think through problems |
| `workflow.md` | Phase transitions and gate rules |
| `product.md` | Domain constraints and user context |
| `tech.md` | Stack, patterns, and conventions |
| `structure.md` | File organization and module boundaries |
| `memory.md` | Promoted learnings across specs |

**Principle:** *Context is durable, not conversational. Alignment is structural, not prompt-based.*

---

## Summary: How specd Thinks

| Instead of... | specd does... |
|---------------|---------------|
| Agent owns the plan | Specs own the plan |
| Linear todo lists | Wave DAG with critical path |
| Self-reported completion | Evidence-gated state transitions |
| Prompt-based alignment | Steering-file constitution |
| API/plugin coupling | Shell-command portability |
| Session-scoped memory | Durable, validated, auditable state |

> **specd thinks like a safety-critical control system, not like an assistant.**
> It enforces that specs are written correctly before execution proceeds.
> It proves tasks are complete with evidence.
> It maintains a durable state machine that outlives any single session.

---

*Agent-agnostic. Spec-driven. Evidence-gated. Deterministic.*
