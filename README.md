# specd — the Spec-Driven Coding Harness

An **agent-agnostic, spec-driven coding harness** that fuses **Kiro's spec workflow** with
**structured reasoning discipline**. It is a small, deterministic CLI plus a drop-in prompt/steering
pack. Any coding agent that can run a shell command can drive it — no API, plugin, or MCP needed.

`specd` is **not** a model and **not** an agent. It is:

1. A **directory convention** under `.specd/` holding specs, steering, and durable state.
2. A **deterministic CLI** that does all bookkeeping — scaffold, validate, status, evidence-gated
   task flips, ledger appends, report render. **Zero LLM calls.**
3. A **prompt pack** (`AGENTS.md` + steering + role prompts) that teaches the host agent the
   workflow and tells it to use `specd` for every state mutation.

The split is the point: agents are non-deterministic; a checkbox flip or a DAG-cycle check must
not be. The CLI guarantees the *process* even when the agent varies.

## Install

```sh
npx specd <command>
# or, from a clone:
npm install && npm run build
node dist/cli.js <command>
```

Requires Node ≥18. Zero runtime dependencies.

## Quickstart

```sh
specd init                                  # scaffold .specd/ + AGENTS.md (idempotent)
specd new my-feature --title "My Feature"   # create a spec with six artifacts + state.json
# author .specd/specs/my-feature/requirements.md (EARS), then gate:
specd check my-feature
specd approve my-feature                     # human approves requirements -> design
# author design.md, then tasks.md (wave DAG), then gate:
specd check my-feature
specd approve my-feature                     # approve design -> tasks
specd approve my-feature                     # approve tasks  -> executing
# execute loop:
specd context my-feature                    # phase-scoped briefing: what to load + next action
specd next my-feature                       # -> next runnable task (focused prompt)
specd next my-feature --all                 # -> whole runnable frontier (fan out builders in parallel)
specd task my-feature T1 --status complete --evidence "commit abc; npm test PASS"
specd report my-feature                     # deterministic snapshot
```

## Commands

| Command | Purpose |
|---|---|
| `specd init [--force]` | Scaffold `.specd/` (steering, roles, config) + `AGENTS.md`. Idempotent. |
| `specd new <slug> [--title "..."]` | Create a spec folder with six artifacts + `state.json`. |
| `specd status [<slug>] [--json]` | Durable ledger / board: counts, wave graph, blockers, next task. |
| `specd context <slug> [--json]` | Minimal phase-scoped briefing: which files to load now + the next action. |
| `specd check <slug> [--json]` | Run all seven validation gates. Exit 0 iff valid. |
| `specd next <slug> [--all] [--json]` | The single next runnable task as a paste-ready prompt block; `--all` prints the whole runnable frontier for parallel dispatch. |
| `specd task <slug> <id> --status <s> [--evidence\|--reason]` | The evidence gate. Dual-writes `tasks.md` + `state.json`. Refuses while gated (override `--force`). |
| `specd approve <slug> [--json]` | Record a human approval: advance the planning phase (requirements→design→tasks→executing), or clear a midreq `awaiting-approval` gate. |
| `specd decision <slug> "<text>" [--supersedes <id>]` | Append a numbered ADR. |
| `specd midreq <slug> "<input>" --impact <low\|medium\|high\|critical> [--interpretation ..] [--changes ..]` | Log mid-flight feedback; gates on high/critical. |
| `specd memory <slug> add\|promote ...` | Source-attributed learnings; promote to project steering. |
| `specd report <slug> [--format md\|html] [--out <path>]` | Deterministic snapshot (markdown or one dependency-free HTML file). |
| `specd waves <slug> [--json]` | The wave DAG, critical path, and blockers. |

**Exit codes:** `0` ok/valid · `1` validation/gate failure · `2` usage error · `3` not found.

## Validation gates (`specd check`)

1. **EARS** — every acceptance criterion matches an EARS pattern; each requirement has a user
   story + ≥1 criterion.
2. **Design** — all required H2 sections present, non-empty, no `TODO`.
3. **Task-schema** — every task has the seven mandatory keys; valid role; `verify` is a command
   unless the role is read-only.
4. **DAG** — acyclic; no orphan deps; deps live in an earlier-or-equal wave.
5. **Evidence** — no task is `complete` without non-empty evidence.
6. **Sync** — `tasks.md` checkboxes match `state.json` statuses (no drift).
7. **Traceability** (warn) — every requirement referenced by ≥1 task.

## The spec folder

```
.specd/
├── config.json
├── steering/      reasoning.md, workflow.md, product.md, tech.md, structure.md, memory.md
├── roles/         investigator.md, builder.md, reviewer.md, verifier.md
└── specs/<slug>/  requirements.md design.md tasks.md decisions.md memory.md
                   mid-requirements.md  +  state.json (CLI-owned)
```

Markdown files are your authored truth for *intent*; `state.json` is machine truth for *status*.
The CLI keeps `tasks.md` checkboxes and `state.json` in sync — never hand-edit `state.json`.

**Portability note:** `.specd/` is self-contained. Removing it removes all harness state with no
side effects outside the directory. `AGENTS.md` at the repo root is a prompt-only file and carries
no durable state.

## Using it with any agent

The portability mechanism is `AGENTS.md` (read by Claude Code, Cursor, Aider, Codex, pi, and
others; Kiro reads the steering files). It tells the host agent to: load `.specd/steering/*`,
follow the workflow, mutate state only through `specd`, adopt the role prompts, and never claim a
task complete without passing evidence. Because the only requirement is "can run a shell command",
the harness works with **any** agent — no API, plugin, or MCP.

## Documentation

Full production documentation lives in **[`docs/`](docs/README.md)**:

| Doc | For |
|---|---|
| [Introduction & Mental Model](docs/introduction.md) | understanding what specd is |
| [Getting Started](docs/getting-started.md) | a full spec, end-to-end |
| [Core Concepts](docs/concepts.md) | phases, gates, waves, evidence, steering |
| [The Spec Folder](docs/spec-anatomy.md) | every artifact, EARS, task schema, `state.json` |
| [CLI Reference](docs/cli-reference.md) | every command and flag |
| [Validation Gates](docs/validation-gates.md) | the seven `specd check` gates |
| [Agent Integration](docs/agent-integration.md) | `AGENTS.md`, roles, parallel dispatch |
| [Architecture](docs/ARCHITECTURE.md) | internals: philosophy→code, concurrency, invariants |
| [Contributing](docs/contributing.md) | dev setup, file map, how to extend |

## Design references

The original `SPEC.md` design document has been **retired** — the implementation is now the source
of truth. To understand or extend `specd`, read, in order:

- **[`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md)** — how the harness works: each philosophy
  principle mapped to the code that enforces it, plus the full lifecycle sequence and key invariants.
- **[`specd-core-philosophy.md`](specd-core-philosophy.md)** — the eight principles (the *why*).
- **[`CLAUDE.md`](CLAUDE.md)** — the contributor map (what file to edit for each task, invariants).

Source comments still cite section numbers (`SPEC §5.2`, `§10`, …) as historical references to that
retired spec; treat them as design rationale, not a live file to open.

## Development

```sh
npm install
npm run build      # tsc -> dist/ + copy templates
npm test           # node --test over all gates, parser round-trip, report, and the e2e scenario
```

## License

MIT — see `LICENSE`.
