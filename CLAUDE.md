# CLAUDE.md — Context for Claude Code

This repo is the development home of **`specd`** — an agent-agnostic, spec-driven coding harness.
You are here to develop the tool, not to use it on a project.

## Build & test (run these constantly)

```sh
npm run build          # tsc → dist/ + copy templates. Always do this before npm test.
npm test               # 65 tests, must all pass
node --import tsx src/cli.ts <command>   # run from source (no build needed)
```

## Project identity

- **Package:** `specd` (npm), binary `specd`
- **Harness root in user repos:** `.specd/`  (not `.fable/` — that was the old name)
- **Error class:** `SpecdError`  (not `FspecError`)
- **Path helpers:** `findSpecdRoot`, `requireSpecdRoot`, `specdDir`
- **Methodology file:** `reasoning.md`  (not `fable5.md`)
- The tool was renamed from `fspec`/`fable` to `specd`. Never reintroduce the old names.

## Architecture in one paragraph

> Full walkthrough (philosophy→code map, lifecycle sequence, concurrency model, invariants):
> [`docs/ARCHITECTURE.md`](docs/ARCHITECTURE.md).

`src/cli.ts` parses args and dispatches to `src/commands/*.ts`. Every command that needs a repo
calls `requireSpecdRoot()` to locate `.specd/` by walking up from cwd. All state lives in
`state.json` (machine truth) + `tasks.md` checkboxes (human truth); `specFiles.ts:reconcile()`
keeps them in sync on every load. `src/commands/task.ts` is the integrity core: it enforces the
evidence gate and dual-writes both files atomically. `src/core/tasksParser.ts` is a bespoke
line-based parser (no AST libs). The six reasoning phases map to spec status via
`phases.ts:phaseForStatus()` (single source of truth); when every task completes the spec enters
the spec-level VERIFY gate (`status: verifying`) and only `specd approve` advances it to `complete`.
Templates in `src/templates/` are copied to `dist/templates/` at build time and read at runtime via
`templatesDir()`.

## File map (what to edit for common tasks)

| Task | Files |
|---|---|
| Fix/add a validation gate | `src/commands/check.ts`, `test/check.test.ts` |
| Change evidence/dep logic | `src/commands/task.ts`, `test/task.test.ts` |
| Change phase/status derivation or the VERIFY gate | `src/core/phases.ts`, `src/commands/task.ts`, `src/commands/approve.ts` |
| Change the phase-scoped context briefing | `src/commands/context.ts` |
| Fix `.specd/` root discovery | `src/core/paths.ts` |
| Add a CLI command | `src/commands/<cmd>.ts` + register in `src/cli.ts` dispatch + USAGE |
| Change `state.json` schema | `src/core/state.ts` (bump `SCHEMA_VERSION`, add a `migrate()` branch) |
| Fix tasks.md parsing | `src/core/tasksParser.ts`, `test/tasksParser.test.ts` |
| Fix DAG / next-task / runnable-frontier logic | `src/core/dag.ts`, `test/dag.test.ts` |
| Change concurrency locking / revision CAS | `src/core/lock.ts`, `src/core/state.ts`, `test/concurrency.test.ts` |
| Change AGENTS.md template | `src/templates/AGENTS.md` (emitted into user repos by `specd init`) |
| Change steering/role prompts | `src/templates/steering/` or `src/templates/roles/` |
| Fix report rendering | `src/core/report.ts`, `src/commands/report.ts`, `test/report.test.ts` |

## Exit codes (non-negotiable)

`0` ok · `1` gate/validation failure · `2` usage error · `3` not found.
Defined in `src/core/exit.ts`. CI and agents branch on these — don't change semantics.

## Test helpers

`test/helpers.ts` exports `run(cwd, ...argv)` which captures stdout/stderr and the exit code by
swapping `console.log`/`process.chdir`. Use it for all integration tests. Use `newTmp()` for
isolated temp dirs.

## Templates vs root AGENTS.md

- `src/templates/AGENTS.md` — gets written into **user repos** by `specd init`. Edit this to
  change what agents in user repos see.
- `/AGENTS.md` (repo root, this file's sibling) — tells agents how to **develop** specd. They
  are completely separate.

## Key invariants — do not break

1. **Evidence gate** (`src/commands/task.ts`): `--status complete` without `--evidence` → exit 1.
   `--status complete` with incomplete deps → exit 1. No exceptions.
2. **Atomic writes** (`src/core/io.ts`): every write goes temp → rename. A crash must never
   corrupt `state.json` or `tasks.md`.
3. **Round-trip stability** (`src/core/tasksParser.ts`): parse then serialize must be byte-stable
   for valid input. The test in `test/tasksParser.test.ts` enforces this.
4. **Sync gate** (`src/commands/check.ts` gate 6): `tasks.md` checkboxes and `state.json` statuses
   must always match after any `specd task` call.
5. **Zero external deps at runtime**: `package.json` has only `devDependencies`. The built CLI
   runs on Node ≥18 with no install step in user repos (`npx specd`).
6. **Spec-level VERIFY gate** (`src/commands/task.ts:deriveStatus`): when every task is complete the
   spec enters `status: verifying` (phase `verify`), **not** `complete`. Only `specd approve`
   advances `verifying → complete`. Never auto-complete a spec.
7. **Per-spec lock around mutation** (`src/core/lock.ts`): every load→mutate→save critical section
   runs under `withSpecLock` (the lock is reentrant, so `loadSpec` nesting is fine). `state.revision`
   compare-and-swap in `saveState` is the defense-in-depth backstop. Never mutate `state.json`
   outside the lock.

## Spec and task history

- `SPEC.md` / `Tasks.md` — the original design spec and build checklist. **These files are no longer
  in the repo.** Source comments still cite section numbers (`SPEC §5.2`, `§10`, etc.) as historical
  references to that spec; treat them as design rationale, not a live file to open. Implementation is
  complete and is itself the source of truth now.
