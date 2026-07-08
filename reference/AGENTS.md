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

<!-- SPECD INIT: BEGIN v1 (do not edit between markers) -->
# AGENTS.md — How any agent drives this repo

This repo uses **specd**, an agent-agnostic, spec-driven harness (Kiro spec workflow + structured reasoning). You drive it entirely through the `specd` CLI via your shell tool. No API, plugin, or
MCP is needed — if you can run a shell command, you can run this harness.

**Foundational Split:** specd core is deterministic and makes zero LLM calls — *you* do all
creative thinking, perceiving, and authoring; the harness only scaffolds and enforces gates.
Brain schedules deterministically; it never thinks. Don't ask the core to reason.

## Five rules (non-negotiable)

1. **Load context first.** At session start run `specd handshake bootstrap --json` when available,
   cache command/config digests, then read always-on steering files
   `.specd/steering/{reasoning,workflow,product,tech,structure}.md`. Before acting on a spec run
   `specd handshake policy <spec> --expect-config-digest <cached> --json` and obey its mode/config
   decision. The sixth steering file, `memory.md`, is loaded phase-scoped (EXECUTE + REFLECT) —
   `specd context <spec>` tells you exactly what to load when.

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
   - `specd verify <spec> <id>` — run the task's declared verification command and record its result.
   - `specd task <spec> <id> --status <s> ...` — the only way to flip a task.
   - `specd brain <start|run|step|status|why|directive|pause|resume|cancel> <spec> [flags]` — drive deterministic orchestration and bounded worker directives. (MCP: `specd_brain`)
   - `specd pinky <claim|heartbeat|progress|query|report|block|release|inbox> [flags]` — record deterministic worker leases, telemetry, bounded queries, progress, and terminal reports. (MCP: `specd_pinky`)
   - Windows orchestration is POSIX-only and fails fast with a clear WSL message; non-orchestration workflow remains portable.
   - `specd init [--orchestration <policy>]` — bootstrap and configure the Brain/Pinky orchestration stack.

   MCP hosts: prefer the **intent-level tools** (`brain_orchestrate`, `brain_status`, …);
   `specd_brain`/`specd_pinky` are raw passthrough for flags the intent tools don't surface —
   see `docs/agent-integration.md`.

4. **Adopt roles** from `.specd/roles/*` when executing: scout (read-only research),
   craftsman (write ONE task), auditor (read-only audit), validator (run checks), brain (deterministic
   controller), or pinky (host worker). If your host has native subagents and
   `config.json.roles.subagentMode = "delegate"`, spawn role-bound subagents for implementation
   work: Simple mode uses `specd dispatch --json` packets, Orchestrated mode uses Brain/Pinky
   missions and the scaffolded `.claude/agents/pinky-{craftsman,scout,auditor,validator}.md`
   workers. If the host lacks subagents, say so inline before work and run the role inline under
   the same constraints.

5. **Evidence gate.** Never mark a task complete without a passing verify or a manual proof, and
   pass that proof as `--evidence`. A craftsman's word is not evidence. Pinky completion reports
   must bind to a matching verification record; host-reported telemetry (tokens, cost, duration) is stored as metadata and is not proof of correctness.

## Optional slash/workflow wrappers

Some hosts can map `/init`, `/steer`, `/spec`, and `/pinky-brain` to the shipped
`scripts/specd-workflow.{sh,py}` wrappers. Treat them as UX glue only: `/spec check`
means native `specd check`, `/spec continue` means `specd context` plus `specd next`
when executing, and `/pinky-brain` delegates to Brain/Pinky or read-only status views.
Wrappers never bypass gates, never complete tasks, never edit `state.json`, and never
forge Pinky reports. If wrapper behavior is unclear, use native `specd` directly.

## Execution mode — Simple vs Orchestrated (per spec, user decides)

Every spec records its own **execution mode** in `state.json` (`specd mode <spec>` shows it).
Simple is the default and the broad-compatibility path; orchestration is always an explicit
opt-in. Capability vs selection are distinct: project `orchestration.enabled` only *permits*
orchestration, while a spec's `executionMode` *selects* it.

1. **Default Simple.** "create/build/spec X" → author the spec in Simple mode. Do **not** start
   Brain/Pinky. In Simple you own every step (`specd next` → implement → `specd verify`).
2. **Explicit opt-in → Orchestrated.** "use Pinky and the Brain", "orchestrate this", "run it
   autonomously" → `specd mode <spec> --set orchestrated`, then drive with `specd brain run`.
   Brain/Pinky **refuse** Simple specs, pointing you back here.
3. **Recommend, don't impose.** After `tasks.md` is approved, consult
   `specd mode <spec> --recommend --json`. On `suggest`/`strong`, surface a one-line suggestion
   (e.g. "23 tasks across wide waves — run with Brain/Pinky, or proceed normally?") and **wait
   for the user**. Never switch without a yes; the verdict is advisory (`userDecides: true`).
4. **Respect the recorded mode.** On later actions read `spec.executionMode` and follow it —
   don't re-litigate each turn.

## What loads when

`specd context <spec>` and its `contextManifest` are authoritative for the minimal file set per
phase, including targeted selectors and over-budget actions. This table is a hint, not a
substitute — **re-run `specd context <spec>` each turn; don't trust this from memory**
(phases change what's in scope).

| Phase | Loads (beyond always-on steering) |
|-------|-----------------------------------|
| INTAKE / PERCEIVE / ANALYZE | spec `requirements.md` as it forms |
| PLAN | `requirements.md`, `design.md`, `tasks.md` |
| EXECUTE | `tasks.md`, `memory.md` |
| VERIFY | `tasks.md`, verification records |
| REFLECT | `memory.md`, `decisions.md` |

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
| `specd-eval-author` | Authoring/refining an eval rubric after `specd eval init` (check kinds, scoring, sandboxed `command`). |
| `specd-brain` | Entering orchestration (sensing, deterministic stepping, program scheduling, no-LLM boundary). |
| `specd-pinky` | Operating a Pinky worker (context, claim, heartbeat, progress, query/inbox, blocker, report, release). |

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
# execute loop (manual):
specd next my-feature            # -> focused task
specd verify my-feature T1       # run declared verification and record the result
specd task my-feature T1 --status complete --evidence "commit abc123; npm test PASS"
# execute loop (orchestrated):
# Brain decisions: dispatch -> spawn Pinky; wait -> backoff/step; awaiting-approval -> ask human;
# escalate/policy-violation -> stop and report; complete-session -> final summary.
# Pinky lifecycle: claim -> heartbeat/progress -> verify -> report/block -> release.
# Terminal reports require matching --verification-ref; tokens/cost/duration are telemetry only.
# orchestration defaults (approvalPolicy, maxWorkers, maxRetries, sessionTimeoutMinutes,
# leaseSeconds, …) live in config.json.orchestration; set them via `specd init --orchestration*`.
# Flags below override per-run; omit them to use the configured defaults.
# specd brain start my-feature
# specd pinky claim --mission mission.json
# specd pinky heartbeat --session s --worker w --attempt 1
# specd verify my-feature T1
# specd pinky report --session s --worker w --spec my-feature --task T1 --attempt 1 --verification-ref ref --summary "done"
# specd brain step my-feature --session s
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
