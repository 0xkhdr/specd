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

# Order-dependence guard — a second run with no cache catches any test or
# golden that leaks state or depends on map-iteration order (see Determinism).
go test ./... -count=2

# Static checks (must be clean).
gofmt -l .            # prints nothing when formatted
go vet ./...
shellcheck scripts/*.sh

# Enforce the coverage regression floor (see Coverage policy).
./scripts/coverage-check.sh
```

Run the concurrency stress harness (many processes hammering one spec):

```sh
./scripts/stress.sh        # uses the built ./specd binary
```

Or run the whole CI gate locally in one shot:

```sh
make ci    # lint + race test + count=2 + coverage floor + perf-gate + stress
```

The `perf-gate` step runs the onboarding deterministic-output checks
(`-run 'Deterministic|BenchmarkContract' -count=2`): byte-stable `init --json`
receipts and stable MCP probe contract fields. Latency baselines are recorded with
`make bench` and **reviewed, not gated** (no flaky wall-clock CI assertions) — see
[docs/agent-harness-baselines.md](docs/agent-harness-baselines.md). Supported vs
snippet-only host evidence lives in
[docs/agent-harness-compat.md](docs/agent-harness-compat.md) and is parity-checked
against the registry by `TestHostCompatibilityMatrix`.

## Test strategy

| Concern | Where | What it proves |
|---|---|---|
| Exit-code determinism | `internal/cmd/commands_test.go`, `main_test.go` | `0=ok 1=gate 2=usage 3=not-found` for every entry path |
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

## Helpers — `internal/testharness`

`testharness.New(t)` returns a `*Harness` bound to an isolated `t.TempDir()`
root. The harness runs commands in-process (no subprocess) and exposes fluent
builders + asserters:

| Helper | Purpose |
|---|---|
| `New(t)` | isolated project root in a temp dir |
| `h.Init()` | run `specd init` to scaffold `.specd/` |
| `h.Spec(slug)` | fluent `SpecBuilder` — `.Title().Req().FullDesign().AddTask().Phase().Gate().Turn().Build()` |
| `h.Run(cmd, args...)` / `h.RunExpect(want, cmd, args...)` | dispatch a command, capture stdout/stderr/code |
| `h.State(slug)` | `StateAsserter` — `.Status().Phase().Gate()…` chained assertions |
| `h.Path(rel)` / `h.SpecPath(slug, name)` | resolve paths under the temp root |
| `h.InitGit()` / `h.GitCommitAll(msg)` / `h.GitHead()` | git fixtures for verification (gitHead) tests |

## State-backend conformance parity

`internal/core/backend_conformance_test.go` runs one shared suite
(`backendConformance`) against every `StateBackend`, so a backend swap can never
weaken the lock + revision-CAS + atomicity contract. The backend table is built
by `availableBackends()`:

| Backend | When it runs | When it skips (with reason) |
|---|---|---|
| `file` | always | — |
| `git` | `git` on `PATH` | `git not on PATH` |
| `postgres` | built `-tags specd_postgres` **and** `SPECD_PG_DSN` set | not compiled in / DSN unset |
| `redis` | built `-tags specd_redis` **and** `SPECD_REDIS_ADDR` set | not compiled in / addr unset |

A missing driver or service is a **visible `t.Skip` with a reason**, never a
silent pass — `go test -run Conformance -v` prints exactly which backends ran vs
skipped, so a default-build CI run cannot masquerade an unexercised backend as
green. To exercise the database-backed backends locally:

```sh
# Postgres: needs a reachable DB with a specd_state(slug text primary key,
# revision int, doc text) table.
go test -tags specd_postgres ./internal/core/ -run Conformance -v \
  -count=1   # SPECD_PG_DSN=postgres://user:pass@localhost/specd?sslmode=disable

# Redis: needs a reachable redis (SPECD_REDIS_ADDR, default 127.0.0.1:6379).
SPECD_REDIS_ADDR=127.0.0.1:6379 \
  go test -tags specd_redis ./internal/core/ -run Conformance -v -count=1
```

Default builds link **no** database driver (`go.mod` declares zero external
deps); `TestDefaultLinksNoDriver` guards that the registry stays empty and
`SelectBackend("redis"|"postgres")` fails closed without the build tag.

## Determinism (golden / report output)

specd emits no nondeterministic report fixtures. The invariants that keep
output stable:

- **Time** is injected via `core.Clock` and swapped for `testharness` /
  `FakeClock` in tests — no report depends on wall-clock time.
- **Ordering**: anything assembled from a map (waves, task lists, agents) is
  **sorted before emit**. Never range a map straight into output.
- **No absolute paths or git HEAD** leak into rendered reports.

There are no checked-in `.golden` files; report tests assert on rendered
content. To verify determinism, `go test ./... -count=2` (and CI) runs the
suite twice with the cache disabled — order-dependence or leaked state surfaces
as a diff between runs. When you add report output, sort first and add a
content assertion; if a future golden fixture is introduced, regenerate it with
its test's documented update flag and review the diff before committing.

## Coverage policy

Coverage is a **regression ratchet**, not a vanity number.
`./scripts/coverage-check.sh` (run in CI) fails the build if coverage drops
below the floor:

| Scope | Floor (enforced) | Long-term target |
|---|---|---|
| overall | `OVERALL_MIN` = **65%** | 85% |
| `internal/core` (the engine) | `CORE_MIN` = **60%** | 95% |

The floors sit just under current measured coverage so a refactor can't
silently lose tests; the targets are where we're driving them. Raise the floors
as coverage improves — never lower them to turn a red build green without a
written justification in the PR.

100% is held on the integrity-critical functions: `ValidateSlug`, the
`SpecdError` constructors, `WithSpecLock`, and `LoadState`. `SaveState` /
`migrate` sit in the 90s (uncovered lines are disk-error branches needing fault
injection). `internal/cmd` is thin command glue; its integrity-relevant
branches (exit codes, traversal rejection, gate handling) are tested directly
rather than chasing print-formatting lines.

## CI & platform matrix

`.github/workflows/ci.yml` runs on every PR and on push to `main`:

| Job | Runs on | Gate |
|---|---|---|
| `lint` | ubuntu | `gofmt -l` (fail on output), `go vet`, `shellcheck scripts/` |
| `test` | ubuntu + macOS | `go test -race -count=1 -coverprofile`, then `-count=2` (includes the onboarding deterministic-output gate; also runnable standalone via `make perf-gate`) |
| `coverage-floor` | ubuntu | `scripts/coverage-check.sh` |
| `stress` | ubuntu | `scripts/stress.sh` cross-process contention |
| `build` | ubuntu + macOS + Windows | `go build` |

`.github/workflows/release.yml` runs only on `v*` tags: it re-runs the race
suite, then GoReleaser.

### Windows limitation (known, documented)

specd **builds and runs on Windows**, but `specd update` self-replacement is
**known-limited**: it renames the new binary over the running executable
(`update.go`), which Windows forbids for an in-use file. Tests run on Linux +
macOS; Windows is build-only in CI. Windows users should reinstall via
`install.sh` semantics / a fresh download rather than `specd update`. Lifting
this requires the rename-to-sidecar + relaunch dance and is tracked as
follow-up, not silently broken.

## Releases & checksum verification

GoReleaser publishes a `SHA256SUMS` file alongside the archives
(`.goreleaser.yml` → `checksum.name_template: SHA256SUMS`). The filename is
load-bearing and must stay identical across three consumers:

- `.goreleaser.yml` — produces `SHA256SUMS`
- `internal/cmd/update.go` — `fetchChecksums` downloads `SHA256SUMS`, fails
  closed on mismatch
- `scripts/install.sh` — downloads + verifies `SHA256SUMS` (`--no-verify` opts
  out loudly)

Changing the name in one place breaks self-update and install verification.
cosign signing is a documented follow-up, intentionally not implemented yet.
