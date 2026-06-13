# Testing specd

> The agent reasons. The harness enforces. The tests guard the harness.

specd's job is to enforce process integrity for AI agents, so its own behavior
must be deterministic and concurrency-safe. The test suite is built around that
contract: exit codes, state atomicity, lock correctness, and input validation
are the load-bearing paths and get the most attention.

## Running the suite

```sh
# Full suite, race detector on, fresh (no cache), with coverage.
go test ./... -race -count=1 -coverprofile=coverage.out

# Per-function coverage (find untested branches).
go tool cover -func=coverage.out

# HTML coverage browser.
go tool cover -html=coverage.out

# Static checks (must be clean).
gofmt -l .        # prints nothing when formatted
go vet ./...
```

Run the concurrency stress harness (many processes hammering one spec):

```sh
./scripts/stress.sh        # uses the built ./specd binary
```

## Test strategy

| Concern | Where | What it proves |
|---|---|---|
| Exit-code determinism | `internal/cmd/exitcode_test.go`, `main_test.go` | `0=ok 1=gate 2=usage 3=not-found` for every entry path |
| Slug / path-traversal | `internal/core/slug_test.go` | `..`, `/`, absolute paths, shell metachars all rejected |
| State atomicity (CAS) | `internal/core/state_cas_test.go` | revision bump, concurrent-write detection, corrupt/malformed/newer-schema handling |
| Lock correctness | `internal/core/lock_test.go`, `concurrency_test.go` | release, reentrancy, timeout, stale reclaim, no lost updates under N goroutines |
| Atomic file I/O | `internal/core/io_test.go` | no temp leftovers, parent-dir creation, append semantics |
| Arg parsing | `internal/cli/args_test.go` | positionals vs flags, boolean flags, value flags |
| DAG / EARS / tasks parsing | `internal/core/dag_test.go`, `ears_test.go`, `tasksparser_test.go` | wave frontier + requirement parsing edge cases |

## Conventions

- **Table-driven** tests with descriptive subtest names
  (`t.Run("rejects_path_traversal_slug_with_usage_code", ...)`).
- **Arrange / Act / Assert** comments mark each section in non-trivial tests.
- **Hermetic**: every test uses `t.TempDir()` and `t.Setenv()`. No network, no
  reliance on the host's git config or real `.specd/` tree.
- Tests that `Chdir` or touch shared lock state are **not** `t.Parallel()`.
- The race detector is mandatory for lock/state changes —
  `internal/core/concurrency_test.go` is designed to fail under `-race` if the
  advisory-lock + CAS path ever regresses.

## Helpers — `internal/testutil`

| Helper | Purpose |
|---|---|
| `NewTempSpecdRoot(t)` | isolated project root with an empty `.specd/specs/` |
| `Chdir(t, dir)` | scoped working-directory switch, auto-restored |
| `NewSpec(t, root, slug, title)` | scaffold a spec with an initial `state.json` |
| `WriteArtifact(t, root, slug, name, content)` | drop a spec artifact (e.g. `requirements.md`) |
| `MustReadState(t, root, slug)` | load state, failing the test on error |

## Coverage targets

100% is enforced on the integrity-critical functions:
`ValidateSlug`, the `SpecdError` constructors, `WithSpecLock`, and `LoadState`.
`SaveState` / `migrate` sit in the 90s (the uncovered lines are
disk-error branches that require fault injection).

Whole-package coverage is highest in `internal/core` (the engine) and
`internal/cli` (100%). `internal/cmd` is mostly thin command glue; its
integrity-relevant branches (exit codes, traversal rejection, gate handling)
are tested directly rather than chasing print-formatting lines.
