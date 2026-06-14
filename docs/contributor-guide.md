# Contributor Guide

Hacking on `specd` itself — building, the codebase map, key contracts, extension
recipes, and the invariants CI enforces.

## Contents

1. [Building from source](#building-from-source)
2. [Codebase map](#codebase-map)
3. [Key code contracts](#key-code-contracts)
4. [Extending the CLI](#extending-the-cli)
5. [Code-style invariants](#code-style-invariants)

---

## Building from source

No dependencies to install — Go stdlib only; templates are embedded via
`go:embed`.

```bash
make build          # → ./specd  (go build -ldflags "-s -w -X main.version=...")
make install        # install into $GOBIN / $GOPATH/bin
make test           # go test -race ./...
make lint           # go vet ./...
go run . <command>  # run from source without building, e.g. go run . status
```

See [TESTING.md](../TESTING.md) for the deterministic test harness
(`internal/testharness`: sandbox repo, in-process runner, `FakeClock`, fluent
spec builder, assertions).

## Codebase map

```
specd/
├── main.go                       # Entry point, arg router, dispatch switch
├── internal/
│   ├── cli/args.go               # Flag/positional parser (Args)
│   ├── cmd/                      # One file per CLI command (Run<Command>)
│   │   ├── init.go boot.go enrich.go new.go check.go approve.go
│   │   ├── next.go dispatch.go verify.go task.go
│   │   ├── decision.go midreq.go memory.go
│   │   ├── report.go waves.go program.go status.go context.go update.go
│   │   └── helpers.go            # Shared command helpers
│   ├── core/                     # Domain logic
│   │   ├── paths.go              # .specd root locator (FindSpecdRoot)
│   │   ├── io.go                 # Atomic writes (AtomicWrite), O_APPEND ledger
│   │   ├── lock.go               # Per-spec advisory lock (WithSpecLock)
│   │   ├── state.go              # state.json load/save + CAS
│   │   ├── phases.go             # Phase ↔ status mapping, design gate
│   │   ├── tasksparser.go        # Line-based tasks.md parser (ParseTasksMd)
│   │   ├── dag.go                # Wave DAG, frontier, critical path
│   │   ├── ears.go               # EARS requirements linter
│   │   ├── specfiles.go          # Artifact accessors, sync + traceability gates, Config
│   │   ├── boot.go               # Boot manifest + boot-freshness gate
│   │   ├── boot_detectors.go     # Deterministic stack detectors (Go/Node/Py/Rust)
│   │   ├── enrich.go             # Enrich plan/apply contract
│   │   ├── enrich_evidence.go    # Enrich freshness evidence + gate
│   │   ├── agents.go             # AGENTS.md marker-based merge
│   │   ├── commands.go           # CommandMeta registry (help/JSON schema)
│   │   ├── help.go program.go slug.go md.go render.go report.go
│   │   ├── ui.go exit.go         # Output/JSON-mode, exit codes + SpecdError
│   │   ├── embed.go              # go:embed of embed_templates/
│   │   └── embed_templates/      # Shipped templates (AGENTS.md, steering, roles, stubs, skills)
│   └── testharness/              # Deterministic test infrastructure
# Unit tests are co-located as *_test.go beside each source file.
```

## Key code contracts

| File | Contract |
|---|---|
| `internal/core/paths.go` | `FindSpecdRoot` walks up from cwd looking for `.specd/`. Callers return `NotFoundError` (exit `3`) if absent. |
| `internal/core/state.go` | `state.json` is machine truth. Load with `LoadState`, write with `SaveState` (atomic + CAS on `revision`). Never hand-edit. |
| `internal/core/io.go` | `AtomicWrite` (temp + fsync + rename) for every file write; ledgers append with `O_APPEND`. |
| `internal/core/lock.go` | `WithSpecLock[T]` wraps every mutating command in a reentrant per-spec advisory lock. |
| `internal/cmd/task.go` | Evidence gate. `--status complete` requires a passing verify record (or `--unverified --evidence` for read-only roles) AND all deps complete. Dual-writes `tasks.md` + `state.json` atomically. |
| `internal/core/tasksparser.go` | Bespoke line parser (`ParseTasksMd`). No external libs. Round-trip byte-stability tested. Returns `SpecdError(1)` with a line number on errors. |

## Extending the CLI

### Adding a command

1. Create `internal/cmd/mycommand.go` with a handler:
   ```go
   func RunMyCommand(args cli.Args) int {
       // Implementation
       return core.ExitOK
   }
   ```
2. Register it in the `dispatch` switch in `main.go`.
3. Add a `CommandMeta` entry in `internal/core/commands.go` (drives help + `--json` schema).
4. Add a co-located `mycommand_test.go` (or extend `internal/cmd/lifecycle_test.go`).

### Adding a validation gate

1. Write the validation logic in the appropriate `internal/core/*.go` file.
2. Wire it into `internal/cmd/check.go`.
3. Add the gate transition in `internal/cmd/approve.go` if it blocks a phase.
4. Add a test (`internal/cmd/commands_test.go` / `lifecycle_test.go`, or a core `*_test.go`).

## Code-style invariants

1. **Zero runtime dependencies** — `go.mod` lists no `require` deps; stdlib only.
2. **Atomic writes** — use `core.AtomicWrite` (temp + fsync + rename), never raw `os.WriteFile`.
3. **Optimistic concurrency** — load `revision`, verify match, increment on write (CAS).
4. **Reentrant locks** — wrap mutating commands in `core.WithSpecLock`.
5. **Round-trip stability** — `ParseTasksMd` must maintain 100% byte equivalence.
6. **Embedded templates** — ship assets via `go:embed`, never read from disk relative to the binary.
