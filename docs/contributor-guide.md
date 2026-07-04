# Contributor Guide

Hacking on `specd` itself — building from source, directory map, core code contracts, extension guidelines, and non-negotiable coding invariants.

---

## 1. Building and Testing

No external dependencies are required. `specd` is built entirely using the Go standard library. Templates and assets are embedded inside the binary via `go:embed`.

### Common Commands

```bash
make build          # Build the binary -> ./specd
make install        # Install into $GOBIN
make test           # Run all unit and integration tests
make lint           # Check code styles and run go vet
go run . <command>  # Run specd directly from source
```

Tests are colocated with the implementation files as `*_test.go`.

---

## 2. Codebase Map

```
specd/
├── main.go                       # Entry point, routes to cmd.Run
├── internal/
│   ├── cli/
│   │   ├── args.go               # Custom parser for CLI arguments and flags
│   │   └── args_test.go
│   ├── cmd/                      # Command handlers and dispatch routing
│   │   ├── registry.go           # Command routing registry (executable map)
│   │   ├── lifecycle.go          # Core lifecycle command implementations (new, approve, check, etc.)
│   │   ├── brain_run.go          # Brain orchestration controller subcommand handler
│   │   └── memory.go             # Memory add/promote subcommand handler
│   ├── core/                     # Core domain logic (locks, state, dag, gates, config)
│   │   ├── config_loader.go      # Project and global YAML config loader
│   │   ├── lock.go               # Per-spec advisory lock helper (WithSpecLock)
│   │   ├── state.go              # state.json structure & SaveStateCAS helper
│   │   ├── tasksparser.go        # Bespoke, round-trip stable task parser
│   │   ├── dag.go                # Directed Acyclic Graph (DAG) task engine
│   │   ├── io.go                 # Atomic write filesystem helper
│   │   ├── commands.go           # Central help command metadata & schema
│   │   ├── gates/                # Validation gate registry and implementations
│   │   │   ├── registry.go       # General gate runner
│   │   │   ├── core.go           # 12 built-in core gates
│   │   │   ├── ears.go           # EARS syntax linter
│   │   │   └── sync.go           # Invariant validation for tasks.md vs state.json
│   │   └── verify/               # Local verify command shell executor
│   └── mcp/                      # Model Context Protocol stdio adapter
```

---

## 3. Core Coding Invariants (Non-negotiable)

When contributing code to `specd`, you must adhere to these structural guardrails:

### I. Zero Runtime Dependencies
`go.mod` must only define the Go module name and Go version. No external packages are allowed. All features (parsing, JSON serialization, execution, sandboxing) must use standard library packages or custom code built on top of them.

### II. Atomic Writes
Never write files using raw `os.WriteFile` or un-buffered output streams. All file modifications must use `core.AtomicWrite` (which writes to a temporary file, calls `fsync` to guarantee durability, and performs an atomic rename/move) to prevent file corruption during sudden interruptions.

### III. Optimistic Concurrency (CAS)
Do not write state data blindly. All state changes to `state.json` must load the current state, verify the expected version using the `revision` number, increment the revision, and use `core.SaveStateCAS` to perform a compare-and-swap update.

### IV. Advisory Lock
Every command that mutates files inside a spec workspace (such as `.specd/specs/<slug>/`) must be wrapped within a reentrant per-spec advisory lock using `core.WithSpecLock`. This guarantees safety across concurrent execution loops.

### V. Parser Round-Trip Stability
The tasks parser (`core.ParseTasksMd`) must be byte-for-byte stable. Parsing a `tasks.md` file and writing it back out must yield the exact same byte sequence.

### VI. Embedded Templates
All templates (default project files, steering constitution guides, roles prompts) must be shipped as embedded assets using `go:embed`. Never look up assets on the local disk using paths relative to the executable binary.

---

## 4. Extending the CLI

### Adding a Subcommand

1. **Write the command handler**: Create or edit a file under `internal/cmd/`. The handler signature is:
   ```go
   func runMyCommand(root string, args []string, flags map[string]string) error
   ```
2. **Register the handler**: Open `internal/cmd/registry.go` and map the verb to your handler in the `executable` map:
   ```go
   var executable = map[string]Handler{
       "mycommand": runMyCommand,
   }
   ```
3. **Declare command metadata**: Add the command metadata to `core.Commands` inside `internal/core/commands.go`. This makes the command discoverable under `specd help` and automatically generates the MCP tool schema.
4. **Colocate unit tests**: Add test coverage to verify execution parameters, flag parsing, and exit codes.

### Adding a Validation Gate

1. **Implement the logic**: Write a check function in `internal/core/gates/` or under the `internal/core/` domain (e.g. within `ears.go`).
2. **Wire the gate**: Register the validation logic inside the `CoreRegistry()` constructor in `internal/core/gates/core.go`:
   ```go
   registry.Register(gateFunc{name: "my-gate", run: myGateCheckFunction})
   ```
3. **Check inputs**: Ensure your check is pure. It should read from `CheckCtx` and output findings. Avoid disk lookups inside the gate.
4. **Assert behavior**: Add test cases to the gate's test file showing passes and expected failures.

---

## 5. Architectural Decision: Custom CLI Parser

`specd` uses a custom, minimal argument parser (`internal/cli/args.go`) rather than using popular third-party frameworks like Cobra or urfave/cli.

- **Zero dependencies**: Third-party CLI frameworks pull in external packages (such as `pflag` or `yaml`), violating the stdlib-only invariant.
- **Determinism**: Keeps argument routing highly predictable. Help and routing behavior are bound together by unit tests (`TestRegistryMatchesHelp` in `internal/cmd/registry_test.go`), preventing documentation drift.
