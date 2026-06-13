# 9. Contributing

For developers working on **`specd` itself** (not using it on a project). Read this alongside
[Architecture](ARCHITECTURE.md) (how the harness works) and
[`../specd-core-philosophy.md`](../specd-core-philosophy.md) (why it works that way).

## Identity — get the names right

The tool was renamed from `fspec`/`fable` to **`specd`**. Never reintroduce the old names.

| Concept | Current | Old (never use) |
|---------|---------|-----------------|
| Package / binary | `specd` | `fspec`, `fable` |
| Harness root in user repos | `.specd/` | `.fable/` |
| Error class | `SpecdError` | `FspecError` |
| Path helpers | `findSpecdRoot`, `requireSpecdRoot`, `specdDir` | — |
| Methodology file | `reasoning.md` | `fable5.md` |

## Dev setup

```sh
npm install
npm run build      # tsc → dist/ + copy src/templates → dist/templates
npm test           # node --test over all gates, parser round-trip, report, e2e (65 tests)
```

Run from source without building:

```sh
node --import tsx src/cli.ts <command>
```

Always `npm run build` before `npm test` if you touched templates — they are read from
`dist/templates/` at runtime.

## Repo layout

```
src/
  cli.ts               # arg router, exit-code contract, dispatch table
  commands/            # one file per CLI command (init, new, status, context, check,
                       #   next, task, approve, decision, midreq, memory, report, waves)
  core/
    paths.ts           # .specd root locator (walks up from cwd)
    io.ts              # atomic write (temp + fsync + rename), O_APPEND ledger append
    lock.ts            # per-spec advisory lock (withSpecLock)
    state.ts           # state.json load/save + revision CAS + schema migration
    phases.ts          # phase ↔ status single source of truth + design/planning gates
    tasksParser.ts     # bespoke line-based tasks.md parser + serializer
    dag.ts             # wave DAG, next-runnable, frontier, cycle/orphan/wave-order detection
    ears.ts            # EARS requirements linter
    report.ts          # md/html assembler (deterministic, no LLM)
    specFiles.ts       # artifact accessors, loadSpec, reconcile tasks↔state
    render.ts          # shared text renderers (wave graph, counts, next summary)
    templates.ts       # template loader + variable substitution
    exit.ts            # SpecdError, exit-code constants
    md.ts              # markdown helpers (strip HTML comments)
  templates/           # shipped to dist/templates/ at build time
    AGENTS.md          # emitted into user repos by specd init
    config.json        # default config scaffold
    steering/          # reasoning, workflow, product, tech, structure, memory
    roles/             # investigator, builder, reviewer, verifier
    specStubs/         # the six artifact stubs
test/
  helpers.ts           # run(cwd, ...argv) harness; newTmp() for isolated temp dirs
  core.test.ts · dag.test.ts · ears.test.ts · tasksParser.test.ts
  check.test.ts · task.test.ts · approve.test.ts · report.test.ts
  concurrency.test.ts · hardening.test.ts
  e2e/full-spec.test.ts  # full lifecycle + evidence-gate enforcement
scripts/
  copy-templates.mjs   # post-build: copies src/templates → dist/templates
```

## File map — what to edit for a common task

| Task | Files |
|------|-------|
| Fix/add a validation gate | `src/commands/check.ts`, `test/check.test.ts` |
| Change evidence/dependency logic | `src/commands/task.ts`, `test/task.test.ts` |
| Change phase/status derivation or the VERIFY gate | `src/core/phases.ts`, `src/commands/task.ts`, `src/commands/approve.ts` |
| Change the phase-scoped context briefing | `src/commands/context.ts` |
| Fix `.specd/` root discovery | `src/core/paths.ts` |
| Add a CLI command | `src/commands/<cmd>.ts` + register in `src/cli.ts` dispatch + USAGE + tests |
| Change `state.json` schema | `src/core/state.ts` (bump `SCHEMA_VERSION`, add a `migrate()` branch) |
| Fix `tasks.md` parsing | `src/core/tasksParser.ts`, `test/tasksParser.test.ts` |
| Fix DAG / next-task / frontier logic | `src/core/dag.ts`, `test/dag.test.ts` |
| Change concurrency locking / revision CAS | `src/core/lock.ts`, `src/core/state.ts`, `test/concurrency.test.ts` |
| Change what user repos receive | `src/templates/*` (rebuild before testing) |
| Fix report rendering | `src/core/report.ts`, `src/commands/report.ts`, `test/report.test.ts` |

## Adding a command — the checklist

1. Create `src/commands/<cmd>.ts` exporting `run(args: Args): number | Promise<number>`.
2. Call `requireSpecdRoot()` first; throw `usageError(...)` for bad args.
3. Wrap any load → mutate → save in `withSpecLock(root, slug, () => { … })`.
4. Register the command in the `dispatch` switch in `src/cli.ts`.
5. Add a line to the `USAGE` string.
6. Add integration tests in `test/` using the `run()` helper.
7. Honor the exit-code contract.

## Test helpers

`test/helpers.ts` exports `run(cwd, ...argv)` — it captures stdout/stderr and the exit code by
swapping `console.log`/`process.chdir`. Use it for all integration tests. Use `newTmp()` for isolated
temp dirs. All **65** tests must pass before a change is done.

## Key invariants — do not break

These are enforced in code and `test/`; weakening any is a regression.

1. **Evidence gate** (`task.ts`) — `--status complete` without `--evidence`, or with incomplete deps,
   → exit 1. No exceptions.
2. **Atomic writes** (`io.ts`) — every state write is temp → `fsync` → `rename`. A crash never
   corrupts `state.json` or `tasks.md`.
3. **Round-trip stability** (`tasksParser.ts`) — parse then serialize is byte-stable for valid input.
4. **Sync gate** (`check.ts` gate 6) — `tasks.md` checkboxes and `state.json` statuses always match
   after any `specd task` call.
5. **Zero runtime deps** — `package.json` has only `devDependencies`; the built CLI runs on Node ≥18
   with no install (`npx specd`).
6. **Spec-level VERIFY gate** (`task.ts:deriveStatus`) — all-tasks-complete → `verifying`, never
   auto-`complete`; only `approve` advances `verifying → complete`; never regress an accepted
   `complete` spec.
7. **Single writer for `state.json`** — only `state.ts:saveState` writes it, only ever under
   `withSpecLock`.
8. **Exit codes** (`exit.ts`) — `0` ok · `1` gate/validation · `2` usage · `3` not found. CI and
   agents branch on these; never change the semantics.

## Templates are shipped, not imported

`src/templates/` is copied verbatim to `dist/templates/` at build (`scripts/copy-templates.mjs`) and
read at runtime via `templatesDir()`. **Rebuild after editing any template.** Note the two distinct
`AGENTS.md` files:

- `src/templates/AGENTS.md` → written into **user repos** by `specd init`.
- `/AGENTS.md` (repo root) → tells agents how to **develop** specd.

They are completely separate; do not unify them.

## Design references

The original `SPEC.md` / `Tasks.md` design documents have been **retired** — the implementation is
now the source of truth. Source comments still cite section numbers (`SPEC §5.2`, `§10`, …) as
historical rationale for that retired spec; treat them as design notes, not a live file to open.

Before structural changes, read [Architecture](ARCHITECTURE.md) (philosophy→code map, lifecycle,
concurrency, invariants) and [`../specd-core-philosophy.md`](../specd-core-philosophy.md) (the eight
principles).
