# 1. Introduction & Mental Model

> *Deterministic process. Non-deterministic agent. Evidence-gated truth.*

## What `specd` is

`specd` is an **agent-agnostic, spec-driven coding harness**. It fuses **Kiro's spec workflow**
(requirements → design → tasks → execution, each gated by a human) with a **structured reasoning
discipline** (a fixed six-phase thinking architecture). Concretely it is three things, and nothing
more:

1. **A directory convention** — everything lives under `.specd/` in the target repo: specs,
   steering, role prompts, config, and durable state.
2. **A deterministic CLI** — a small TypeScript program (`specd`) that does *all* bookkeeping:
   scaffold, validate, status, evidence-gated task flips, ledger appends, report rendering. It makes
   **zero LLM calls** and has **zero runtime dependencies**.
3. **A prompt pack** — `AGENTS.md` + steering files + four role prompts that teach the host agent
   the workflow and instruct it to mutate state *only* through `specd`.

## What `specd` is **not**

- It is **not a model.** It contains no weights and calls no API.
- It is **not an agent.** It does not reason, write code, or make decisions about your product.
- It is **not a framework you import.** Your application code never depends on it. It writes no
  files outside `.specd/`.

## The foundational split

**The agent reasons. The harness enforces.**

This is the single idea everything else follows from. A coding agent provides creativity, analysis,
and code generation — all inherently non-deterministic. A checkbox flip, a DAG-cycle check, or a
state transition must *never* be non-deterministic. So `specd` draws a hard line:

| The agent owns | The harness owns |
|----------------|------------------|
| Understanding the request | Recording the request's lifecycle |
| Writing requirements, design, tasks | Validating that they are well-formed |
| Writing code | Refusing "done" without evidence |
| Deciding *what* to do next | Computing *which task is runnable* |
| Judgement and intent | Process integrity and durable truth |

The split exists because agents hallucinate, forget, and drift — but the **process** must be
guaranteed even when the **agent** varies. `specd` thinks like a safety-critical control system, not
like an assistant.

## Why specs are the source of truth

The agent does **not** hold the plan in its context window. The plan lives on disk as durable,
versioned, human-readable artifacts. This buys three things a context window cannot:

- **Durability** — state outlives any session, crash, or model swap.
- **Auditability** — every transition is recorded and traceable.
- **Focus** — the agent loads only what the current phase needs (`specd context`), so its window
  holds signal, not the whole repo of docs.

Markdown is *authored truth for intent*. `state.json` is *machine truth for status*. The CLI keeps
them in sync; any drift is a gate failure.

## The two truths, kept in sync

```
 tasks.md  (human truth)            state.json  (machine truth)
 ─────────────────────────          ───────────────────────────
 - [x] T1 — parse config            "T1": { "status": "complete",
   - role: builder                            "evidence": "commit abc; npm test PASS" }
   ...
            ▲                                        ▲
            └──────────  specd  reconciles  ─────────┘
                  (every load; drift = check gate 6 failure)
```

You author and read `tasks.md`. The CLI owns `state.json` — **never hand-edit it.** When you flip a
task with `specd task`, the CLI dual-writes *both* files atomically so they can never disagree.

## Evidence gates everything

Nothing moves forward without proof. *Trust is not assumed; it is recorded.*

- The only way to mark a task done is
  `specd task <slug> <id> --status complete --evidence "<proof>"`.
- Evidence must be **non-empty** — a commit SHA, test output, a CI link. A builder's word is not
  evidence.
- A task cannot complete while its dependencies are incomplete.
- When every task is complete, the spec does **not** auto-finish. It enters a spec-level VERIFY gate
  (`status: verifying`); only a human running `specd approve` advances it to `complete`.

The agent cannot mark its own homework.

## Work is waves, not lines

Tasks form a **DAG of concurrent batches**, not a flat todo list:

- **Wave 1** — tasks with zero dependencies (can run in parallel).
- **Wave 2** — tasks whose deps were satisfied by Wave 1.
- **Wave N** — continue until the critical path is complete.

`specd next` hands out the single next runnable task with focused context. `specd next --all` returns
the whole runnable frontier so an orchestrator can fan out builders in parallel. Concurrency is
explicit, dependencies are validated, and the critical path is always visible.

## When to use specd

specd shines when a change is large enough that **process integrity matters more than speed**:

- Multi-step features where an agent could lose track of what is done.
- Work driven by autonomous or semi-autonomous agents that need a guardrail.
- Anything where you need an auditable record of *what was built and proven*.

It is deliberately heavyweight for a one-line fix. The workflow's own steering says so: a *question*
gets answered and stopped; only a *change* enters the spec lifecycle.

## Where to go next

- New to specd? → [Getting Started](getting-started.md) walks a full spec end-to-end.
- Want the vocabulary? → [Core Concepts](concepts.md).
- Want to understand the internals? → [Architecture](ARCHITECTURE.md).
