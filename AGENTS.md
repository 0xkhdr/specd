# AGENTS.md

This file provides guidance to coding agents when working with code in this repository.

## What this is

`specd` is a **spec-driven coding harness CLI** (Go, standard library only, zero runtime
dependencies, single static binary). It moves process enforcement out of the LLM's context
window into a deterministic, local, tool-gated pipeline: requirements → design → tasks →
evidence-gated execution. **The agent reasons; the harness enforces.**

Module: `github.com/0xkhdr/specd`. Requires Go 1.22+ (declared min in `go.mod`; toolchain
line pins 1.26).

## Build, test, lint

There is **no root Makefile** (the one under `reference/` belongs to the frozen v1 museum —
see below). Build and test directly:

```bash
go build -o specd .            # single static binary
go run . help                  # run without building

go test ./... -race -count=1                         # full suite (as CI runs it)
go test ./... -count=2                                # F4: catch iteration-order flakiness
go test ./internal/cmd -run TestLifecycleE2E -count=1 # one test by name
```

Lint gates (CI runs each; run before pushing):

```bash
gofmt -l .            # must be empty — CI fails on any unformatted file
go vet ./...
./scripts/test-lint.sh   # test-suite structural lint (no banned suffixes, no space-separated subtest names, no dup helpers)
./scripts/docs-lint.sh   # asserts docs/CHEATSHEET.md mirrors docs/command-reference.md verbatim
# CI also runs gofmt, go vet, go mod tidy check, and the scripts above.
```

Regression harnesses (`scripts/`) re-run every task's `verify:` line and re-assert
each wave's invariant against a freshly built binary in a throwaway tree:

```bash
./scripts/regress-all.sh      # re-run every task verify, aggregate by exit code
./scripts/regress-domains.sh  # per-domain black-box invariant checks
./scripts/regress-lint.sh     # static smell audit of verify tables
```

## Architecture

Entry point `main.go` → `internal/cli` (arg parsing) → `internal/cmd` (dispatch). One handler
per verb lives in `internal/cmd/`; `internal/cmd/registry.go` maps verb → `Handler`. Verbs are
declared once in `internal/core/commands.go`; unknown verbs **fail closed (exit 2)**, deferred
verbs print a deferral notice and exit 0 — they never silently no-op.

Pure domain logic lives in `internal/core/` (no LLM anywhere in these paths):

- **State & storage** — `state.go`, `io.go`, `lock.go`, `paths.go`. Writes go through
  `core.AtomicWrite`; `state.json` mutations use compare-and-swap on a revision counter;
  per-spec work is serialized by a reentrant lock (`core.WithSpecLock`).
- **DAG & execution** — `dag.go`, `frontier.go`, `phases.go`. Tasks form an acyclic DAG;
  the "frontier" is the concurrent set of tasks whose deps are resolved (waves, not lines).
- **Tasks parser** — `tasksparser.go`, byte-stable (round-trips without reformatting).
- **Evidence** — `evidence.go`, `task_complete.go`, `verify/exec.go`. A task completes **only**
  against a passing verify record (exit code 0 pinned to a resolvable git HEAD). There is no
  bypass flag. Read-only tasks carry a trivially-passing verify line (e.g. `printf ok`).
- **Gates** — `internal/core/gates/`: EARS syntax (`ears.go`), design/section checks, task
  schema, acyclic DAG, evidence, sync (`sync.go`), context budget (`contextbudget.go`),
  approval, plus an opt-in security gate (`gates/security/`). `specd check` runs the registry;
  `specd approve` advances a phase only when gates pass.
- **Templates & scaffold** — `embed_templates/` (roles, steering) via `go:embed`, `roles.go`,
  `scaffold.go`; `specd init` scaffolds `.specd/` and writes `AGENTS.md` into the target project.

Other layers:

- `internal/orchestration/` — the opt-in deterministic **brain** controller: leases
  (`lease.go`), decisions (`decide.go`), ACP ledger (`acp.go`), driver/session. Drives
  wave-based execution loops safely without any LLM in the decision path.
- `internal/mcp/` — serves the command palette as a stdio MCP server (`specd mcp`).
- `internal/context/` — builds the bounded, cited context manifest for a single task.
- `internal/integration/` — role/steering snippet registry + conformance tests.

## Runtime surface (in a specd-managed project)

`.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json,.lock}` plus
`.specd/roles/*.md` and `.specd/steering/*.md`. **Note the split:** runtime reads
`.specd/specs/`; this repo's own in-flight planning artifacts live in top-level `specs/`.
`regress-lint.sh` smell "A" exists to catch verify lines that target the wrong one.

Roles constrain what an agent may do: **scout** (read-only explore), **craftsman** (write +
verify, exactly one atomic task per invocation), **validator** (read-only, runs verify line),
**auditor** (read-only, audits a diff against acceptance).

## Non-negotiable invariants (guardrails)

When changing this codebase, preserve these — detail in `docs/contributor-guide.md` §3:

- **Determinism first.** No LLM in any gate, DAG, or report path. They are pure functions of
  on-disk `.specd/` state; reports are generated from `state.json` + task artifacts.
- **Evidence integrity.** No task completes without a passing verify record (exit 0 pinned to a
  real git HEAD). No bypass flag exists — do not add one.
- **Structural invariants.** Atomic writes, CAS on `state.json` revision, reentrant per-spec
  lock, byte-stable tasks parser, `go:embed` templates, **zero runtime dependencies**
  (keep `go.mod`/`go.sum` tidy — CI runs `go mod tidy` and fails on a diff).
- **Subtractive bias.** When unsure, cut or defer and record the decision.
- **Docs sync.** If you touch CLI verbs or flags, update `docs/command-reference.md` **and**
  `docs/CHEATSHEET.md` together (`docs-lint.sh` enforces they match).

## `reference/` — do not touch

`reference/` is the frozen v1 implementation: a read-only museum. Never import, build, copy
from, or edit it. Its `Makefile`, scripts, and docs describe the old system, not this one.

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
