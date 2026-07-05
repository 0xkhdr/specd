# AGENTS.md — working on specd itself

`specd` is a spec-driven coding harness CLI: Go, standard library only, zero runtime
dependencies, single static binary. This section briefs agents contributing to specd's
own codebase. For the product documentation, start at [README.md](README.md) and
[docs/](docs/); the `<!-- specd:agents -->` block below is the runtime briefing that
`specd init` writes into *user* projects.

Orientation:

- `internal/` + `main.go` — the Go binary. One handler per command under
  `internal/cmd/`; pure domain logic under `internal/core/` (gates, state, DAG, tasks
  parser, config).
- `docs/` — developer-facing docs: concepts, user guide, command reference, validation
  gates, agent integration, contributor guide.
- `specs/` — planning artifacts for in-flight work (e.g. `specs/docs-revamp/spec.md`).
- `reference/` — frozen v1 implementation. Read-only museum: never import, build, or
  copy from it.

Non-negotiable guardrails (detail in `docs/contributor-guide.md` §3):

- **Determinism first** — no LLM sits in any gate, DAG, or report path; they are pure
  functions of on-disk `.specd/` state.
- **Evidence integrity** — no task completes without a passing verify record (exit 0
  pinned to a resolvable git HEAD). There is no bypass flag.
- **ADR-8 invariants** — atomic writes (`core.AtomicWrite`), CAS on the `state.json`
  revision, reentrant per-spec lock (`core.WithSpecLock`), byte-stable tasks parser,
  `go:embed` templates, zero runtime deps.
- **Subtractive bias** — when unsure, cut or defer, and record the decision.

<!-- specd:agents begin -->
# specd — host integration guide

**Agent = Model + Harness.** You (the model) supply reasoning. `specd` (the harness)
makes the plan safely delegable: it owns state, gates, and evidence — deterministically,
with no LLM in its decision path. Read this file before acting on a specd project.

## The loop
1. `specd status` — see the spec, phase, and current task frontier.
2. `specd context <slug> <task>` — get the lean, cited context manifest for one task.
3. Do the task under its **role** (below). Touch only the task's declared `files:`.
4. `specd verify` — record evidence (exit code + git HEAD). This, not your say-so, is
   what marks a task complete.
5. `specd check` — run the readiness gates. `specd approve` advances the phase only if
   they pass.

## Roles (read `.specd/roles/<role>.md` before acting as one)
- 🔍 **scout** — read-only explore & report. Never bound to a write task.
- 🛠️ **craftsman** — write + verify. Exactly one atomic task per invocation.
- 🧪 **validator** — read-only; runs the verify line and reports the record.
- 🛡️ **auditor** — read-only; audits a diff/scope against acceptance.

A task's `role:` determines what it may do. Read-only roles never write and never
fabricate a passing check.

## Guardrails (non-negotiable)
- **Evidence integrity.** No task completes without a passing verify record (exit code 0
  pinned to a real git HEAD). A read-only task carries a verify line it can pass
  (e.g. `printf ok`); there is no flag that bypasses the evidence gate.
- **Determinism.** Gates, DAG, and reports are pure functions of on-disk `.specd/` state.
- **Scope.** Touch only a task's declared files. Record deviations via `specd decision`.
- **Blocked means stop.** Retry once, then report `blocked` with the exact blocker.

## On-disk surface
- `.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json,.lock}`
- `.specd/roles/*.md`, `.specd/steering/*.md` — the role and steering constitutions.

Steering files (`.specd/steering/`) carry the project's reasoning, workflow, product,
tech, and structure rules. Load a steering file when its phase needs it.

<!-- specd:agents end -->


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
