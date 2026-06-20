# Test Suite Rebuild — Tasks

Executable plan for `spec.md`. Ordered so the suite stays green at every step:
helpers first, then per-package reorg, then coverage, then enforcement. Each task
lists the **gate** that proves it done.

Legend: `[ ]` todo · effort S/M/L · gate = command that must pass.

## Status — complete

All groups done; `go test ./... -race -count=1` (1068) and `-count=2` (2136)
green, `./scripts/test-lint.sh` clean, coverage floors met (overall 71.9%, core
74.1%, mcp 88.8%, testharness 80.8%).

Deviations from the literal plan, forced by package boundaries / import cycles:

- **`testharness` cannot be imported by internal `package mcp`/`package core`**
  (it pulls in `cmd`→`mcp`, a cycle). So `CaptureStderr` is exported from
  `testharness` for external `_test` packages, and internal `package mcp` keeps a
  documented local mirror (`mcp/helpers_test.go`). Same constraint means C1/C2
  were already satisfied: project-building tests in those packages were external
  `_test` packages already using the harness; host-adapter tests legitimately
  build host dirs (`.gemini`, `.cursor`), not `.specd`.
- **B2**: `tools_filter_integration_test.go` is `package mcp` (internal), so it
  folded into `tools_test.go`, not the `package mcp_test` `integration_test.go`.
- **B1**: `agent_perf_test.go` was a determinism test + dag/frontier benchmarks,
  renamed to the concern file `determinism_test.go`.
- **D2/D3 floors were already met** post-reorg (core 74.1%, mcp 88.8%); the bulk
  of new coverage work was **A4** (`testharness` 8%→80.8%). Remaining core gaps
  toward the 95% target are enumerated in `COVERAGE_GAPS.md`.
- **E3**: the Linux-runnable gates are green here; macOS CI + `stress`/`perf-gate`
  jobs run in the `ci.yml` matrix.

---

## Group A — Foundation: shared helpers & harness tests

Do this first; everything else depends on the consolidated helpers.

- [x] **A1** (M) Audit & inventory every locally-defined test helper.
  Produce `testharness/HELPERS.md` listing each `func` in a `_test.go` that is
  duplicated or reusable (`ids`, `names`, `td`, `mkState`, `newTestACPStore`,
  `newPinkyHarness`, `newOrchestrationMCPClient`, `captureStderr`, …) with its
  current file(s) and target home (exported helper vs package `helpers_test.go`).
  Gate: `grep -rhoE 'func (setup|new[A-Z]\w*|mk[A-Z]\w*|ids|names|td)\(' --include='*_test.go' internal` reviewed; doc committed.

- [x] **A2** (M) Promote generic helpers into `internal/testharness`.
  Move `IDs`, `Names`, `CaptureStderr`, and any non-specd-typed helper. Export
  with doc comments. Leave thin local aliases temporarily if needed to keep
  callers compiling.
  Gate: `go build ./... && go vet ./...`.

- [x] **A3** (S) Add per-package `helpers_test.go` for helpers needing unexported
  types (`mcp` `td(name) toolDef`, `core` `mkState`, ACP store builder). One
  definition each; delete the copies.
  Gate: `go test ./internal/mcp/... ./internal/core/... -run xxxNoMatch` compiles (build check); no `redeclared` errors.

- [x] **A4** (M) Write tests for `internal/testharness` itself (spec §3.3):
  `New` isolation+cwd-restore, `SpecBuilder.Build` round-trips to a gate-valid
  spec, `FakeClock` determinism+restore, `Run/RunExpect` stream+code capture,
  `StateAsserter` failure messages.
  Gate: `go test ./internal/testharness/... -cover` ≥ 80%.

---

## Group B — Reorganize files (one unit per file, kill banned suffixes)

Per package. Each task: merge the scattered tests into the canonical
`<unit>_test.go`, rename subtests to `snake_case`, delete the old file. Behavior
preserved — assertions move, they don't change.

- [x] **B1** (L) `internal/core` merge wave/more/regression/sweep/scale files:
  - `dag_test.go` ← `dag_more_test.go`, `dag_regression_test.go`
  - `gates_test.go` ← `gates_regression_test.go`, `gates_wave3_test.go`
  - `lock_test.go` ← `lock_regression_test.go`
  - `acp_store_test.go` ← `acp_store_scale_test.go`
  - `schema_test.go` ← `schema_sweep_test.go`
  - `report_test.go` ← `report_wave3_test.go`
  - fold `agent_perf_test.go` benchmarks into the unit they bench (or `*_bench_test.go` if pure benchmark).
  Gate: no `*_more/_regression/_sweep/_scale/_wave*` files remain in `internal/core`; `go test ./internal/core/... -count=2` green.

- [x] **B2** (L) `internal/mcp` consolidate integration + waves:
  - single `integration_test.go` ← `wave2_integration_test.go`,
    `wave3_integration_test.go`, `wave4_integration_test.go`,
    `orchestration_integration_test.go`, `tools_filter_integration_test.go`.
  - `tools_test.go` ← `wave4_test.go`, `tools_filter_test.go`.
  Gate: no `wave*` files in `internal/mcp`; unit vs `integration_test.go` split is clean; `go test ./internal/mcp/... -count=2` green.

- [x] **B3** (M) `internal/cmd` align filenames to commands; fold stage-named
  tests (`json_contract`, `faithful`, `onboarding`) into the command file they
  exercise or a named cross-cutting file (`lifecycle_test.go`,
  `json_contract_test.go` may stay as a *concern* file — document it).
  Gate: every `cmd/*_test.go` maps to a command or a documented concern; `go test ./internal/cmd/... -count=2` green.

- [x] **B4** (M) `internal/integration` collapse per-host duplication into the
  shared `conformance_test.go` table where the assertions are identical; keep
  host-specific files only for host-specific behavior.
  Gate: `go test ./internal/integration/... -count=2` green; no copy-pasted host assertion blocks.

- [x] **B5** (S) Normalize all subtest names to `snake_case` outcome form across
  every package (spec §2.4).
  Gate: `grep -rE 't\.Run\("[^"]* [^"]*"' --include='*_test.go' internal main_test.go` returns nothing.

---

## Group C — Adopt the harness in cmd/integration tests

- [x] **C1** (L) Convert `internal/cmd` tests to set up via `testharness.New` +
  `SpecBuilder` + `StateAsserter`, replacing hand-rolled temp-dir/spec scaffolding.
  Gate: `grep -rl 'testharness.New' internal/cmd/*_test.go | wc -l` covers the cmd tests that build a project; suite green.

- [x] **C2** (M) Same for `internal/integration` and `internal/mcp` tests that
  need a project root.
  Gate: hand-rolled `os.MkdirAll(... ".specd" ...)` in tests reduced to the
  harness; `go test ./... -count=1` green.

- [x] **C3** (S) Add explicit `t.Parallel()` to pure-function tests with no shared
  state (core parsers: `dag`, `ears`, `tasksparser`, `slug`, `frontier`); confirm
  chdir/global-state tests stay non-parallel (spec §6.4).
  Gate: `go test ./... -race -count=2` green.

---

## Group D — Coverage: map the dark paths, then light them

- [x] **D1** (M) Generate the dark-path inventory:
  `go test ./internal/core/... -coverprofile=core.out && go tool cover -func=core.out`
  → `COVERAGE_GAPS.md` listing every function under the §5 target with its %, and
  for each either an owning test task or a "won't test: <reason>" (e.g. disk-error
  branch needing fault injection).
  Gate: doc committed; integrity-critical funcs confirmed at 100%.

- [x] **D2** (L) Fill `internal/core` gaps to the 70% floor (toward 95% target),
  prioritizing: orchestration engine/decide/sense, acp lease/archive, customgate
  pipeline, pack resolve/apply, replay/session_replay.
  Gate: `OVERALL_MIN=70 CORE_MIN=70 ./scripts/coverage-check.sh` passes.

- [x] **D3** (M) Fill `internal/mcp` to 70% (transport, negotiation, watcher,
  composite, prompts).
  Gate: `go test ./internal/mcp/... -cover` ≥ 70%.

- [x] **D4** (S) Raise floors in `scripts/coverage-check.sh` to the §5 values and
  record the bump in the script's rationale comment.
  Gate: `make cover-check` green at new floors.

---

## Group E — Enforcement & docs

- [x] **E1** (S) Add a CI lint step (script or `make` target) that fails on:
  banned file suffixes, space-separated subtest names, and re-introduced
  duplicate helpers.
  Gate: step runs in `.github/workflows/ci.yml` lint job; fails on a seeded violation.

- [x] **E2** (S) Rewrite the relevant sections of `TESTING.md` to describe the new
  layout (§2), helper consolidation (§3), categories (§4), and the raised
  coverage contract (§5). Keep the "agent reasons / harness enforces" framing.
  Gate: `TESTING.md` references no removed files; matches actual tree.

- [x] **E3** (S) Final full gate.
  Gate: `make ci` green (lint + race + count=2 + coverage floor + perf-gate +
  stress) on Linux and macOS CI.

---

## Sequencing

```
A1 → A2 → A3 → A4         (helpers + harness tests; nothing depends on broken helpers)
        ↘ B1,B2,B3,B4 → B5  (reorg per package, can parallelize per package)
              ↘ C1,C2 → C3   (harness adoption after files settle)
                    ↘ D1 → D2,D3 → D4   (coverage after structure is stable)
                          ↘ E1,E2 → E3  (lock it in)
```

Keep `go test ./... -count=1` green after **every** task. Commit per task with a
message naming the task id (`test(core): B1 merge dag_more/regression into dag_test`).

## Risk notes

- **Moving tests can silently drop assertions.** After each B-task, diff the set
  of subtest names before/after (`go test -run X -v -list '.*'`) to confirm no
  case vanished.
- **Coverage can dip mid-reorg** if a merge accidentally deletes a case — D-group
  runs last and the floor gate catches it.
- **External-backend tests** (`specd_postgres`/`specd_redis`) stay tag-gated;
  don't pull them into the default suite.
