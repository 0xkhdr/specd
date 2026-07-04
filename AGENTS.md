# AGENTS.md — Operating brief for specd

> **Read `PROJECT.md` first.** It is the single authoritative context document for this
> repository: the product philosophy (Agent = Model + Harness, the eight principles),
> the binding ADRs, the scope triage (29 → 16 verbs), per-domain decisions and
> invariants, the roadmap, and the audited current position with the remaining
> production waves (P0–P6).

Quick orientation:

- `specs/` — the authored per-domain specs (`spec.md` + `tasks.md`) and `progress.md`.
  **Do not trust `progress.md` until it is re-audited (PROJECT.md §8, finding F1).**
- `internal/` + `main.go` — the rebuilt zero-dependency Go binary.
- `reference/` — the frozen v1 implementation. Read-only museum: never import, build,
  or copy from it.
- `The_New_SDLC_With_Vibe_Coding.pdf` — the philosophical anchor.

Non-negotiable guardrails (full detail in PROJECT.md §3): determinism first (no LLM in
any decision/gate/render path), evidence integrity absolute (no completion without a
passing verify record), the ADR-8 hard invariants (atomic writes, CAS on revision,
reentrant lock, parser byte round-trip, embedded templates, zero runtime deps), and
subtractive bias (unsure = CUT/DEFER, recorded).

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
- **Evidence integrity.** No task completes without a passing verify record. The only
  escape hatch is `--unverified --evidence`, for read-only work with no runnable artifact.
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
