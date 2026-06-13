# specd — Architecture

How the harness works, principle by principle, file by file. Read this with
[`specd-core-philosophy.md`](../specd-core-philosophy.md) (the *why*) and [`CLAUDE.md`](../CLAUDE.md)
(what file to edit for a given task) open alongside.

`specd` is **not a model and not an agent**. It is a directory convention (`.specd/`), a
deterministic zero-LLM CLI, and a prompt pack. The agent reasons; the harness enforces. Everything
below exists to keep the *process* deterministic even when the agent driving it is not.

---

## 1. The one-paragraph model

`src/cli.ts` parses argv and dispatches to `src/commands/*.ts`. Every command that touches a repo
calls `requireSpecdRoot()` (`src/core/paths.ts`) to locate `.specd/` by walking up from cwd. State
lives in two places that must always agree: `state.json` (machine truth — status, phase, evidence)
and `tasks.md` checkboxes (human truth). `src/core/specFiles.ts:loadSpec()` is the canonical
read/mutate entry point — it loads state, `reconcile()`s structural fields from `tasks.md`, persists
if anything drifted, and hands back `{ state, doc }`. `src/commands/task.ts` is the integrity core:
it enforces the evidence gate and dual-writes both files atomically. The six reasoning phases map to
spec status through `src/core/phases.ts:phaseForStatus()` (single source of truth). When every task
completes the spec enters a spec-level VERIFY gate (`status: verifying`); only `specd approve`
advances it to `complete`.

---

## 2. Philosophy → code map

Each of the eight principles, mapped to the file that enforces it. Trace any principle to its code in
one hop.

| # | Principle | Enforced by | Notes |
|---|-----------|-------------|-------|
| 1 | **Agent reasons, harness enforces** (deterministic, zero LLM) | `src/cli.ts` (pure arg routing), all mutation in `src/commands/*`; no network/model deps | `package.json` has only `devDependencies` |
| 2 | **Specs are source of truth** (6 markdown artifacts + `state.json`) | `src/core/specFiles.ts:ARTIFACTS` + `reconcile()`; drift = gate-6 failure in `check.ts` | `reconcile()` runs on every `loadSpec()` |
| 3 | **Evidence gates every change** | `src/commands/check.ts` (7 gates); `src/commands/task.ts` rejects `complete` without `--evidence` or with incomplete deps (exit 1) | the agent can't mark its own homework |
| 4 | **Waves, not lines** (concurrent DAG batches) | `src/core/dag.ts` (`nextRunnable`, `runnableFrontier`, `groupWaves`, `criticalPath`, cycle/orphan/wave-order detection); `next`/`waves` commands | `next --all` exposes the runnable frontier for parallel dispatch |
| 5 | **Agent-agnostic** (any shell-capable agent) | `src/templates/AGENTS.md` + `roles/{investigator,builder,reviewer,verifier}.md`; `config.json.roles.subagentMode` | no API/plugin/MCP required |
| 6 | **Human gates at boundaries** | `src/commands/approve.ts` (planning ratchet + VERIFY→complete); `src/commands/midreq.ts` raises `awaiting-approval`; `next`/`task` refuse while gated | `approve` is the only sanctioned phase-advance |
| 7 | **Deterministic reporting** (snapshot from state, never memory) | `src/commands/report.ts` + `src/core/report.ts` (md + single-file html) | tested in `test/report.test.ts` |
| 8 | **Steering as constitution** (durable, outlives specs) | `src/commands/init.ts` writes `steering/{reasoning,workflow,product,tech,structure,memory}.md`; `src/commands/context.ts` loads them phase-scoped | `context` curates context to the phase |

---

## 3. Lifecycle sequence

The full life of a spec, with the command that drives each transition and the gate that guards it.

```
                                     ┌───────────────── specd midreq (high/critical) ──────────────┐
                                     │                  raises awaiting-approval gate              │
                                     ▼                                                              │
INTAKE ─ specd new ─▶ requirements ─ check ─ approve ─▶ design ─ check ─ approve ─▶ tasks ─ check ─ approve ─▶ executing
 (no state)            (ANALYZE)      gate1   human       (PLAN)   gate2-4  human    (PLAN)  gate2-4  human       (EXECUTE)
                                                                                                                    │
                                                          specd next / next --all  ◀───────────────────────────────┘
                                                          specd task <id> --status complete --evidence "<proof>"
                                                          (evidence gate; dual-write tasks.md + state.json)
                                                                            │
                                                       all tasks complete ──▼
                                                                       verifying ── specd approve ──▶ complete
                                                                        (VERIFY)      human accept      (REFLECT)
                                                                                                          │
                                                                                            specd report / memory promote
```

- **`specd new <slug>`** — scaffolds six artifacts + `state.json` (`status: requirements`, phase `analyze`).
- **`specd check`** — runs all 7 gates; exit 0 iff valid. CI and agents branch on the exit code.
- **`specd approve`** — the human boundary. Advances the planning ratchet
  (`requirements → design → tasks → executing`) only once the gate for *that* phase's artifact is
  green; later clears a midreq gate; finally accepts the spec-level VERIFY (`verifying → complete`).
- **`specd next` / `next --all`** — hands out the single focused runnable task, or the whole runnable
  frontier for parallel dispatch.
- **`specd task <id> --status complete --evidence "<proof>"`** — the only way to flip a task. No
  evidence or incomplete deps → exit 1.
- **`verifying`** — the spec-level VERIFY beat: every task is done but the spec is *not* auto-completed.
  Only `approve` accepts it. This mirrors the per-task evidence gate at spec granularity.

The phase shown under each status is derived by `phases.ts:phaseForStatus()` — never stored
independently, never editable by the agent.

---

## 4. Concurrency model

The philosophy invites parallelism ("waves run in parallel"). Two agents driving Wave-N tasks of the
same spec both read→modify→write `state.json`; without coordination the second write silently
clobbers the first (lost update). Two layers prevent this:

1. **Per-spec advisory lock** — `src/core/lock.ts:withSpecLock(root, slug, fn)` wraps the
   load→mutate→save critical section of every mutating command (`task`, `approve`, `midreq`,
   `memory`, `decision`) and the conditional save inside `loadSpec`. It is an `O_EXCL` lockfile at
   `.specd/specs/<slug>/.lock`, reentrant within a process (so a command's lock + `loadSpec`'s lock
   don't deadlock), with stale-lock reclaim (a lock older than `SPECD_LOCK_STALE_MS`, default 30s, is
   presumed orphaned) and a bounded acquire timeout (`SPECD_LOCK_TIMEOUT_MS`, default 5s) that fails
   loudly with exit 1 on contention rather than blocking forever.
2. **Optimistic revision (compare-and-swap)** — `State.revision` is bumped on every `saveState`.
   Before writing, `saveState` re-reads the on-disk revision; if it no longer matches the loaded
   value, another writer slipped in and the save aborts (exit 1) instead of clobbering. This is
   defense-in-depth: in correctly-locked usage the revisions always match.

**Ledger appends** (`mid-requirements.md`, `memory.md`, `decisions.md`) go through
`io.ts:appendFile`, now a single `O_APPEND` write — the kernel serializes appends to one fd, so
concurrent appends never lose an entry (no read-modify-write window).

`SCHEMA_VERSION` is `2`; the `revision` field arrived in v2 and the `migrate()` branch in `state.ts`
upgrades v1 files in place (default `revision: 0`).

---

## 5. Key invariants (do not break)

These are enforced in code and in `test/`; weakening any of them is a regression.

1. **Evidence gate** (`task.ts`): `--status complete` without `--evidence` → exit 1; with incomplete
   deps → exit 1. No exceptions.
2. **Atomic writes** (`io.ts:atomicWrite`): every state write is temp → `fsync` → `rename(2)`. A
   crash never corrupts `state.json` or `tasks.md`.
3. **Round-trip stability** (`tasksParser.ts`): parse then serialize is byte-stable for valid input.
4. **Sync gate** (`check.ts` gate 6): `tasks.md` checkboxes and `state.json` statuses always match
   after any `specd task` call.
5. **Zero runtime deps**: built CLI runs on Node ≥18 with no install (`npx specd`).
6. **Spec-level VERIFY gate** (`task.ts:deriveStatus`): all-tasks-complete → `verifying`, never
   auto-`complete`. Only `approve` advances `verifying → complete`. It also refuses to regress an
   already-accepted `complete` spec back to `verifying`.
7. **Single writer for `state.json`**: only `state.ts:saveState` writes it, only ever under
   `withSpecLock`.
8. **Exit codes** (`exit.ts`): `0` ok · `1` gate/validation · `2` usage · `3` not found. CI and
   agents branch on these — never change the semantics.

---

## 6. Where to make changes

See the file-map table in [`CLAUDE.md`](../CLAUDE.md) — it lists, per common task, exactly which
source file and test file to edit. The short version: gates live in `commands/check.ts`, the evidence
core in `commands/task.ts`, phase/status derivation in `core/phases.ts`, the DAG in `core/dag.ts`,
concurrency in `core/lock.ts` + `core/state.ts`, and everything emitted into user repos in
`src/templates/`.
