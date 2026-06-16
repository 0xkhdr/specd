---
name: specd-foundations
description: The specd constitution — the eight principles, the Foundational Split, exit codes, the .specd/ file map, and the index of every other specd skill with its trigger. Load this once per session before driving the harness; it makes every other skill lazy-loadable.
---

# specd foundations

`specd` is an agent-agnostic, spec-driven harness you drive entirely through the
`specd` CLI via your shell. **You reason; the harness enforces.** Read this once at
the start of a session, then pull a stage skill only when you enter that stage.

## The Foundational Split (Principle 1)

The agent does the creative thinking (perceive the repo, author specs, write code).
The harness is deterministic, zero-LLM, zero-inference, and writes nothing outside
`.specd/`. It has exactly two jobs: **scaffold** (`init`, `new`) and **enforce**
(`check`, `verify`, `task`, the gates). It never perceives the repo or authors prose
for you — that is your work.

## The eight principles

1. **The Foundational Split** — agent thinks, harness enforces.
2. **Specs as the source of truth** — the plan is versioned Markdown on disk, not
   floating in the context window.
3. **Evidence gates every state change** — trust is recorded, not assumed.
4. **Waves, not lines** — work is a DAG of concurrent batches, not a flat list.
5. **Agent-agnostic by design** — one CLI, any host with a shell.
6. **Human gates at phase boundaries** — semantic transitions need `specd approve`.
7. **Deterministic reporting** — reports derive from `state.json` + artifacts.
8. **Steering as constitution** — durable steering files outlive a chat session.

## Exit codes (every command)

| Code | Meaning |
|------|---------|
| `0` | Success |
| `1` | Gate / validation failed (enforcement said no) |
| `2` | Usage error (bad flags, unknown command) |
| `3` | Not found (`.specd/` or spec missing) |

## The `.specd/` file map

```
.specd/
  config.json              # defaultVerify, roles.subagentMode, gate severities
  steering/                # durable constitution (you author product/tech/structure)
    reasoning.md  workflow.md            # frozen: the thinking + lifecycle loop
    product.md    tech.md    structure.md # YOU author from repo evidence (see specd-steering)
    memory.md                            # promoted learnings (phase-scoped load)
  roles/                   # investigator · builder · reviewer · verifier prompts
  skills/                  # this skill pack (read on demand, see index below)
  specs/<slug>/            # one dir per feature
    requirements.md (EARS) · design.md · tasks.md (wave DAG)
    decisions.md (ADR) · memory.md · mid-requirements.md
    state.json             # CLI-owned machine truth — never hand-edit
```

The Markdown is your authored truth for *intent*. `state.json` is machine truth for
*status*; mutate it only through `specd` (never flip a `tasks.md` checkbox by hand).

## Skill index — load each only when its trigger fires (progressive disclosure)

| Skill | Load when |
|-------|-----------|
| `specd-steering` | After `init`, before authoring any spec — and whenever steering drifts. Teaches you to inspect the repo and author `product/structure/tech.md` + set `config.defaultVerify`. |
| `specd-requirements` | Entering the requirements phase. EARS syntax + the `ears` gate. |
| `specd-design` | Entering the design phase. The mandatory `design.md` sections + the `design` gate. |
| `specd-tasks` | Entering the tasks phase. The wave DAG, the seven task keys, acyclicity + the `task-schema`/`dag` gates. |
| `specd-execute` | Entering executing/verifying. The next→implement→verify→complete loop, roles, `dispatch`, the `evidence` gate. |

Pay context only for the stage you are in. This `specd-foundations` is the only
always-loaded skill.
