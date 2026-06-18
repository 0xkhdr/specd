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
- `specd update` and `install.sh` verify a release `SHA256SUMS` digest before
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
    report.go waves.go program.go update.go
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
scripts/                      # install.sh uninstall.sh coverage-check.sh stress.sh
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
- Modify templates → edit `internal/core/embed_templates/`, rebuild, verify `specd init` still
  works in a temp dir.
- Change the `state.json` shape → update `internal/core/state.go` and add a migration if existing
  files could be misread.

<!-- SPECD INIT: BEGIN v1 (do not edit between markers) -->
# AGENTS.md — How any agent drives this repo

This repo uses **specd**, an agent-agnostic, spec-driven harness (Kiro spec workflow + structured reasoning). You drive it entirely through the `specd` CLI via your shell tool. No API, plugin, or
MCP is needed — if you can run a shell command, you can run this harness.

## Five rules (non-negotiable)

1. **Load context first.** At the start of every session, read the always-on steering files
   `.specd/steering/{reasoning,workflow,product,tech,structure}.md`. The sixth, `memory.md`, is
   loaded phase-scoped (EXECUTE + REFLECT) — `specd context <spec>` tells you exactly what to load when.

2. **Follow the workflow** in `.specd/steering/workflow.md` — the INTAKE → PERCEIVE → ANALYZE →
   PLAN → EXECUTE → VERIFY → REFLECT lifecycle. Each `→` is a gate.

3. **Mutate state only through `specd`.** Never hand-edit `state.json`. Never flip a `tasks.md`
   checkbox yourself. Use:
   - `specd context <spec>` — phase-scoped briefing: the minimal files to load now + next action.
   - `specd status [<spec>]` — orient ("where am I").
   - `specd next <spec>` — get your next focused task.
   - `specd check <spec>` — before claiming any phase complete (and CI runs it on every push).
   - `specd approve <spec>` — record a human approval: advances the planning phase
     (requirements → design → tasks → executing), or clears a midreq `awaiting-approval` gate.
   - `specd task <spec> <id> --status <s> ...` — the only way to flip a task.

4. **Adopt roles** from `.specd/roles/*` when executing: investigator (read-only research),
   builder (write ONE task), reviewer (read-only audit), verifier (run checks). If your host has
   native subagents and `config.json.roles.subagentMode = "delegate"`, delegate; otherwise run
   the role inline under the same constraints.

5. **Evidence gate.** Never mark a task complete without a passing verify or a manual proof, and
   pass that proof as `--evidence`. A builder's word is not evidence.

## Skills — progressive disclosure

specd ships a skill pack under `.specd/skills/<name>/SKILL.md` — plain Markdown you
read with your shell. Read a stage skill **before** entering that stage and not
before, so you pay context only for the work in front of you.

| Skill | Read when |
|-------|-----------|
| `specd-foundations` | Once per session — the constitution + this index. |
| `specd-steering` | After `init`, before any spec — inspect the repo and author `product/structure/tech.md` + set `config.defaultVerify`. Replaces the old boot/enrich step. |
| `specd-requirements` | Entering the requirements phase (EARS + the `ears` gate). |
| `specd-design` | Entering the design phase (the 7 `design.md` sections + the `design` gate). |
| `specd-tasks` | Entering the tasks phase (wave DAG, 7 task keys, `task-schema`/`dag` gates). |
| `specd-execute` | Entering executing/verifying (the next→verify→complete loop + `evidence` gate). |

## Quickstart

```
specd init                       # scaffold .specd/ + the skill pack (already done if you see this file)
# bootstrap steering: read .specd/skills/specd-steering/SKILL.md, then inspect the
# repo (manifests, dir tree, README, CI) and author product.md / structure.md /
# tech.md and set config.defaultVerify yourself — this replaces the old boot/enrich.
specd new my-feature --title "My Feature"
# write .specd/specs/my-feature/requirements.md (EARS), then:
specd check my-feature           # gate: requirements
specd approve my-feature         # human approves → advances to design
# write design.md, then tasks.md (wave DAG), then:
specd check my-feature           # gate: design + tasks + DAG
specd approve my-feature         # approve design → tasks
specd approve my-feature         # approve tasks  → executing
# execute loop:
specd next my-feature            # -> focused task
specd task my-feature T1 --status complete --evidence "commit abc123; npm test PASS"
# when the last task is done the spec enters `verifying`:
specd approve my-feature         # accept spec-level verification → complete
specd report my-feature          # snapshot
```

## The spec folder

Each feature lives in `.specd/specs/<slug>/` with six artifacts:
`requirements.md` (EARS) · `design.md` · `tasks.md` (wave DAG) · `decisions.md` (ADR) ·
`memory.md` (learnings) · `mid-requirements.md` (feedback log) · plus CLI-owned `state.json`.

The markdown files are your authored truth for *intent*. `state.json` is machine truth for
*status* — the CLI keeps `tasks.md` checkboxes and `state.json` in sync. Do not touch it directly.

<!-- SPECD INIT: END v1 -->


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
