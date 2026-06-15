# spec.md — specd Codebase Quality & Structure Regression

**Status:** proposed
**Author:** regression analysis (branch `regression`)
**Date:** 2026-06-15
**Scope:** the `specd` development repo itself (Go CLI + prompt pack), not target repos.

---

## 1. Objective

Perform a full regression on `specd`'s code quality and structure and define the
concrete improvements that raise the tool to its next quality tier. This spec
captures the findings of a three-axis analysis — **code structure**, **test
coverage & robustness**, and **CI / tooling / docs / security / packaging** — and
turns them into approved, traceable requirements. Companion execution plan lives
in [`tasks.md`](tasks.md).

> Guiding constraint: preserve specd's hard invariants — **stdlib-only, zero LLM
> calls, deterministic output, the Foundational Split, and the evidence gate.**
> No change here may weaken `internal/cmd/task.go` (the integrity core) or add a
> runtime dependency.

---

## 2. Baseline assessment (what is already good)

The regression confirmed the codebase is unusually disciplined. These are
**non-goals to preserve**, not problems:

- **Clean layering.** `main → cli/cmd/core`; `core` imports nothing upward; no
  circular-dependency risk.
- **Concurrency is sound.** Reentrant per-spec advisory lock (`lock.go`,
  goroutine-id keyed), `SaveState` revision-CAS with an `assertLocked` debug
  guard, atomic temp+fsync+rename writes. Covered by `concurrency_test.go`
  (32-goroutine serialization + panic-unwind), `lock_test.go`, `state_cas_test.go`.
- **Security controls exist and match the documented model.** `verify` runs via
  `sh -c` with an env allowlist + NUL rejection + printed command/cwd
  (`verify.go`), slug path validation (`slug.go`), fail-closed SHA256 in
  `update.go` / `install.sh`, `EnvInt` clamp+warn-once.
- **Integrity spine is behaviorally tested.** All 7 gates, parser round-trip
  byte-stability, evidence gate, cycle detection.
- **Toolchain is green.** `go vet`, `gofmt`, race suite, `-count=2`
  order-dependence, coverage floor, and cross-process stress all pass in CI.

**Current measured baseline:** 60.0% total statement coverage; enforced floors
59% overall / 49% `internal/core` (per-package measurement understates real
coverage — CLI tests exercise much of `core` cross-package).

---

## 3. Findings → requirements (EARS)

Requirements use EARS phrasing for consistency with specd's own gate. Each maps
to tasks in `tasks.md` and to a severity from the regression (H/M/L).

### 3.1 Correctness & robustness

- **R1.1 (H)** When `specd program` builds the program graph while a spec's
  `state.json` has been deleted concurrently, the system shall return a gate
  error rather than panic. *(BuildProgram dereferences a nil state at
  `internal/core/program.go:172`; `LoadState` returns `(nil, nil)` for a missing
  file — a TOCTOU nil-deref.)*
- **R1.2 (H)** While performing atomic writes and append-only ledger writes, the
  system shall propagate `MkdirAll`, `Sync`, `Rename`, and append failures to the
  caller and shall never report success on a partial write. *(`io.go` failure
  paths are untested; `AppendFile` has zero error-path coverage.)*

### 3.2 Test coverage of integrity-critical paths

- **R2.1 (H)** Where cross-spec program state is loaded or saved, the system
  shall be covered by tests for corrupt-input rejection, edge dedup, empty-edge
  pruning, and save→load round-trip stability. *(`LoadProgram`/`SaveProgram` only
  exercised via one CLI happy path.)*
- **R2.2 (M)** The DAG scheduler functions `RunnableFrontier`, `GroupWaves`, and
  `WaveViolations` shall have direct unit tests asserting frontier ordering,
  exclusion of blocked/running tasks, and later-wave dependency detection.
- **R2.3 (M)** The system shall assert the **content** of `DetectCycle`'s returned
  path (self-loop, multi-node cycle with acyclic tail) and shall verify the
  documented invariant that `CriticalPath` returns nil iff a cycle exists.
- **R2.4 (M)** `LintEars` shall be tested for its acceptance-criteria state
  machine: criterion lines before the marker, multiple mixed-validity blocks, and
  correct line numbers.
- **R2.5 (M)** `MergeSection` / `MergeAgentsMD` shall be tested for idempotency,
  marker-absent append, file creation, malformed-marker fallback, and
  preservation of out-of-marker content. *(These mutate the user's `AGENTS.md`.)*
- **R2.6 (M)** `buildBrief` shall be tested across all status branches, including
  the blocked sub-conditional and the unknown-status default.
- **R2.7 (M)** For every command with `--json` output, at least one test shall
  `json.Unmarshal` into the expected struct and assert key fields, locking the
  machine-readable contract beyond substring checks.
- **R2.8 (M)** After R2.1–R2.7 land, the coverage floors shall be raised to sit
  just below the new measured coverage (ratchet, never lowered to pass a build).

### 3.3 Code structure & duplication

- **R3.1 (M)** The repeated per-command prologue (root locate + slug extraction +
  empty check, duplicated across ~14 handlers) shall be consolidated into a shared
  helper in `internal/cmd/helpers.go`.
- **R3.2 (M)** The awaiting-approval gate check (byte-identical
  `{"kind":"gated"}` block in `task.go`, `dispatch.go`, `next.go`) shall be
  extracted into one helper.
- **R3.3 (M)** The frontier "no-runnable reason" switch and the
  doc-overrides-state task-view merge (duplicated in `dispatch.go` and `next.go`)
  shall be unified behind a `core` accessor.
- **R3.4 (M)** `RunVerify` shall separate subprocess execution and state mutation
  from presentation, mirroring the already-factored `runVerifyCommand`.
- **R3.5 (L)** Ad-hoc `map[string]interface{}` JSON payloads (~22 sites) shall be
  migrated to named structs where a stable schema exists (gated/frontier
  payloads), following the pattern in `status.go`/`program.go`.
- **R3.6 (L)** The hand-rolled insertion sort in `tasksparser.go:362` shall use
  `sort.Ints`; the duplicated annotation-suffix rendering
  (`serializeTask`/`RenderTaskLine`) shall share one helper.

### 3.4 CI, tooling & supply chain

- **R4.1 (H)** The CI pipeline shall run `govulncheck ./...` and a static
  analyzer (`staticcheck`), governed by a checked-in `.golangci.yml`. *(Only
  gofmt/vet/shellcheck run today; the tool executes agent-authored shell and
  self-updates over the network.)*
- **R4.2 (M)** All GitHub Actions references shall be pinned (no floating
  `@master`, e.g. `ludeeus/action-shellcheck`), and a `dependabot.yml` shall keep
  the `github-actions` ecosystem current.
- **R4.3 (M)** The release workflow shall run the full `make ci` gate (or depend
  on a green CI run for the commit) before publishing a tag — not only
  `go test -race`.
- **R4.4 (M)** `go.mod` shall declare an explicit `toolchain`, and the test job
  shall run a Go version matrix (declared minimum + `stable`).

### 3.5 Documentation & repo hygiene

- **R5.1 (H)** `AGENTS.md` (repo layout) and `TESTING.md` (harness table) shall be
  corrected to remove references to the deleted `boot`/`enrich` subsystem.
- **R5.2 (M)** The coverage-floor numbers in `scripts/coverage-check.sh` and
  `TESTING.md` shall be reconciled (script says 59/49; docs say 60/58).
- **R5.3 (M)** The repo shall add `SECURITY.md` (disclosure contact + the existing
  threat model), `CONTRIBUTING.md` (pointer to the contributor guide), and
  issue/PR templates.
- **R5.4 (M)** Version examples in `README.md` and `install.sh` shall match
  reality, and the next tag shall be semver-correct (`0.2.0`+) for the breaking
  boot/enrich removal.
- **R5.5 (L)** Exported symbols in the public-facing surfaces shall gain godoc
  comments where missing (breadth pass).

### 3.6 Scripts & packaging

- **R6.1 (H)** `install.sh` shall render colors portably under its declared
  `/bin/sh` shebang (`printf '%b'`/`tput`, not `\033` in the format string), and
  `install.sh`/`uninstall.sh` shall carry the executable bit for consistency.
- **R6.2 (L)** `stress.sh` shall warn (not silently skip) when `python3` is
  absent and its JSON torn-write check cannot run.
- **R6.3 (L)** `.goreleaser.yml` shall optionally add SBOM and reproducible-build
  (`mod_timestamp`) settings; signing remains a documented deferral.

---

## 4. Design / approach

1. **Bug-first, then tests, then refactor, then tooling, then docs.** Fixes that
   change behavior (R1.x) land with their regression tests so the fix is proven.
   Pure refactors (R3.x) ride on top of the strengthened test net, never before
   it.
2. **Mechanical extractions only for structure.** R3.1–R3.4 are extract-method
   refactors guarded by `TestRegistryMatchesHelp` and the existing CLI suite;
   behavior must be byte-identical (verified by `-count=2` and JSON tests).
3. **Ratchet, don't chase the number.** Coverage floors (R2.8) move only after new
   tests exist, and only upward.
4. **No new dependencies at runtime.** Linters/scanners are CI-only dev tooling;
   the shipped binary stays stdlib-only.
5. **Every task carries a deterministic `verify:`** so completion is
   evidence-gated, in keeping with specd's own model.

---

## 5. Non-goals

- No change to the spec lifecycle, gate semantics, exit-code contract, or the
  evidence gate's strictness.
- No reintroduction of `boot`/`enrich` or any in-binary repo perception.
- No runtime dependencies; no LLM calls; no on-disk template reads.
- No large architectural rewrite — the layering is sound and stays.

---

## 6. Acceptance criteria

- `make ci` passes on every change (lint + race + `-count=2` + coverage floor +
  stress).
- The `BuildProgram` nil-deref (R1.1) and `io.go` error paths (R1.2) are fixed and
  covered by failing-then-passing tests.
- New tests exist for R2.1–R2.7; coverage floors raised (R2.8) without lowering
  any existing floor.
- Refactors R3.1–R3.4 leave all CLI/JSON output byte-identical.
- CI runs `govulncheck` + `staticcheck` (R4.1); no floating action refs (R4.2).
- `AGENTS.md`/`TESTING.md` drift removed (R5.1); coverage docs reconciled (R5.2).
- `install.sh` colors render under `/bin/sh` (R6.1).
