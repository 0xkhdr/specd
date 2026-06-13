# CLAUDE.md — Contributor Guidelines

Quick reference for developers and coding agents contributing to `specd`.

## Commands

### Build & Compilation
```sh
make build             # go build -ldflags "-s -w -X main.version=$(VERSION)" -o specd .
make install           # go install (puts specd in $GOPATH/bin)
```

### Running Tests
```sh
make test              # go test -race ./...
```

### Running from Source
```sh
go run . <command>     # Run directly without building
```

### Linting
```sh
make lint              # go vet ./...
```

---

## Code Style & Invariants

All contributions must respect these core architectural constraints:

1. **Zero Runtime Dependencies**: `go.mod` must have no `require` directives. Use stdlib only.
2. **Durable & Atomic File Writes**: Never use `os.WriteFile` directly for mutable state (`state.json` or `tasks.md`). Use `AtomicWrite` in [`internal/core/io.go`](internal/core/io.go) — writes to a temp file, calls `Sync()`, then `os.Rename` (POSIX atomic).
3. **Optimistic Concurrency (CAS)**: State mutations must call `LoadState`, verify the `revision` field matches on-disk, and increment on write via `SaveState`.
4. **Reentrant Advisory Locks**: Mutating commands must acquire the spec-specific advisory lock via `WithSpecLock` in [`internal/core/lock.go`](internal/core/lock.go). Uses `O_EXCL` lockfile + `sync.Map` reentrancy counter.
5. **Round-Trip Parser Stability**: `ParseTasks` → `SerializeTasks` must produce identical bytes. Tests in [`internal/core/tasksparser_test.go`](internal/core/tasksparser_test.go) enforce this.
6. **Exit Code Semantics**:
   - `0`: Success.
   - `1`: Validation gate / check failed (EARS, design headers, DAG cycle, evidence gate).
   - `2`: Usage error / bad CLI arguments.
   - `3`: `.specd/` root or specified spec slug not found.

---

## Project Structure

```
main.go                         # Entry point; dispatch switch; version ldflags
internal/
  cli/
    args.go                     # Hand-rolled arg parser — no external deps
  cmd/
    *.go                        # One file per command (init, new, check, task, verify, …)
    helpers.go                  # specdExit(), usageExit()
  core/
    dag.go                      # DAG engine: cycle detection, waves, critical path, frontier
    ears.go                     # EARS linter: regex patterns for 5 EARS forms
    embed.go                    # //go:embed embed_templates — templates baked into binary
    embed_templates/            # Template files (AGENTS.md, config.json, steering/, roles/, specStubs/)
    exit.go                     # Exit codes and SpecdError type
    io.go                       # AtomicWrite, AppendFile, ReadOrNull, ReadOrDefault
    lock.go                     # WithSpecLock[T any] — advisory lock with generics
    paths.go                    # Canonical .specd/ path helpers
    phases.go                   # PhaseForStatus, DesignGate, PhaseReadiness, PlanningAdvance
    program.go                  # ProgramManifest, BuildProgram (cross-spec DAG)
    render.go                   # WaveGraph, NextSummary, CountTasks, BlockerLines, …
    report.go                   # RenderMarkdown, RenderHTML, GetBadge
    specfiles.go                # Config types, LoadConfig, LoadSpec, ListSpecs, RequireSpec
    state.go                    # All types + LoadState/SaveState (CAS), migrate() v0→v4
    tasksparser.go              # ParseTasks, SerializeTasks, ApplyTaskAnnotation
    ui.go                       # ANSI color output; SPECd_JSON env; NO_COLOR support
    help.go                     # RenderHelp, RenderCommandHelp, RenderHelpJSON
    commands.go                 # CommandMeta slice (19 commands)
    md.go                       # Markdown helpers
Makefile
.goreleaser.yml                 # Multi-platform release builds (linux/darwin/windows, amd64/arm64)
.github/workflows/release.yml  # CI: vet + test + goreleaser on tag push
scripts/
  install.sh                   # curl-pipe installer — downloads pre-built binary from GitHub Releases
  uninstall.sh                 # Removes binary and PATH entries
```

---

## Adding a New Command

1. Create `internal/cmd/<name>.go` with `func Run<Name>(args cli.Args) int`.
2. Add the dispatch case in `main.go`'s `dispatch()` switch.
3. Add `CommandMeta` entry in `internal/core/commands.go`.
4. Write unit tests in `internal/core/` if adding core logic; command-level tests go in `internal/cmd/`.

## Templates

Templates live in `internal/core/embed_templates/` and are embedded at compile time via `//go:embed`. Do **not** modify the root-level `src/templates/` path — it no longer exists. Edit files directly in `embed_templates/`.
