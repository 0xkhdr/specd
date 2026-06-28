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
├── main.go                       # Entry point, arg router, dispatch via cmd.Registry
├── internal/
│   ├── cli/args.go               # Flag/positional parser (Args)
│   ├── cmd/                      # One file per CLI command (Run<Command>)
│   │   ├── init.go new.go check.go approve.go
│   │   ├── next.go dispatch.go verify.go task.go
│   │   ├── decision.go midreq.go memory.go
│   │   ├── report.go waves.go program.go status.go context.go update.go
│   │   ├── registry.go           # Command → handler dispatch table (cmd.Registry)
│   │   └── helpers.go            # Shared command helpers (specdExit, usageExit, errLine)
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
| `internal/core/config_loader.go` | Config discovery/cascade: embedded defaults → global YAML/legacy JSON → project YAML/legacy JSON → env overlay. `LoadConfig` is the compatibility wrapper; use `LoadConfigWithDiagnostics`/`LoadConfigStrict` for source metadata or validation. |
| `internal/core/config_migrate.go` | Deterministic legacy JSON → YAML v2 renderer/migration, including parse round-trip validation. |

### Config loader architecture

Human-authored config defaults to YAML v2 (`.specd/config.yml` and `$XDG_CONFIG_HOME/specd/config.yml`), while legacy v1 JSON remains readable through compatibility wrappers. The loader deliberately avoids external dependencies: `parseSimpleYAML` accepts the subset emitted by `RenderConfigYAML` and documented examples. New fields must be added in four places: the `Config` struct/defaults, `applyConfigDoc` camelCase + snake_case translation, validation (`ValidateConfigDoc` and/or `ValidateEffectiveConfig`), and migration rendering/tests.

Merge semantics are field-presence based: absent fields leave the lower layer intact, explicit `false`/`0` values apply, and slices replace lower-layer slices. After file merge, `applyConfigEnv` applies supported `SPECD_*` overrides as a final layer and emits diagnostics with only env var name + target field. Never dump raw environment blocks into JSON output.

Security boundaries: config is untrusted policy input. Validate authority-bearing orchestration values after all layers, keep secret-shaped orchestration keys rejected, and do not add env vars that smuggle shell commands, tokens, provider names, models, or credentials into orchestration policy. Machine state files stay JSON; YAML migration must not rename `state.json`, `.specd/program.json`, runtime `session.json`, or integration JSON.

## Extending the CLI

### Adding a command

1. Create `internal/cmd/mycommand.go` with a handler:
   ```go
   func RunMyCommand(args cli.Args) int {
       // Implementation
       return core.ExitOK
   }
   ```
2. Add it to `cmd.Registry` in `internal/cmd/registry.go` (the dispatch table).
3. Add a `CommandMeta` entry in `internal/core/commands.go` (drives help + `--json` schema).
4. Add a co-located `mycommand_test.go` (or extend `internal/cmd/lifecycle_test.go`).

`TestRegistryMatchesHelp` fails if steps 2 and 3 disagree, so dispatch and help
can never drift. Emit machine output via `core.PrintJSON` (lists non-nil → `[]`),
return a `core.Exit*` constant, and send diagnostics to stderr (`core.Error` /
`errLine`). See [command reference](./command-reference.md#output-streams).

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

## Decision: custom CLI parser, not Cobra/urfave

specd keeps its own ~40-line argument parser (`internal/cli/args.go`) and a flat
dispatch table (`cmd.Registry`) instead of adopting Cobra, urfave/cli, or Viper.
This is a conscious, recorded choice — not drift:

- **Zero runtime dependencies is a product value.** `go.mod` lists no `require`
  entries; Cobra pulls in a transitive tree (pflag, etc.) that contradicts the
  "stdlib only" invariant.
- **Determinism.** The harness enforces; the parser must be predictable. Cobra's
  flag/help magic and global state fight the reproducibility the gates depend on.
- **Small, stable surface.** ~19 commands, a fixed flag grammar. The custom
  parser is fully covered by `args_test.go`, including a registration guard.
- **Help cannot drift** without a framework: `cmd.Registry` (dispatch) and
  `core.Commands` (help metadata) are tied by `TestRegistryMatchesHelp`, giving
  the single-source-of-truth benefit a framework would provide, at no dependency
  cost.

Revisit only if the command set grows large/dynamic enough that hand-maintaining
metadata becomes the bottleneck — not before.
