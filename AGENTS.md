# AGENTS.md — How any agent works on this repo

This is the **development repo for `specd`** — a spec-driven coding harness CLI. You are here to
build, fix, or extend the tool itself, not to use it on a project.

---

## What this repo is

`specd` is a deterministic Go CLI + prompt pack that teaches any coding agent to follow
a structured spec workflow (requirements → design → tasks → evidence-gated execution). It writes
no files outside `.specd/` in target repos. Zero runtime dependencies (Go stdlib only). Zero LLM calls.

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
- `scripts/install.sh --force` and `install.sh` verify a release `SHA256SUMS` digest before
  replacing any binary and **fail closed** on mismatch (`install.sh
  --no-verify` opts out loudly).
- `SPECD_*` int env vars go through `core.EnvInt` (clamp + one warning).
- The `.lock` file (`PID epochMillis`) is non-secret; `state.json`/`tasks.md`
  are written `0644` minus umask.

Full detail in `docs/validation-gates.md` → "Security model".

## Build & test

```sh
make build             # go build -ldflags "-s -w -X main.version=..." -o specd .
make test              # go test ./... -race -count=1
make ci                # full local gate: lint + race test + count=2 + coverage floor + stress
go run . <command>     # run from source without building, e.g. go run . status
```

All tests must pass (race detector clean) before any change is considered done. The
full gate is `make ci` (lint, race suite, order-dependence `-count=2`, coverage floor,
cross-process stress). Tests cover:
- Every validation gate (EARS, design, task-schema, DAG, evidence, sync, traceability)
- Parser round-trips (byte-stable `ParseTasksMd`)
- Report rendering (md + html, deterministic — no golden files; assert on content)
- End-to-end lifecycle scenario (init → execute → report)
- Concurrency hardening (per-spec lock, revision CAS, atomic appends, runnable frontier)

See [TESTING.md](TESTING.md) for the deterministic test harness (`internal/testharness`)
and the coverage policy.

---

## Repo layout

```
main.go                       # entry point, arg router, dispatch via cmd.Registry
internal/
  cli/args.go                 # flag/positional parser (Args)
  cmd/                        # one file per CLI command (Run<Command>)
    init.go new.go status.go context.go check.go next.go dispatch.go
    task.go verify.go approve.go decision.go midreq.go memory.go
    report.go waves.go program.go
    registry.go               # command → handler dispatch table (cmd.Registry)
    helpers.go                # shared helpers (specdExit, usageExit, errLine,
                              #   requireRootAndSlug, approvalGateBlocked)
    *_test.go                 # unit tests co-located beside each command
  core/                       # domain logic
    paths.go                  # .specd root locator (FindSpecdRoot, walks up from cwd)
    io.go                     # atomic write (temp + fsync + rename), O_APPEND ledger append
    lock.go                   # per-spec advisory lock (WithSpecLock) for concurrent mutation
    state.go                  # state.json load/save (machine ledger) + revision CAS
    phases.go                 # phase ↔ status single source of truth
    tasksparser.go            # line-based tasks.md parser + serializer (ParseTasksMd)
    dag.go                    # wave DAG, next-runnable, runnable-frontier, cycle detection
    ears.go                   # EARS requirements linter
    report.go                 # md/html assembler (deterministic, no LLM)
    specfiles.go              # artifact accessors, sync + traceability gates, Config
    agents.go                 # AGENTS.md marker-based merge
    commands.go               # CommandMeta registry (drives help + --json schema)
    render.go slug.go md.go ui.go exit.go help.go program.go
    embed.go                  # go:embed of embed_templates/
    embed_templates/          # shipped templates (AGENTS.md, config, steering, roles, stubs)
  testharness/                # deterministic test infra (sandbox, in-process runner, FakeClock)
scripts/                      # install.sh coverage-check.sh stress.sh
```

---

## Key contracts

- **`internal/core/paths.go`** — `FindSpecdRoot` walks up from cwd looking for `.specd/`. All path
  helpers are derived from the root. Callers return `NotFoundError` (exit `3`) if not found.

- **`internal/core/state.go`** — `state.json` is machine truth for task status. Load with
  `LoadState`, write via `SaveState` (atomic + CAS on `revision`). Never hand-edit. Structural
  fields are reconciled from `tasks.md` into state on every load.

- **`internal/cmd/task.go`** — the evidence gate. `--status complete` requires a passing verify
  record (or `--unverified --evidence` for read-only roles) AND all deps `complete`. Dual-writes
  `tasks.md` checkboxes + `state.json` atomically. This is the integrity core — do not weaken it.

- **`internal/core/tasksparser.go`** — bespoke line parser (`ParseTasksMd`). No external libs.
  Round-trip byte-stability is tested. Returns `SpecdError(1)` with a line number on structural errors.

- **Exit codes:** `0` ok · `1` gate/validation failure · `2` usage error · `3` not found. Defined
  in `internal/core/exit.go`. All commands follow this contract; CI branches on it.

---

## Templates are shipped

`internal/core/embed_templates/` is compiled into the binary via `go:embed` in
`internal/core/embed.go` — there are no disk-relative template reads at runtime. If you modify a
template, rebuild before testing.

`internal/core/embed_templates/AGENTS.md` is what gets written into **user repos** by `specd init`
— it is different from this root `AGENTS.md` (which is for developing specd).

---

## Design references

The original `SPEC.md` / `Tasks.md` design documents have been **retired** — the implementation is
now the source of truth. Before making structural changes, read:

- **`docs/contributor-guide.md`** — CLI architecture, concurrency model, and codebase details.
- **`TESTING.md`** — test harness, determinism invariants, and coverage policy.

Source comments cite `SPEC §x` as historical rationale for the retired spec — not a live file.


---

## Working on this repo

- Fix a bug → edit `internal/`, `make build`, `make test`.
- Add/change a gate → edit `internal/cmd/check.go` (+ the gate logic in `internal/core/`) and
  matching tests.
- Add a command → add `internal/cmd/<cmd>.go`, register in `cmd.Registry`
  (`internal/cmd/registry.go`), add a `CommandMeta` in `internal/core/commands.go`, add tests.
  `TestRegistryMatchesHelp` fails if dispatch and help disagree.
- Change the `state.json` shape → update `internal/core/state.go` and add a migration if existing
  files could be misread.

<!-- headroom:rtk-instructions -->
# RTK (Rust Token Killer) - Token-Optimized Commands

When running shell commands, **always prefix with `rtk`**. This reduces context
usage by 60-90% with zero behavior change. If rtk has no filter for a command,
it passes through unchanged — so it is always safe to use.

## Key Commands
```bash
# Git (59-80% savings)
rtk git status          rtk git diff            rtk git log

# Files & Search (60-75% savings)
rtk ls <path>           rtk read <file>         rtk grep <pattern>
rtk find <pattern>      rtk diff <file>

# Test (90-99% savings) — shows failures only
rtk pytest tests/       rtk cargo test          rtk test <cmd>

# Build & Lint (80-90% savings) — shows errors only
rtk tsc                 rtk lint                rtk cargo build
rtk prettier --check    rtk mypy                rtk ruff check

# Analysis (70-90% savings)
rtk err <cmd>           rtk log <file>          rtk json <file>
rtk summary <cmd>       rtk deps                rtk env

# GitHub (26-87% savings)
rtk gh pr view <n>      rtk gh run list         rtk gh issue list

# Infrastructure (85% savings)
rtk docker ps           rtk kubectl get         rtk docker logs <c>

# Package managers (70-90% savings)
rtk pip list            rtk pnpm install        rtk npm run <script>
```

## Rules
- In command chains, prefix each segment: `rtk git add . && rtk git commit -m "msg"`
- For debugging, use raw command without rtk prefix
- `rtk proxy <cmd>` runs command without filtering but tracks usage
<!-- /headroom:rtk-instructions -->
