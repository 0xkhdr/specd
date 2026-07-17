# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

`specd` is a **spec-driven coding harness CLI** (Go, standard library only, zero runtime
dependencies, single static binary). It moves process enforcement out of the LLM's context
window into a deterministic, local, tool-gated pipeline: requirements → design → tasks →
evidence-gated execution. **The agent reasons; the harness enforces.**

Module: `github.com/0xkhdr/specd`. Requires Go 1.26+ (the `go` directive in `go.mod`).

## Build, test, lint

There is **no Makefile**. Build and test directly:

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

The regression harness (`scripts/regress-domains.sh`) re-asserts each domain's
invariant black-box against a freshly built binary in a throwaway tree:

```bash
./scripts/regress-domains.sh  # per-domain black-box invariant checks
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
`.specd/roles/*.md` and `.specd/steering/*.md`. Runtime state always lives under
`.specd/` inside the managed project — never in this repository's own tree.

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
  (there is no `go.sum` — nothing to sum; CI runs `go mod tidy` and fails on any `go.mod` diff).
- **Subtractive bias.** When unsure, cut or defer and record the decision.
- **Docs sync.** If you touch CLI verbs or flags, update `docs/command-reference.md` **and**
  `docs/CHEATSHEET.md` together (`docs-lint.sh` enforces they match).

