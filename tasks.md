# tasks.md — specd Quality Regression Execution Plan

Companion to [`spec.md`](spec.md). Tasks are grouped into dependency **waves**:
every task in a wave may run in parallel; a wave starts only when the prior wave
is complete. Each task lists its `role`, `depends`, traced `requirements`, and a
deterministic `verify:` command (specd's evidence-gate idiom).

Legend — role: `coder` (writes prod code), `tester` (writes tests),
`investigator`/`reviewer` (read-only, `verify: N/A`).
Verify baseline for code/test tasks: `make ci` unless a tighter command is given.

---

## Wave 1 — Correctness fixes (bug-first, land with their tests)

- [ ] **T1 — Fix `BuildProgram` nil-state TOCTOU deref**
  - role: coder · depends: — · requirements: R1.1
  - Guard the `LoadState` result in `internal/core/program.go:168-172`: when a
    listed spec's `state.json` has vanished, return a `GateError` (or skip with a
    recorded orphan) instead of dereferencing `state.Status`.
  - verify: `go test ./internal/core/ -run TestBuildProgram -race -count=1`

- [ ] **T2 — Test: `BuildProgram` with a spec whose state.json is missing**
  - role: tester · depends: — · requirements: R1.1
  - Add a case that deletes `state.json` after scaffolding and asserts a clean
    error (no panic). Pairs with T1 (write red, then T1 makes it green).
  - verify: `go test ./internal/core/ -run TestBuildProgram -race -count=1`

- [ ] **T3 — Harden `io.go` error paths (`AtomicWrite`, `AppendFile`)**
  - role: coder · depends: — · requirements: R1.2
  - Audit `MkdirAll`/`WriteString`/`Sync`/`Rename`/append for swallowed errors;
    ensure a partial write never returns nil.
  - verify: `make build && go vet ./...`

- [ ] **T4 — Test: `io.go` failure modes**
  - role: tester · depends: — · requirements: R1.2
  - Cover parent-is-a-file (`MkdirAll` fail), unwritable dir, and `AppendFile`
    error propagation; assert no temp file left behind on failure.
  - verify: `go test ./internal/core/ -run 'TestAtomicWrite|TestAppend' -race -count=1`

---

## Wave 2 — Coverage of integrity-critical paths

- [ ] **T5 — Test: `LoadProgram`/`SaveProgram` round-trip + corrupt input**
  - role: tester · depends: T1,T2 · requirements: R2.1
  - New `internal/core/program_test.go`: corrupt JSON → gate error, duplicate-edge
    dedup, empty-edge pruning, save→load stability.
  - verify: `go test ./internal/core/ -run TestProgram -race -count=2`

- [ ] **T6 — Test: DAG `RunnableFrontier`/`GroupWaves`/`WaveViolations`**
  - role: tester · depends: — · requirements: R2.2
  - Table tests for frontier ordering (wave then ordinal), exclusion of
    blocked/running tasks, and later-wave dependency violations.
  - verify: `go test ./internal/core/ -run TestDag -race -count=2`

- [ ] **T7 — Test: `DetectCycle` path content + `CriticalPath` invariant**
  - role: tester · depends: — · requirements: R2.3
  - Assert the cycle node sequence (self-loop, multi-node + acyclic tail) and the
    invariant `CriticalPath==nil iff DetectCycle!=nil`.
  - verify: `go test ./internal/core/ -run 'TestDetectCycle|TestCriticalPath' -race -count=2`

- [ ] **T8 — Test: `LintEars` acceptance-criteria state machine**
  - role: tester · depends: — · requirements: R2.4
  - Cover criterion-before-marker, mixed-validity multi-blocks, and line numbers.
  - verify: `go test ./internal/core/ -run TestLintEars -race -count=1`

- [ ] **T9 — Test: `MergeSection`/`MergeAgentsMD` idempotency & marker edges**
  - role: tester · depends: — · requirements: R2.5
  - New `internal/core/agents_test.go`: re-merge is idempotent, marker-absent
    append, file creation, malformed-marker fallback, out-of-marker preserved.
  - verify: `go test ./internal/core/ -run TestMerge -race -count=2`

- [ ] **T10 — Test: `buildBrief` across all status branches**
  - role: tester · depends: — · requirements: R2.6
  - verify: `go test ./internal/cmd/ -run TestBuildBrief -race -count=1`

- [ ] **T11 — Test: `--json` schema stability via `Unmarshal`**
  - role: tester · depends: — · requirements: R2.7
  - For dispatch/status/context/next/program: unmarshal into the expected struct,
    assert key fields (not substrings).
  - verify: `go test ./internal/cmd/ -run TestJSON -race -count=1`

---

## Wave 3 — Coverage ratchet (after new tests land)

- [ ] **T12 — Raise coverage floors to new measured baseline**
  - role: coder · depends: T5,T6,T7,T8,T9,T10,T11 · requirements: R2.8
  - Re-measure overall + `internal/core`; bump `OVERALL_MIN`/`CORE_MIN` in
    `scripts/coverage-check.sh` to just below measured. Never lower a floor.
  - verify: `./scripts/coverage-check.sh`

---

## Wave 4 — Structure & duplication refactors (behavior byte-identical)

- [ ] **T13 — Extract `requireRootAndSlug` command prologue helper**
  - role: coder · depends: T11 · requirements: R3.1
  - Collapse the ~14 duplicated root+slug+empty-check prologues into
    `internal/cmd/helpers.go`.
  - verify: `make ci`

- [ ] **T14 — Extract approval-gate check helper**
  - role: coder · depends: T11 · requirements: R3.2
  - Unify the identical `{"kind":"gated"}` blocks in task/dispatch/next.
  - verify: `make ci`

- [ ] **T15 — Unify frontier reason switch + task-view merge in `core`**
  - role: coder · depends: T6,T11 · requirements: R3.3
  - Add `core.ResolveTaskView(doc,state,id)` and a shared no-runnable reason
    renderer; call from dispatch.go and next.go.
  - verify: `make ci`

- [ ] **T16 — Split `RunVerify` execution from presentation**
  - role: coder · depends: T11 · requirements: R3.4
  - Mirror the already-factored `runVerifyCommand`; do not weaken the evidence
    gate.
  - verify: `make ci`

- [ ] **T17 — Typed JSON payload structs + small cleanups**
  - role: coder · depends: T11 · requirements: R3.5,R3.6
  - Replace stable `map[string]interface{}` payloads with named structs; swap
    `tasksparser.go` insertion sort for `sort.Ints`; share annotation-suffix
    rendering between `serializeTask`/`RenderTaskLine`.
  - verify: `make ci`

---

## Wave 5 — CI / tooling / supply chain

- [ ] **T18 — Add `govulncheck` + `staticcheck` jobs and `.golangci.yml`**
  - role: coder · depends: — · requirements: R4.1
  - verify: `go run golang.org/x/vuln/cmd/govulncheck@latest ./...`

- [ ] **T19 — Pin floating action refs + add `dependabot.yml`**
  - role: coder · depends: — · requirements: R4.2
  - Pin `ludeeus/action-shellcheck@master` to a tag/SHA; add
    `.github/dependabot.yml` for `github-actions`.
  - verify: `N/A` (read-only config; reviewer confirms) — complete with
    `--unverified --evidence "<workflow diff + actionlint output>"`

- [ ] **T20 — Release workflow runs full `make ci`**
  - role: coder · depends: — · requirements: R4.3
  - verify: `N/A` — complete with `--unverified --evidence "<release.yml diff>"`

- [ ] **T21 — Add `toolchain` directive + Go version matrix on test job**
  - role: coder · depends: — · requirements: R4.4
  - verify: `go build ./... && go vet ./...`

---

## Wave 6 — Docs, hygiene & scripts

- [ ] **T22 — Fix AGENTS.md/TESTING.md boot/enrich drift**
  - role: coder · depends: — · requirements: R5.1
  - verify: `N/A` — complete with `--unverified --evidence "grep shows no boot/enrich in layout/harness tables"`

- [ ] **T23 — Reconcile coverage-floor numbers (script ↔ TESTING.md)**
  - role: coder · depends: T12 · requirements: R5.2
  - verify: `./scripts/coverage-check.sh`

- [ ] **T24 — Add SECURITY.md, CONTRIBUTING.md, issue/PR templates**
  - role: coder · depends: — · requirements: R5.3
  - verify: `N/A` — complete with `--unverified --evidence "files present, links valid"`

- [ ] **T25 — Fix README/install.sh version examples + next semver tag note**
  - role: coder · depends: — · requirements: R5.4
  - verify: `N/A` — complete with `--unverified --evidence "version refs corrected"`

- [ ] **T26 — Fix `install.sh` portable colors + exec bits**
  - role: coder · depends: — · requirements: R6.1
  - Use `printf '%b'`/`tput`; `chmod +x install.sh uninstall.sh`.
  - verify: `shellcheck scripts/*.sh && sh -n scripts/install.sh`

- [ ] **T27 — godoc breadth, stress.sh python3 warning, goreleaser SBOM**
  - role: coder · depends: — · requirements: R5.5,R6.2,R6.3
  - verify: `shellcheck scripts/*.sh && go vet ./...`

---

## Traceability summary

| Wave | Tasks | Requirements | Theme |
|------|-------|--------------|-------|
| 1 | T1–T4 | R1.1, R1.2 | Correctness fixes |
| 2 | T5–T11 | R2.1–R2.7 | Integrity-path tests |
| 3 | T12 | R2.8 | Coverage ratchet |
| 4 | T13–T17 | R3.1–R3.6 | Structure refactors |
| 5 | T18–T21 | R4.1–R4.4 | CI / supply chain |
| 6 | T22–T27 | R5.1–R5.5, R6.1–R6.3 | Docs / scripts / packaging |

**Suggested first cut (highest ROI):** T1–T4 (a real bug + its safety net),
T18 (`govulncheck`/`staticcheck`), T22 (doc drift). These are low-risk,
high-signal, and unblock the rest.
