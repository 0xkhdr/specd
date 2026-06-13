# specd Contributor Guide

This guide is for developers and AI agents contributing to the development of the `specd` tool itself. It details the internal codebase structure, custom parsing engines, concurrency safeguards, and instructions on how to extend the harness.

---

## 1. Codebase Directory Walkthrough

`specd` is a single-binary Go application with **zero external runtime dependencies**. All dependencies are from the Go standard library.

```
main.go                                  # Entry point; CLI dispatch; version ldflags
Makefile                                 # Build, test, lint commands
go.mod                                   # Module definition (no external requires)
.goreleaser.yml                          # Multi-platform release builds
.github/workflows/release.yml            # CI: tests + goreleaser on tag

internal/
├── cli/
│   └── args.go                          # Hand-rolled argument parser (no flag library)
│
├── cmd/
│   ├── init.go, new.go, approve.go      # Lifecycle commands
│   ├── check.go, next.go, dispatch.go   # Execution commands
│   ├── verify.go, task.go               # Verification/task status commands
│   ├── decision.go, midreq.go, memory.go # ADR, feedback, learning commands
│   ├── status.go, context.go, report.go # Inspection/reporting commands
│   ├── waves.go, program.go             # DAG visualization + cross-spec commands
│   ├── update.go, version.go            # Meta commands (self-update, version)
│   └── helpers.go                       # Common exit/error helpers
│
└── core/
    ├── paths.go                         # Canonical .specd/ path traversal
    ├── io.go                            # AtomicWrite, AppendFile, ReadOrNull, ReadOrDefault
    ├── lock.go                          # Reentrant file-based advisory lock (O_EXCL)
    ├── exit.go                          # SpecdError type and exit code constants
    ├── state.go                         # State schema, LoadState/SaveState (CAS), migrations
    ├── specfiles.go                     # Spec directory accessor; config/manifest loaders
    ├── phases.go                        # Phase transitions, status mapping, design checklist
    ├── tasksparser.go                   # Custom markdown parser for tasks.md
    ├── dag.go                           # DAG construction, waves, frontier, critical path
    ├── ears.go                          # EARS requirement syntax validator (regex)
    ├── program.go                       # Cross-spec DAG construction and frontier
    ├── render.go                        # Wave graph ASCII rendering, summary helpers
    ├── report.go                        # Markdown and HTML report generation
    ├── ui.go                            # ANSI colors, JSON mode, NO_COLOR support
    ├── help.go                          # Help text rendering and JSON schema export
    ├── commands.go                      # CommandMeta registry (19 commands)
    ├── md.go                            # Markdown parsing helpers
    │
    ├── embed.go                         # //go:embed declaration for templates
    └── embed_templates/                 # Baked-in template files
        ├── AGENTS.md                    # Agent prompt pack template
        ├── config.json                  # Default config stub
        ├── roles/                       # Role persona templates
        │   ├── investigator.md
        │   ├── builder.md
        │   ├── reviewer.md
        │   └── verifier.md
        ├── steering/                    # Global steering constitution
        │   ├── reasoning.md
        │   ├── workflow.md
        │   ├── product.md
        │   ├── tech.md
        │   ├── structure.md
        │   └── memory.md
        └── specStubs/                   # Spec artifact stubs
            ├── requirements.md
            ├── design.md
            ├── tasks.md
            ├── decisions.md
            ├── mid-requirements.md
            └── memory.md

scripts/
├── install.sh                           # curl | bash installer (downloads GitHub Release binary)
└── uninstall.sh                         # Removes binary and PATH entry
```

---

## 2. Validation Gate Pipeline

The validation pipeline runs sequentially during `specd check` and is enforced at phase boundaries when running `specd approve`.

### The 7 Validation Gates:
1.  **EARS Gate** (`internal/core/ears.go`): Regex-based EARS requirement syntax validator. Ensures every requirement contains a user story and all acceptance criteria match one of five EARS patterns.
2.  **Design Gate** (`internal/core/phases.go`): Checks `design.md` for all 7 mandatory section headers (Overview, Architecture, Components, Data Models, Error Handling, Verification Strategy, Risks) free of TODO markers.
3.  **Task-Schema Gate** (`internal/core/tasksparser.go`): Validates all tasks have 7 mandatory keys (why, role, files, contract, acceptance, verify, depends). Builder/verifier tasks cannot have `verify: N/A`.
4.  **DAG Gate** (`internal/core/dag.go`): Detects cycles, orphan dependencies, and wave violations in task dependency graph.
5.  **Evidence Gate** (`internal/cmd/check.go`, `internal/core/state.go`): No task complete without evidence. Non-read-only tasks require passing verify record (`specd verify` exit 0).
6.  **Sync Gate** (`internal/core/specfiles.go`): Markdown checkbox statuses (`[ ]`, `[/]`, `[x]`, `[!]`) in `tasks.md` must match `state.json` task statuses.
7.  **Traceability Gate** (`internal/core/specfiles.go`): Every requirement ID referenced in tasks must exist in requirements.md. Unreferenced requirements severity controlled by `config.gates.traceability` (warn/error).

---

## 3. Concurrency & Durability Model

### 1. Advisory Lock (`internal/core/lock.go`)
Mutating operations acquire spec-level lockfile `.specd/specs/<slug>/.lock` using `O_EXCL` (exclusive create). Lock acquisition retries with exponential backoff; stale locks (>30s) auto-reclaim.

Key function: `WithSpecLock[T](root, slug string, fn func() (T, error)) (T, error)`

Reentrancy tracking via `sync.Map` prevents deadlock when a process acquires the same lock multiple times.

### 2. Compare-And-Swap (CAS) for State Mutations
Every `state.json` load captures the `revision` number. On save, CAS verifies revision matches; write aborts if revision changed (concurrent write detected). Prevents clobbering.

Key functions: `LoadState()`, `SaveState()` in `internal/core/state.go`

### 3. Atomic File Writes
`AtomicWrite(path, data string)` in `internal/core/io.go` implements:
1. Create temp file in same directory (name includes PID for debugging).
2. Write content + `Sync()` to disk.
3. `os.Rename()` atomic swap (POSIX).

Temp file cleaned up via deferred `os.Remove()` on error.

### 4. Atomic Ledger Appends
Logs like `decisions.md`, `mid-requirements.md`, `memory.md` append via `AppendFile()` with `os.O_APPEND`. Ensures serialized, non-interleaved writes.

---

## 4. Parser Internals: `tasksparser.go`

To remain dependency-free, `specd` implements a custom tasks.md parser without AST libraries. Preserves formatting, whitespace, and comments.

**ParseTasks** (line-by-line state machine):
- Detects `## Wave N` headers and task lines `- [ ] TID — title`.
- Parses indented metadata keys (`  - key: value`).
- Decodes task annotations (checkbox state, evidence, timestamp) from task line suffix.

**SerializeTasks** (byte-stable reconstruction):
- Rebuilds document line-by-line.
- Round-trip guarantee: `ParseTasks(doc) → SerializeTasks() → ParseTasks()` yields identical bytes.
- Tests in `internal/core/tasksparser_test.go` enforce stability.

---

## 5. Extending the CLI

### How to Add a CLI Command
1.  **Create Command File**: Create `internal/cmd/mycommand.go` with handler:
    ```go
    func RunMyCommand(args cli.Args) int {
      root := core.MustFindSpecdRoot()
      slug := args.Positional(0)
      // Implementation
      return core.ExitOK
    }
    ```
2.  **Register Dispatch**: Add case in `main.go`'s dispatch switch:
    ```go
    case "mycommand":
      return cmd.RunMyCommand(args)
    ```
3.  **Add CommandMeta**: Register help metadata in `internal/core/commands.go`:
    ```go
    {
      Command: "mycommand", Category: "inspection",
      Description: "Brief description",
      Usage: "specd mycommand <slug>",
      // ... flags, examples, exit codes
    }
    ```
4.  **Add Tests**: Create tests in `internal/core/*_test.go` for core logic, or `internal/cmd/` for command-level behavior.

### How to Add/Modify a Validation Gate
1.  **Define Logic**: Write validation in appropriate core file (e.g., `internal/core/ears.go` for syntax, or inline in `internal/cmd/check.go`).
2.  **Add to check**: In `internal/cmd/check.go`, call validation and collect failures:
    ```go
    if err := validateMyGate(spec); err != nil {
      failures = append(failures, core.ValidationFailure{
        Gate: "my-gate", Severity: "fail", Message: err.Error(),
      })
    }
    ```
3.  **Block phase if needed**: In `internal/cmd/approve.go`, gate the transition:
    ```go
    if cfg.Gates.MyGate && hasViolations {
      return core.ExitGate // Exit 1
    }
    ```
4.  **Add test**: Verify gate detection in `internal/core/*_test.go`.

### How to Modify `state.json` Schema
1.  **Update State Struct**: Edit `State` type in `internal/core/state.go`.
2.  **Increment SchemaVersion**: Bump `SchemaVersion` constant.
3.  **Write Migration**: Add case in `migrate()` function:
    ```go
    case 2:
      st.SchemaVersion = 3
      st.MyNewField = defaultValue
    ```
4.  **Add Migration Test**: Verify upgrade in `internal/core/state_test.go` or equivalent.

---

## 6. Build & Release

### Local Development
```sh
make build         # Compile binary to ./specd
make test          # Run all tests (internal/core/*_test.go)
make lint          # Run go vet
go run . <cmd>     # Run directly without building
```

### Release Build
Tagged commits (`v*`) trigger `.github/workflows/release.yml`:
1. Run tests.
2. Run `goreleaser` (via `.goreleaser.yml`) to build multi-platform binaries (linux/darwin, amd64/arm64).
3. Create GitHub Release with artifacts.
4. `install.sh` downloads from Releases and installs to `~/.local/bin/specd`.

### Key Build Flags
Version is stamped via ldflags:
```sh
go build -ldflags "-s -w -X main.version=<git-sha>"
```
- `-s -w`: Strip symbols/debuginfo for smaller binary.
- `-X main.version`: Set version variable at compile time.
