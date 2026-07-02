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
(`-run 'Deterministic|BenchmarkContract|ManifestDisabledMode' -count=2`): byte-stable `init --json`
receipts, stable MCP probe contract fields, and disabled-mode manifest behavior.
Latency baselines are recorded with `make bench` and **reviewed, not gated** (no
flaky wall-clock CI assertions) — see
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

- **One file per source unit**: `slug.go → slug_test.go`, `lock.go →
  lock_test.go`. The unit file holds *all* of that unit's tests — happy path,
  edge cases, and regressions. There are no `_more` / `_regression` / `_sweep` /
  `_scale` / `wave*` dumping-ground files; a regression becomes a subtest named
  for the defect it guards inside the canonical file.
- **Cross-cutting suites get a named home** for the *concern*, not a dev stage:
  `core/backend_conformance_test.go`, `core/concurrency_test.go`,
  `cmd/lifecycle_test.go`, `cmd/json_contract_test.go`, `mcp/integration_test.go`,
  `integration/conformance_test.go`. Each declares its concern in a file-level
  comment.
- **Table-driven** tests with `snake_case` subtest names describing the asserted
  outcome (`t.Run("rejects_path_traversal_slug_with_usage_code", ...)`) — never
  space-separated / sentence-case labels.
- **Arrange / Act / Assert** comments mark each section in non-trivial tests.
- **Hermetic**: every test uses `t.TempDir()` and `t.Setenv()`. No network, no
  reliance on the host's git config or real `.specd/` tree.
- **Parallelism is explicit**: pure-function tests with no shared state (the
  `dag` / `ears` / `tasksparser` / `slug` / `frontier` parsers) call
  `t.Parallel()`. Tests that `Chdir` or touch shared lock / global state
  (`os.Stdout`, `core.Clock`) are **not** `t.Parallel()`.
- The race detector is mandatory for lock/state changes —
  `internal/core/concurrency_test.go` is designed to fail under `-race` if the
  advisory-lock + CAS path ever regresses.

These structural rules are enforced in CI by `./scripts/test-lint.sh` (the
`make test-lint` target / the `lint` job): it fails on banned file suffixes,
space-separated subtest names, and duplicate helper definitions within a
package.

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
| `testharness.CaptureStderr(t, fn)` / `CaptureStdout` | capture a function's stream writes (shared, replaces per-file `captureStderr`) |
| `testharness.NewFakeOrchestrationHost(h)` / `NewFakePinkyWorker(h, id)` | drive Brain/Pinky orchestration flows deterministically |

**Helper consolidation (no copy-paste).** A test author never re-implements
setup. Cross-package, specd-type-free helpers are exported from `testharness`
(e.g. `CaptureStderr`). Helpers that need a package's unexported types live once
in that package's `helpers_test.go` (`core/helpers_test.go` holds `ids`,
`mkState`, `newTestACPStore`; `mcp/helpers_test.go` holds `td`, `names`). The
full audit and placement rule is in `internal/testharness/HELPERS.md`. Internal
`package mcp`/`package core` test files cannot import `testharness` (it imports
`cmd`→`mcp`, a cycle), so those packages keep a documented local mirror; external
`_test` packages use the shared helper.

The harness is itself tested (`testharness/*_test.go`, ≥ 80% coverage) so a bug
in the infrastructure surfaces there, not as a confusing failure downstream.

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
| overall | `OVERALL_MIN` = **79%** | 85% |
| `internal/core` (the engine) | `CORE_MIN` = **80%** | 90% → 95% |
| `internal/cmd` (CLI/orchestration glue) | `CMD_MIN` = **71%** | 80% |
| `internal/worker` (process seam) | `WORKER_MIN` = **88%** | 95% |
| `internal/mcp` | `MCP_MIN` = **88%** | 90% |
| `internal/testharness` | `HARNESS_MIN` = **80%** | 90% |
| `internal/spec` (role/phase/status enums) | `SPEC_MIN` = **99%** | 99% |
| `internal/context` | `CONTEXT_MIN` = **92%** | 95% |
| `internal/runner` (verify sandbox backends) | `RUNNER_MIN` = **92%** | 95% |
| `internal/pack` | `PACK_MIN` = **87%** | 90% |
| `internal/schema` | `SCHEMA_MIN` = **84%** | 90% |

The floors sit just under current measured coverage so a refactor can't
silently lose tests; the targets are where we're driving them. Raise the floors
as coverage improves. Floors only ratchet up: never lower one to turn a red
build green; add tests or document an intentional coverage-shape change in the
PR.

The lower five rows (`internal/spec`, `internal/context`, `internal/runner`,
`internal/pack`, `internal/schema`) were added as ratchet steps in Wave 3 (A8):
previously only the overall number guarded them, so a regression in a substantive
package could pass CI as long as the aggregate held. `internal/spec` jumped from
~46% to 100% when `role.go`/phase/status gained direct tests; the rest are floored
just under their current measured coverage and ratchet upward toward 85/90/95.

100% is held on the integrity-critical functions: `ValidateSlug`, the
`SpecdError` constructors (`UsageError` / `GateError` / `NotFoundError`),
`WithSpecLock`, and `LoadState`. `SaveState` / `migrate` sit in the 90s
(uncovered lines are disk-error branches needing fault injection). `internal/cmd`
is thin command glue; its integrity-relevant branches (exit codes, traversal
rejection, gate handling) are tested directly rather than chasing
print-formatting lines.

Every `internal/core` function still under its coverage target is tracked as a
follow-up test task or annotated "won't test: <reason>" at the call site; there
is no separate dark-path inventory file.

## CI & platform matrix

`.github/workflows/ci.yml` runs on every PR and on push to `main`:

| Job | Runs on | Gate |
|---|---|---|
| `lint` | ubuntu | `gofmt -l` (fail on output), `go vet`, `shellcheck scripts/`, docs lint |
| `analyze` | ubuntu | `go mod tidy` diff check, golangci-lint v2.1.6, govulncheck |
| `test` | ubuntu + macOS | `go test -race -count=1 -coverprofile`, then `-count=2`, plus `make perf-gate` |
| `coverage-floor` | ubuntu | `scripts/coverage-check.sh` |
| `stress` | ubuntu | `scripts/stress.sh` cross-process contention |
| `stress-acp` | ubuntu | `scripts/stress-acp.sh` ACP ledger contention |
| `stress-orchestration` | ubuntu | `scripts/stress-orchestration.sh` Brain/Pinky orchestration contention |
| `stress-program` | ubuntu | `scripts/stress-program.sh` cross-spec program scheduling contention |
| `stress-brain-recovery` | ubuntu | `scripts/stress-brain-recovery.sh` recovery/reclaim paths |
| `stress-checkpoint-fault` | ubuntu | `scripts/stress-checkpoint-fault.sh` checkpoint fault injection |
| `build` | ubuntu + macOS + Windows | host `go build`; ubuntu also cross-compiles linux/arm64, darwin/arm64, windows/amd64 |

`.github/workflows/release.yml` runs only on `v*` tags: it re-runs the race
suite, then GoReleaser.

### Windows limitation (known, documented)

specd **builds and runs on Windows**, but task execution and Brain/Pinky
worker orchestration depend on a POSIX shell (execution commands are invoked
with `-c`; orchestration fails fast with `orchestration requires a POSIX
shell (sh); not supported on Windows — run under WSL`, see `README.md`'s
Windows note). Tests run on Linux + macOS; Windows is build-only in CI.
Windows users should run under WSL, or use a bash-like environment (e.g. Git
for Windows) on the `PATH`, for orchestration and verification work.

## Releases & checksum verification

GoReleaser publishes a `SHA256SUMS` file alongside the archives
(`.goreleaser.yml` → `checksum.name_template: SHA256SUMS`). The filename is
load-bearing and must stay identical across two consumers:

- `.goreleaser.yml` — produces `SHA256SUMS`
- `scripts/install.sh` — downloads + verifies `SHA256SUMS` (`--no-verify` opts
  out loudly)

Changing the name in one place breaks install verification.
cosign signing is a documented follow-up, intentionally not implemented yet.
