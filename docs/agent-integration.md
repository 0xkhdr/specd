# 7. Agent Integration

How any coding agent drives `specd`, and how the prompt pack makes the harness portable. This is the
"agent-agnostic" principle in practice: *the harness is portable, the process is universal, the agent
is interchangeable.*

## The integration mechanism: `AGENTS.md`

There is no API, plugin, SDK, or MCP server. The *only* requirement is that the host agent can run a
shell command. The integration is a single prompt file at the repo root: **`AGENTS.md`**, written by
`specd init`.

`AGENTS.md` is read automatically by Claude Code, Cursor, Aider, Codex, pi, and others (Kiro reads
the steering files directly). It teaches the host agent five non-negotiable rules:

1. **Load context first.** At session start, read the always-on steering files
   `.specd/steering/{reasoning,workflow,product,tech,structure}.md`. `memory.md` is loaded
   phase-scoped. `specd context <spec>` says exactly what to load when.
2. **Follow the workflow** in `steering/workflow.md` — the INTAKE → PERCEIVE → ANALYZE → PLAN →
   EXECUTE → VERIFY → REFLECT lifecycle. Each `→` is a gate.
3. **Mutate state only through `specd`.** Never hand-edit `state.json`; never flip a `tasks.md`
   checkbox by hand.
4. **Adopt roles** from `.specd/roles/*` when executing.
5. **Evidence gate.** Never mark a task complete without a passing verify or manual proof, passed as
   `--evidence`. A builder's word is not evidence.

Because the contract is "run a shell command," the same harness drives any agent identically.

## The orientation loop

An agent driving specd does not keep the plan in its head. It re-orients each turn with three cheap
commands:

```sh
specd context <spec>     # what phase am I in, what to load, what's the next action
specd status <spec>      # where am I overall (counts, waves, blockers)
specd next <spec>        # what is my next unit of work
specd check <spec>       # am I allowed to advance
```

`specd context` is the key context-engineering primitive — it curates the window to the current
phase instead of dumping every doc every turn. See [CLI Reference §context](cli-reference.md#context).

## The four roles

When executing a task, the agent adopts the **role** named in that task's `role:` key. Each role has
a strict capability boundary and a fixed structured output block (`=== ROLE RESULT ===`). The roles
live in `.specd/roles/` and you can customize them per project.

### investigator — read-only research

- **Capability:** locate, understand, trace. **May NOT write code or files.**
- Answers exactly the question in the task `contract`; reports `file:line` for every claim; no
  speculation.
- The only role (with reviewer) allowed `verify: N/A`.

### builder — write exactly one task

- **Capability:** implement **one** atomic task. **May write code.**
- Implements the `contract` and nothing else — no scope creep. Touches only the files in `files:`
  (plus their tests).
- Makes `acceptance` true; runs (or hands to the verifier) the `verify:` line and captures the
  result as evidence.
- **One task per invocation.** Does not start the next. A builder's "done" is not evidence — the
  verify result is.
- If blocked, stops after **one** retry and reports `blocked` with the exact blocker. Records any
  deviation via `specd decision` before finishing.

### reviewer — read-only defect audit

- **Capability:** audit a diff. **May NOT modify code.**
- Hunts correctness bugs, drift, missed edge cases, security issues, broken contracts.
- Severity-tags every finding (`critical|high|medium|low`); one line per finding
  (`path:line: <severity>: <problem>. <fix>.`); no praise, no scope creep.

### verifier — run checks, capture evidence

- **Capability:** run tests/types/build. **May NOT modify code.**
- Runs the task's `verify:` line exactly; reports pass/fail counts and **verbatim** failure output.
- Maps results back to the `acceptance` criteria. Its `passed` result is the evidence that gates
  `specd task … complete`. Does not fix code — the builder does.

### A typical task arc

```
investigator → maps the extension point   (read-only, file:line findings)
builder      → implements the one task     (writes code, runs verify)
verifier     → confirms it passes          (verbatim results → evidence)
reviewer     → audits the diff             (severity-tagged findings)
            → specd task <spec> <id> --status complete --evidence "<verifier result>"
```

Not every task needs all four — most builder tasks are build + verify. Use investigator/reviewer
where the contract calls for research or audit.

## Inline vs. delegated roles (`subagentMode`)

`config.json` has `roles.subagentMode`:

- **`inline`** (default) — the driving agent plays each role itself, switching personas under the
  same capability constraints.
- **`delegate`** — if the host has native subagents, dispatch each role to a fresh subagent. This
  pairs naturally with `specd next --all`: fan the runnable frontier out to parallel builder
  subagents.

The constraint is identical either way; only the execution substrate differs.

## Driving parallel work safely

The runnable frontier (`specd next --all`) lets an orchestrator dispatch multiple builders at once —
one per currently-runnable task. specd makes this safe with two layers (see
[Architecture §4](ARCHITECTURE.md#4-concurrency-model)):

1. A **per-spec advisory lock** (`withSpecLock`) wraps every load → mutate → save critical section,
   so two builders flipping different tasks of the same spec cannot clobber each other.
2. An **optimistic revision compare-and-swap** in `saveState` aborts a write (exit 1) if the on-disk
   revision changed underfoot — defense-in-depth behind the lock.

Practical rule for orchestrators: dispatch the frontier, let each builder complete its task with its
own `specd task` call, then call `specd next --all` again for the next frontier.

## The gate handshake for mid-flight changes

When the user changes the requirements mid-execution, the agent must **not** silently absorb it:

```sh
specd midreq <spec> "<verbatim user input>" --impact <low|medium|high|critical> \
  --interpretation "<what you'll do>" --changes "<plan delta>"
```

`high`/`critical` raises `gate = awaiting-approval`. While gated, `specd next` and `specd task`
refuse to act. The agent presents the revised plan and waits; a human runs `specd approve <spec>` to
clear the gate and resume. This is the structural enforcement of "machines check correctness, humans
check intent."

## Steering is the constitution

The five always-loaded-or-phase-scoped steering files are *durable* shared context — they outlive
any single spec and any single session. They are how you align an agent **structurally** rather than
by re-prompting:

| File | Tune it to encode… |
|------|--------------------|
| `reasoning.md` | the thinking discipline (usually leave as shipped) |
| `workflow.md` | the lifecycle and gate rules (usually leave as shipped) |
| `product.md` | your domain constraints and user context |
| `tech.md` | your stack, patterns, and conventions |
| `structure.md` | your file organization and module boundaries |
| `memory.md` | learnings promoted across specs (grown by `memory promote`) |

Edit `product.md`, `tech.md`, and `structure.md` for your project; they are the highest-leverage way
to make every agent's output fit your codebase.

## Two `AGENTS.md` files — don't confuse them

- **Repo-root `AGENTS.md`** (written by `specd init` into a *user* repo) — teaches the host agent to
  *use* specd on that project. This is the one this document describes.
- **`/AGENTS.md` in the specd source repo** — teaches an agent to *develop* specd itself. Different
  audience, different content. See [Contributing](contributing.md).
