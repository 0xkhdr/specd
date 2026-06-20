# Test Suite Specification — specd

> The agent reasons. The harness enforces. The tests guard the harness.

This spec defines the **target state** of the specd test suite. It is the
contract a rebuilt suite must satisfy: one naming scheme, one set of shared
helpers, one organization principle, and a coverage map that ties every
integrity-critical path to a named test. `tasks.md` is the executable plan that
gets us there.

This is a **reorganization + coverage-raise**, not a behavior change to specd.
No production code under test changes. Existing assertions are preserved (they
encode real, hard-won invariants) — they are *moved, renamed, de-duplicated, and
gap-filled*, not rewritten from scratch.

---

## 1. Why rebuild

The current suite (118 `_test.go` files, ~18k test LOC, 69.3% overall coverage)
works but has drifted. Concrete problems, with evidence:

| # | Problem | Evidence |
|---|---------|----------|
| P1 | **File names track dev stages, not behavior** | `wave2_integration_test.go`, `wave3_integration_test.go`, `wave4_test.go`, `report_wave3_test.go`, `gates_wave3_test.go`. "Wave 4" means nothing to a reader looking for where tool-filtering is tested. |
| P2 | **Catch-all dumping-ground suffixes** | `dag_more_test.go`, `dag_regression_test.go`, `gates_regression_test.go`, `lock_regression_test.go`, `acp_store_scale_test.go`, `agent_perf_test.go`, `schema_sweep_test.go`. A behavior can live in any of three files; nothing says which. |
| P3 | **Two competing subtest-naming styles** | `t.Run("parent_traversal_rejected", …)` (snake_case) coexists with `t.Run("timeout yields 124 + timedOut", …)` (sentence case) in the same package. |
| P4 | **First-class harness is barely used** | `internal/testharness` is documented as "the testing infrastructure every specd test consumes," yet `testharness.New` appears in **3 of 118** test files. The other 115 roll their own setup. |
| P5 | **Helpers duplicated per-file** | `ids()`, `names()`, `td()`, `mkState()`, `newTestACPStore()`, `newPinkyHarness()` are redefined locally instead of living in one shared place. |
| P6 | **The harness itself is nearly untested** | `internal/testharness` is 1089 source LOC backed by **87** test LOC. The thing all other tests trust is the least guarded. |
| P7 | **Unit and integration tests are intermixed** | `internal/mcp` mixes pure-function tests with multi-component `*_integration_test.go` in one package, separated only by filename, with no build-tag selectability. |
| P8 | **Coverage sits well below documented targets** | overall 69.3% (target 85%), `internal/core` floor 60% (target 95%). The gap is unmapped — no inventory of *which* paths are dark. |

What is **good and must be preserved**:

- The hermetic discipline (`t.TempDir`, `t.Setenv`, injected `core.Clock`).
- The `backendConformance` shared suite run against every `StateBackend`.
- Visible `t.Skip(reason)` for unavailable backends — never a silent pass.
- The determinism strategy (`-count=2`, sort-before-emit, no golden churn).
- The coverage-as-ratchet policy (`scripts/coverage-check.sh`).
- The `slug_test.go` table-driven pattern — this is the **reference style**.

---

## 2. Target organization

### 2.1 Package layout (unchanged)

Tests stay in-package with the code they exercise (`package core`,
`package mcp`, …) so they can reach unexported symbols. We do **not** introduce a
separate `_test` package tree. The reorganization is within each package.

```
internal/
  core/          engine: state, lock, dag, gates, ears, acp, orchestration, …
  cmd/           CLI command glue + exit codes
  mcp/           MCP server, tools, resources, prompts, transport
  integration/   agent-harness adapters (claude, codex, cursor, gemini, vscode)
  cli/           arg parsing
  testharness/   SHARED test infrastructure (now also has its own tests)
main_test.go     top-level dispatch / exit-code smoke
```

### 2.2 One file per source unit — `<unit>_test.go`

Every test file maps to exactly one source file: `slug.go → slug_test.go`,
`lock.go → lock_test.go`, `tools.go → tools_test.go`. The test file holds **all**
tests for that unit — happy path, edge cases, regressions, and what used to be
scattered into `_more` / `_regression` / `_sweep`.

**Banned suffixes** (must be merged into the canonical unit file or, if genuinely
cross-cutting, see 2.3):

- `_more`, `_regression`, `_sweep`, `_scale`, `_perf` (as a *file* suffix),
  `_wave2/3/4`, numbered `wave*`.

A former "regression" test does not get a separate file; it becomes a subtest
inside the unit file with a name that states the bug it guards (see 2.4).

### 2.3 Cross-cutting suites get an explicit, named home

Some tests legitimately span units. These get **one** well-known file per
package, named for the *concern*, not a stage:

| Concern | File | Replaces |
|---------|------|----------|
| State-backend conformance | `core/backend_conformance_test.go` (keep) | — |
| Cross-process concurrency / race | `core/concurrency_test.go` (keep) | `lock_regression`, `state_cas` race bits |
| End-to-end CLI lifecycle | `cmd/lifecycle_test.go` (keep) | `wave*` cmd tests |
| MCP multi-component integration | `mcp/integration_test.go` (consolidate) | `wave2/3/4_integration`, `*_integration` |
| Agent-harness conformance | `integration/conformance_test.go` (keep) | per-host duplication |

### 2.4 Naming conventions (single scheme)

- **Test functions**: `TestUnitBehavior` — `TestValidateSlug`,
  `TestWithSpecLockReentrant`, `TestRunnableFrontier`. Benchmarks `BenchmarkX`,
  fuzz `FuzzX`, examples `ExampleX`.
- **Subtests**: `snake_case` describing the asserted outcome, not the input.
  ✅ `t.Run("rejects_path_traversal_with_usage_code", …)`
  ✅ `t.Run("reentrant_lock_does_not_deadlock", …)`
  ❌ `t.Run("timeout yields 124 + timedOut", …)` (spaces, sentence case)
  ❌ `t.Run("test2", …)` (says nothing)
- **Regression subtests** name the defect:
  `t.Run("regression_unreferenced_commits_not_dropped", …)`.
- **Table field names** are uniform: `name`, `input`/`args`, `want`,
  `wantErr`, `wantCode`.

### 2.5 Reference structure (every non-trivial test)

Table-driven, `Arrange / Act / Assert` comments, exactly as `slug_test.go`:

```go
func TestValidateSlug(t *testing.T) {
    tests := []struct {
        name     string
        slug     string
        wantErr  bool
        wantCode int // checked only when wantErr
    }{
        {"simple_lowercase_ok", "auth", false, 0},
        {"parent_traversal_rejected", "..", true, ExitUsage},
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Act
            err := ValidateSlug(tt.slug)
            // Assert
            ...
        })
    }
}
```

Single-assertion tests that gain nothing from a table may stay flat, but still
carry Arrange/Act/Assert markers when non-obvious.

---

## 3. Shared helpers — consolidate into `internal/testharness`

All cross-file helpers move into `testharness` (or a small package-local
`helpers_test.go` when they need unexported types from that package). The goal:
a test author never re-implements setup.

### 3.1 Promote to `testharness`

| Helper | Today | Target |
|--------|-------|--------|
| `New(t)` isolated root | exists, **underused** | default entry for every cmd/integration test |
| `SpecBuilder` (`.Title().Req()…Build()`) | exists | the only way to author fixture specs |
| `StateAsserter` (`.Status().Phase()…`) | exists | the only way to assert state.json |
| `FakeClock` | exists | mandatory for any time-dependent test |
| git fixtures (`InitGit/GitCommitAll/GitHead`) | exists | sole source of git fixtures |

### 3.2 De-duplicate local helpers

`ids()`, `names()`, `td()`, `mkState()`, `newTestACPStore()`,
`newPinkyHarness()`, `newOrchestrationMCPClient()`, `captureStderr` are each
defined in one file and copy-pasted elsewhere. Each becomes **one** definition:

- Generic, no specd types → exported `testharness` helper
  (`testharness.IDs`, `testharness.CaptureStderr`).
- Needs an unexported package type (`toolDef`, ACP internals) → one
  `helpers_test.go` in that package, shared by all `_test.go` in it.

### 3.3 Test the harness (P6)

`testharness` is load-bearing; it gets real tests:

- `New` produces an isolated, chdir'd root and restores cwd on cleanup.
- `SpecBuilder.Build()` emits a gate-valid spec (round-trip: build → load →
  assert).
- `FakeClock` advances deterministically and is restored on cleanup.
- `Run`/`RunExpect` capture stdout, stderr, and exit code independently.
- `StateAsserter` failures point at the right field.

Target: `internal/testharness` ≥ 80% statement coverage.

---

## 4. Test categories & selectability

Every test declares its category so CI can run fast feedback separately from
slow/external suites.

| Category | Marker | Runs in | Notes |
|----------|--------|---------|-------|
| **Unit** | default (no tag, no skip) | every `go test ./...` | pure functions, in-process, < 50ms each |
| **Integration** | `t.Run` under a `*_integration_test.go` file **and** in-process | default suite | multi-component, still hermetic |
| **Conformance** | shared suite over a table (backends, hosts) | default; rows `t.Skip(reason)` when dep absent | never silent-pass |
| **Stress / race** | `-race`, cross-process via `scripts/stress*.sh` | dedicated CI job | unchanged |
| **External-backend** | `//go:build specd_postgres` / `specd_redis` | opt-in build tag + env | unchanged |
| **Benchmark / determinism** | `Benchmark*`, `-run Deterministic -count=2` | `make perf-gate` | byte-stable receipts |

Rule: a test that needs the network, a real service, or > 1s wall-clock **must**
be tag-gated or skip-with-reason. The default `go test ./...` stays hermetic and
fast.

---

## 5. Coverage contract

Coverage is a **ratchet**, raised here, never lowered without written PR
justification (`scripts/coverage-check.sh` enforces the floor).

| Scope | Current | New floor (this work) | Long-term target |
|-------|---------|----------------------|------------------|
| overall | 69.3% | **70%** | 85% |
| `internal/core` (engine) | ~60–70% | **70%** | 95% |
| `internal/testharness` | ~8% | **80%** | 90% |
| `internal/mcp` | — | **70%** | 85% |

**100% held** on integrity-critical functions (no regression permitted):
`ValidateSlug`, `SpecdError` constructors, `WithSpecLock`, `LoadState`. The
dark-path inventory (Task group D) names every currently-uncovered branch in
`internal/core` and assigns it to a test or an explicit "won't test, here's why"
note (e.g. disk-error branches needing fault injection).

---

## 6. Determinism & isolation invariants (preserved, now enforced uniformly)

1. **Hermetic**: `t.TempDir()` + `t.Setenv()` only. No host git config, no real
   `~/.specd`, no network. The default suite passes with the machine offline.
2. **Time injected**: any time-dependent path uses `FakeClock`; no
   `time.Now()` assertions.
3. **Order-independent**: `go test ./... -count=2` is green. Anything assembled
   from a map is sorted before emit/assert.
4. **Parallel discipline**: tests that `Chdir` or touch shared lock/global state
   (`os.Stdout`, `core.Clock`) are **not** `t.Parallel()`. Pure-function tests
   with no shared state **should** be `t.Parallel()` — explicitly, so the
   default is documented, not accidental.
5. **Race-clean**: lock/state/concurrency tests pass under `-race`;
   `concurrency_test.go` is designed to fail under `-race` on a CAS+lock
   regression.

---

## 7. Definition of done

- [ ] No test file uses a banned suffix (`_more`, `_regression`, `_sweep`,
      `_scale`, `wave*`); every file maps to one source unit or a named
      cross-cutting suite (§2.2–2.3).
- [ ] All subtests use `snake_case` outcome names; zero space-separated names
      (greppable check in CI: `grep 't.Run("[^"]* '` returns nothing).
- [ ] No duplicated helper bodies; `ids/names/td/mkState/…` each defined once
      (§3.2).
- [ ] `testharness` has its own tests at ≥ 80% (§3.3).
- [ ] Every cmd/integration test sets up via `testharness.New` + `SpecBuilder`
      (§3.1) unless it documents why it can't.
- [ ] `go test ./... -race -count=1` green; `-count=2` green; `make ci` green.
- [ ] Coverage floors raised to §5 and met; integrity-critical 100% intact.
- [ ] Dark-path inventory exists and every entry is tested or annotated (§5).
- [ ] `TESTING.md` updated to describe the new layout and conventions.

---

## 8. Non-goals

- No change to specd runtime behavior or public CLI/MCP contracts.
- No new test framework or assertion library — stdlib `testing` only.
- No golden-file fixtures introduced (content assertions stay; §6.3).
- No CI topology change beyond what §4 selectability requires.
- Not chasing 100% line coverage on print-formatting glue.
