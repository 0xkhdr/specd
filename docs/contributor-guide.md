# specd — Contributor Guide

Codebase walkthrough, non-negotiable invariants, and extension recipes. Read
[concepts.md](concepts.md) first for the *why*.

## Build, test, lint

There is **no Makefile**.
Build and test directly:

```bash
go build -o specd .                                    # single static binary
go run . help                                          # run without building

go test ./... -race -count=1                           # full suite (as CI runs it)
go test ./... -count=2                                 # catch iteration-order flakiness
go test ./internal/cmd -run TestLifecycleE2E -count=1  # one test by name
```

Lint gates (CI runs each — run before pushing):

```bash
gofmt -l .              # must be empty — CI fails on any unformatted file
go vet ./...
go mod tidy             # must produce no diff (zero runtime deps)
./scripts/test-lint.sh  # test-suite structural lint
./scripts/docs-lint.sh  # asserts docs/CHEATSHEET.md mirrors docs/command-reference.md verbatim
```

The regression harness re-asserts each domain's invariant black-box against a freshly built
binary in a throwaway tree:

```bash
./scripts/regress-domains.sh  # per-domain black-box invariant checks
```

## Architecture

Entry: `main.go` → `internal/cli` (arg parsing) → `internal/cmd` (dispatch). One handler per
verb lives in `internal/cmd/`; `internal/cmd/registry.go` maps verb → `Handler`. Verbs are
declared once in `internal/core/commands.go`; unknown verbs **fail closed (exit 2)**, deferred
verbs print a deferral notice and exit 0.

### Domain map

| Domain | Packages / key files | Owns |
|---|---|---|
| **CLI & dispatch** | `main.go`, `internal/cli`, `internal/cmd` (`registry.go`, `dispatch.go`, `lifecycle.go`) | Arg parsing, verb→`Handler` map, phase enforcement, fail-closed on unknown verbs, deferral notices. |
| **Command palette (SOT)** | `internal/core/commands.go` | `var Commands` — every verb, flag, exit code, allowed phase, examples. Feeds help/dispatch/MCP/roles. `HelpSchemaVersion`. |
| **State & storage** | `state.go`, `io.go`, `lock.go`, `paths.go` | `core.AtomicWrite`, CAS on `state.json` revision, reentrant `core.WithSpecLock`, path resolution. |
| **Lifecycle / phases** | `phases.go` | Statuses → phases (`perceive→…→reflect`), forward-only advance. |
| **DAG & execution** | `dag.go`, `frontier.go` | Acyclic task DAG; the concurrent runnable frontier (waves). |
| **Tasks parser** | `tasksparser.go` (+ fuzz) | Byte-stable round-trip parse of `tasks.md`. |
| **Evidence & verify** | `evidence.go`, `task_complete.go`, `verify/exec.go`, `criteria.go` | Verify records (exit code + git HEAD); task completion; per-criterion evidence. |
| **Gates** | `internal/core/gates/` + `gates/security/` | The 22 core gates + opt-in security gate. |
| **Templates & scaffold** | `internal/core/embed_templates/`, `roles.go`, `scaffold.go`, `managed.go` | `init`/`new` scaffolding; managed-region repair/refresh; `AGENTS.md` emission. |
| **Config** | `config_loader.go`, `config_validate.go` | Effective config, digests (handshake). |
| **Memory** | `memory.go` | Append/promote steering-memory patterns. |
| **Program / cross-spec** | `program.go`, `commitlink.go` | `link`/`unlink`, cross-spec dep ordering, program view. |
| **Reporting** | `report.go`, `report_metrics.go`, `prometheus.go`, `prsummary.go`, `history.go`, `telemetry.go` | Deterministic status/PR/metrics/history reports; Prometheus textfile; token/cost ledger. |
| **Handshake** | `handshake.go`, `manifest_tools.go`, `mcpconfig.go` | Bootstrap material, palette/config digests. |
| **Orchestration** | `internal/orchestration/` (`lease.go`, `decide.go`, `acp.go`, `driver.go`, `session.go`, `brakes.go`, `checkpoint.go`, `recover.go`, `authority.go`, `sense.go`) | Opt-in deterministic controller — no LLM in the decision path. |
| **MCP server** | `internal/mcp/` | Serves the palette as a stdio MCP server. |
| **Context manifest** | `internal/context/` | Bounded, cited per-task context. |
| **Integration** | `internal/integration/` | Role/steering snippet registry + conformance tests. |

The machine context manifest is additive and opt-in. Keep the human-readable output stable.
Machine-manifest required lanes must
resolve beneath the canonical root; required overflow, unknown schema/trust/item values, stale
receipts, and route/capability identity mismatch fail closed. Receipts contain digests and totals,
never content or secrets. Treat skills, memory, requirements, and source text as untrusted data;
only harness metadata carries authority.

Quality context/report projections keep proof, gaps, stale evidence, scores, and review distinct.
Ledger ingestion validates redaction first, appends immutable source-digest references, and
never stores raw datasets, traces, prompts, or provider output.

Host capability negotiation is deterministic and explicit. `initialize` reports every driver key
(`context_loading`, `sandbox`, `telemetry`, `eval`, `a2a`); optional gaps downgrade to local
behavior, while missing sandbox refuses mutable execution with recovery guidance. Do not add a
host path that silently omits an unsupported capability.

### Runtime `specs/` location

**Runtime** reads `.specd/specs/` inside a managed project — runtime state never
lives in this repository's own tree.

## Non-negotiable invariants

Preserve these when changing the codebase:

1. **Determinism first.** No LLM in any gate, DAG, or report path. They are pure functions of
   on-disk `.specd/` state.
2. **Evidence integrity.** No task completes without a passing verify record (exit 0 pinned to
   a real git HEAD). **No bypass flag exists — do not add one.**
3. **Structural invariants.** Atomic writes, CAS on the `state.json` revision, reentrant
   per-spec lock, byte-stable tasks parser, `go:embed` templates, **zero runtime dependencies**
   (there is no `go.sum` — nothing to sum; CI runs `go mod tidy` and fails on any `go.mod` diff).
4. **Subtractive bias.** When unsure, cut or defer and record the decision.
5. **Docs sync.** If you touch CLI verbs or flags, update `docs/command-reference.md` **and**
   `docs/CHEATSHEET.md` together (`docs-lint.sh` enforces byte-identical match).

## Concurrency & durability model

- **Atomic writes.** All state writes go through `core.AtomicWrite` (write temp + rename), so
  a crash never leaves a half-written `state.json`.
- **CAS on revision.** `state.json` carries a revision counter; mutations compare-and-swap on
  it, so a concurrent writer that raced loses cleanly instead of clobbering.
- **Reentrant per-spec lock.** `core.WithSpecLock` serializes all per-spec work; reentrant so
  nested calls within one goroutine don't deadlock.
- **Byte-stable parser.** `tasksparser.go` round-trips `tasks.md` without reformatting, so a
  parse+write is a no-op diff — hand edits and tool edits coexist.

## Extension recipes

### Add a verb

1. Append a `Command` entry to `var Commands` in `internal/core/commands.go` (name, usage,
   description, flags, `AllowedPhases`, `ExitCodes: stdCodes()`, at least one example,
   `SpecSlugArg` if it phase-checks a spec).
2. Add the handler in `internal/cmd/` and register it in `registry.go`.
3. Update `docs/command-reference.md` **and** copy it to `docs/CHEATSHEET.md`; run
   `./scripts/docs-lint.sh`.
4. Add tests; the handler-parity test asserts every non-deferred verb has a handler.

### Add a gate

1. Write a `func(CheckCtx) []Finding` in `internal/core/gates/`.
2. `registry.Register(gateFunc{name: "...", run: ...})` in `CoreRegistry()`.
3. Keep it **pure** — read nothing from disk; take everything through `CheckCtx`. Zero-valued
   inputs must disable it (parity: an empty `CheckCtx` yields no findings).
4. Document it in `docs/validation-gates.md`.

## `reference/` — do not touch

`reference/` is the frozen v1 implementation: a read-only museum. **Never import, build, copy
from, or edit it.** Its `Makefile`, scripts, and docs describe the old system, not this one —
they document features that do not exist in the current binary.
