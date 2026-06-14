# AGENTS.md — How any agent works on this repo

This is the **development repo for `specd`** — a spec-driven coding harness CLI. You are here to
build, fix, or extend the tool itself, not to use it on a project.

---

## What this repo is

`specd` is a deterministic TypeScript CLI + prompt pack that teaches any coding agent to follow
a structured spec workflow (requirements → design → tasks → evidence-gated execution). It writes
no files outside `.specd/` in target repos. Zero runtime dependencies. Zero LLM calls.

---

## Security model

`tasks.md` is **agent-authored input**, not trusted config — treat every
`verify:` line and env var as hostile until validated.

- `specd verify` executes `verify:` lines via `sh -c` (override:
  `SPECD_VERIFY_SHELL`) as the invoking user. This is intentional code
  execution — only run it on trusted `tasks.md`. The child env is scrubbed to
  an allowlist (`PATH`, `HOME`, `LANG`, `LC_ALL`, `TMPDIR`, `SPECD_*`), NUL
  bytes are rejected, and the command + cwd are printed before running.
- Spec slugs are path-validated (`^[a-z0-9][a-z0-9-]*$`) — no traversal.
- `specd update` and `install.sh` verify a release `SHA256SUMS` digest before
  replacing any binary and **fail closed** on mismatch (`install.sh
  --no-verify` opts out loudly).
- `SPECD_*` int env vars go through `core.EnvInt` (clamp + one warning).
- The `.lock` file (`PID epochMillis`) is non-secret; `state.json`/`tasks.md`
  are written `0644` minus umask.

Full detail in `docs/validation-gates.md` → "Security model".

## Build & test

```sh
npm run build          # tsc → dist/  +  copy src/templates → dist/templates
npm test               # node --test over test/*.test.ts + test/e2e/*.test.ts  (65 tests)
node --import tsx src/cli.ts <command>   # run from source without building
```

All 65 tests must pass before any change is considered done. Tests cover:
- Every validation gate (EARS, design, task-schema, DAG, evidence, sync, traceability)
- Parser round-trips
- Report golden files (md + html)
- End-to-end lifecycle scenario (init → execute → report)
- Concurrency hardening (per-spec lock, revision CAS, atomic appends, runnable frontier)

---

## Repo layout

```
src/
  cli.ts               # arg router, exit codes, dispatch table
  commands/            # one file per CLI command
    init.ts            # specd init — scaffold .specd/ + AGENTS.md
    new.ts             # specd new <slug>
    status.ts          # specd status
    context.ts         # specd context — phase-scoped briefing
    check.ts           # specd check — all 7 gates
    next.ts            # specd next — next runnable task (--all = frontier)
    task.ts            # specd task — evidence gate + dual-write
    approve.ts         # specd approve — planning ratchet + VERIFY gate
    decision.ts        # specd decision — ADR append
    midreq.ts          # specd midreq — feedback log
    memory.ts          # specd memory — learnings + promote
    report.ts          # specd report — md/html snapshot
    waves.ts           # specd waves — wave DAG view
  core/
    paths.ts           # .specd root locator (walks up from cwd)
    io.ts              # atomic write (temp + rename), O_APPEND ledger append
    lock.ts            # per-spec advisory lock (withSpecLock) for concurrent mutation
    state.ts           # state.json load/save (machine ledger) + revision CAS
    phases.ts          # phase ↔ status single source of truth
    tasksParser.ts     # line-based tasks.md parser + serializer
    dag.ts             # wave DAG, next-runnable, runnable-frontier, cycle detection
    ears.ts            # EARS requirements linter
    report.ts          # md/html assembler (deterministic, no LLM)
    specFiles.ts       # artifact accessors, reconcile tasks↔state
    render.ts          # wave graph text renderer
    templates.ts       # template loader + variable substitution
    exit.ts            # SpecdError, exit code constants
    md.ts              # markdown helpers (strip HTML comments)
  templates/           # shipped as dist/templates/ at runtime
    AGENTS.md          # emitted into user repos by specd init
    config.json        # default config scaffold
    steering/          # reasoning.md, workflow.md, product/tech/structure/memory.md
    roles/             # investigator, builder, reviewer, verifier prompt files
    specStubs/         # six artifact stubs (requirements, design, tasks, etc.)
test/
  helpers.ts           # run() harness — captures stdout/stderr, swaps cwd
  core.test.ts         # paths, io, state
  dag.test.ts          # DAG engine
  ears.test.ts         # EARS linter
  tasksParser.test.ts  # parser round-trips
  check.test.ts        # all 7 gate fixtures
  report.test.ts       # golden-file render tests
  e2e/full-spec.test.ts  # full lifecycle + evidence-gate enforcement
scripts/
  copy-templates.mjs   # post-build: copies src/templates → dist/templates
```

---

## Key contracts

- **`src/core/paths.ts`** — `findSpecdRoot` walks up from cwd looking for `.specd/`. All path
  helpers (`specdDir`, `steeringDir`, `rolesDir`, `specsDir`, `specDir`) are derived from the
  root. `requireSpecdRoot` throws `SpecdError(3)` if not found.

- **`src/core/state.ts`** — `state.json` is machine truth for task status. Write via `saveState`
  (atomic). Never hand-edit. `reconcile()` in `specFiles.ts` syncs structural fields from
  `tasks.md` into state on every load.

- **`src/commands/task.ts`** — the evidence gate. `--status complete` requires non-empty
  `--evidence` AND all deps `complete`. Dual-writes `tasks.md` checkboxes + `state.json`
  atomically. This is the integrity core — do not weaken it.

- **`src/core/tasksParser.ts`** — bespoke line parser for the §7.3 format. No AST libs. Round-trip
  stability is tested. Throws `SpecdError(1)` with line number on structural errors.

- **Exit codes:** `0` ok · `1` gate/validation failure · `2` usage error · `3` not found. Defined
  in `src/core/exit.ts`. All commands follow this contract; CI branches on it.

---

## Templates are shipped

`src/templates/` is copied verbatim to `dist/templates/` at build time
(`scripts/copy-templates.mjs`). The CLI reads templates from `dist/templates/` at runtime via
`templatesDir()` in `paths.ts`. If you modify a template, rebuild before testing.

`src/templates/AGENTS.md` is what gets written into **user repos** by `specd init` — it is
different from this root `AGENTS.md` (which is for developing specd).

---

## Design references

The original `SPEC.md` / `Tasks.md` design documents have been **retired** — the implementation is
now the source of truth. Before making structural changes, read:

- **`docs/contributor-guide.md`** — CLI architecture, concurrency model, and codebase details.
- **`CLAUDE.md`** — contributor guidelines, build/test commands, and code style invariants.

Source comments cite `SPEC §x` as historical rationale for the retired spec — not a live file.


---

## Working on this repo

- Fix a bug → edit `src/`, `npm run build`, `npm test`.
- Add/change a gate → edit `src/commands/check.ts` + matching test fixtures in `test/check.test.ts`.
- Add a command → add `src/commands/<cmd>.ts`, register in the `dispatch` switch in `src/cli.ts`,
  add to the `USAGE` string, add tests.
- Modify templates → edit `src/templates/`, rebuild, verify `specd init` still works in a
  temp dir.
- Change the `state.json` shape → update `src/core/state.ts` and add a migration if existing
  files could be misread.
