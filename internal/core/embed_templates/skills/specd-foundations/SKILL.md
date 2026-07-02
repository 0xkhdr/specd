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
  roles/                   # scout · craftsman · auditor · validator prompts
  skills/                  # this skill pack (read on demand, see index below)
  specs/<slug>/            # one dir per feature
    requirements.md (EARS) · design.md · tasks.md (wave DAG)
    decisions.md (ADR) · memory.md · mid-requirements.md
    state.json             # CLI-owned machine truth — never hand-edit
```

The Markdown is your authored truth for *intent*. `state.json` is machine truth for
*status*; mutate it only through `specd` (never flip a `tasks.md` checkbox by hand).

## Optional slash/workflow wrappers

If a repo ships `scripts/specd-workflow.sh` and `scripts/specd-workflow.py`, hosts may map
`/init`, `/steer`, `/spec`, and `/pinky-brain` to them. These wrappers are convenience only:
they delegate mutations to native `specd`, keep steering writes limited to canonical files,
never complete tasks, and never forge Pinky proof. When in doubt, run native `specd`.

## Execution mode (per spec)

Each spec runs in one of two modes, recorded in its `state.json` (`specd mode <spec>` shows it):

- **base** (default) — the plain lifecycle; you, the host agent, own every step.
- **orchestrated** — Brain/Pinky may drive it; opt in with `specd mode <spec> --set orchestrated`.

Base is always the default; orchestration is an explicit, per-spec opt-in — never auto-escalate.
Project `orchestration.enabled` only *permits* orchestration; the spec's mode *selects* it. After
`tasks.md`, `specd mode <spec> --recommend` gives a deterministic, advisory verdict — surface it as
a suggestion and let the user decide. See AGENTS.md "Execution mode" for the full protocol.

## Skill index — load each only when its trigger fires (progressive disclosure)

| Skill | Load when |
|-------|-----------|
| `specd-steering` | After `init`, before authoring any spec — and whenever steering drifts. Teaches you to inspect the repo and author `product/structure/tech.md` + set `config.defaultVerify`. |
| `specd-requirements` | Entering the requirements phase. EARS syntax + the `ears` gate. |
| `specd-design` | Entering the design phase. The mandatory `design.md` sections + the `design` gate. |
| `specd-tasks` | Entering the tasks phase. The wave DAG, the seven task keys, acyclicity + the `task-schema`/`dag` gates. |
| `specd-execute` | Entering executing/verifying. The next→implement→verify→complete loop, roles, `dispatch`, the `evidence` gate. |
| `specd-eval-author` | Authoring/refining an eval rubric after `specd eval init`. The four check kinds, scoring/`minScore`, trajectory predicates, the sandboxed `command` contract. |
| `specd-brain` | Entering orchestration. Sensing specd-owned state, deterministic stepping, program scheduling, the no-LLM boundary. |
| `specd-pinky` | Operating a Pinky worker. Context, claim, heartbeat, progress, query/inbox, blocker, report, release under lease. |
| `specd-review` | Reviewing a completed spec. The `review_report.md` sections, reviewer brief, `review checklist`, and the `review` gate. |
| `specd-maintenance` | Scheduled maintenance programs. `program schedule`/`program tick`, host-triggered, no-daemon, idempotent ticks. |
| `specd-ingest` | Legacy ingestion. Read `inventory.json`, reverse-engineer requirements/design/tasks, close the `ingest` coverage gate. |

Pay context only for the stage you are in. This `specd-foundations` is the only
always-loaded skill; it indexes all seven stage skills above.
