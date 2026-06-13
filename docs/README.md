# specd Documentation

Production documentation for **`specd`** — an agent-agnostic, spec-driven coding harness.

`specd` is **not a model and not an agent**. It is a deterministic, zero-LLM CLI plus a prompt
pack that teaches *any* coding agent to follow a disciplined spec workflow: requirements → design →
tasks → evidence-gated execution. The agent reasons; the harness enforces.

> *Deterministic process. Non-deterministic agent. Evidence-gated truth.*

---

## Read in this order

If you are **new**, follow the path top to bottom. If you are **extending the tool**, jump to
Architecture and Contributing.

| # | Document | What it covers | Audience |
|---|----------|----------------|----------|
| 1 | [Introduction & Mental Model](introduction.md) | What specd is, the foundational split, when to use it | Everyone |
| 2 | [Getting Started](getting-started.md) | Install, `init`, and a full first spec end-to-end | Users |
| 3 | [Core Concepts](concepts.md) | Phases, statuses, gates, waves, evidence, steering | Users |
| 4 | [The Spec Folder](spec-anatomy.md) | `.specd/` layout, the six artifacts, EARS, task schema, `state.json`, `config.json` | Users |
| 5 | [CLI Reference](cli-reference.md) | Every command, every flag, exit codes, example output | Users |
| 6 | [Validation Gates](validation-gates.md) | The seven `specd check` gates in detail | Users |
| 7 | [Agent Integration](agent-integration.md) | `AGENTS.md`, the four roles, steering, parallel dispatch | Agent authors |
| 8 | [Architecture](ARCHITECTURE.md) | Philosophy→code map, lifecycle, concurrency, invariants | Contributors |
| 9 | [Contributing](contributing.md) | Dev setup, file map, testing, how to extend | Contributors |

The original design rationale lives in [`../specd-core-philosophy.md`](../specd-core-philosophy.md)
(the *why*, eight principles).

---

## The 60-second version

```sh
npx specd init                                   # scaffold .specd/ + AGENTS.md
npx specd new my-feature --title "My Feature"    # create a spec (6 artifacts + state.json)

# 1. ANALYZE — author requirements.md in EARS, then gate + approve
npx specd check my-feature
npx specd approve my-feature                      # requirements → design

# 2. PLAN — author design.md, then tasks.md (wave DAG), gating between each
npx specd check my-feature
npx specd approve my-feature                      # design → tasks
npx specd approve my-feature                      # tasks  → executing

# 3. EXECUTE — loop: get a task, build it, prove it
npx specd context my-feature                      # phase-scoped briefing
npx specd next my-feature                         # next runnable task
npx specd task my-feature T1 --status complete --evidence "commit abc; npm test PASS"

# 4. VERIFY — when the last task completes the spec enters `verifying`
npx specd approve my-feature                      # human accepts → complete

# 5. REFLECT
npx specd report my-feature                       # deterministic snapshot
```

**Exit codes:** `0` ok · `1` gate/validation failure · `2` usage error · `3` not found.

---

## Core guarantees

- **Zero runtime dependencies.** Built CLI runs on Node ≥18 with no install step (`npx specd`).
- **Zero LLM calls.** Every command is deterministic bookkeeping. The model never runs *inside* the
  harness.
- **Atomic, crash-safe state.** Every write is temp → `fsync` → `rename`. A crash never corrupts
  `state.json` or `tasks.md`.
- **Self-contained.** All state lives under `.specd/`. Delete the directory and the harness is gone
  with no side effects elsewhere.
- **Portable.** The only integration requirement is "can run a shell command." No API, plugin, or
  MCP.
