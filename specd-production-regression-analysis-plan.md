# Analysis and Action Plan: specd Production Regression Planning

## 1. Intent and Success

**Requested outcome:** Identify, catalog, and plan a comprehensive regression test strategy covering every development area that must be reviewed and hardened before `specd` can be released for production use.

**Task class:** Hardening + Regression Planning (pre-production readiness assessment)

**Constraints:**
- Must cover all load-bearing surfaces: CLI commands, state management, concurrency, security, MCP server, sandboxing, reporting, installation, CI/CD, documentation
- Must be evidence-based from the repository; no invented claims
- Must produce implementation-ready specs and tasks for a coding agent

**Measurable definition of success:**
- Every identified regression area has a mapped spec with acceptance criteria
- Every spec has atomic, dependency-ordered tasks with validation commands
- Coverage includes: functional correctness, concurrency safety, security boundaries, cross-platform compatibility, performance baselines, install integrity, documentation accuracy, and CI reliability

---

## 2. Scope and Decisions

**In scope:**
- All 13 CLI commands and their flag surfaces
- State persistence, atomicity, CAS, and lock correctness
- DAG scheduling, task frontier computation, and wave execution
- Verification execution (shell, bwrap, container) and sandbox isolation
- MCP stdio/HTTP/SSE server and tool dispatch
- Agent onboarding (`init`, pack application, host detection, MCP registration)
- Report generation (Markdown, HTML, PR summary, dashboard)
- Installation script integrity and checksum verification
- CI/CD pipeline (lint, test, coverage, stress, release)
- Cross-platform builds (linux/darwin/windows × amd64/arm64)
- Security boundaries (env scrubbing, path traversal, auth, isolation)
- Documentation accuracy against code behavior

**Out of scope:**
- External agent behavior (agents are out of specd's control)
- LLM API interactions (specd is stdlib-only, no LLM calls)
- Production deployment orchestration beyond the binary/installer

**Assumptions:**
- A1: The existing 7 optimization specs in `specs/` represent the current known improvement areas; the regression plan must verify they don't introduce regressions
- A2: `go test ./...` is the primary validation harness; shell stress scripts are secondary
- A3: The release target is a v1.0.0 stable release

**Decision gates:**
- G1: Should the regression plan include fuzz testing? (Decision: yes, for parser and state surfaces)
- G2: Should we add a dedicated integration test for the full `new → check → approve → verify → complete` lifecycle? (Decision: yes)
- G3: Should Windows-specific paths be covered by CI stress tests? (Decision: build-only in CI; full stress requires WSL or Windows runner)

---

## 3. Repository Context

**Stack:** Go 1.22, stdlib-only (no external dependencies), POSIX shell scripts, GitHub Actions

**Architecture:**
- `main.go` — CLI entry point, command routing, `--json` bridging, version propagation
- `internal/cli/` — argument parsing (`cli.Args`)
- `internal/cmd/` — 13 command implementations: `check`, `verify`, `approve`, `next`, `init`, `mcp`, `report`, `task`, `status`, `new`, `midreq`, `migrate`, `doctor` (removed)
- `internal/core/` — state management, gates, DAG, lock, CAS, config, templates, report rendering
- `internal/runner/` — verify execution backends (`shRunner`, `bwrap`, `container`)
- `internal/worker/` — Brain/Pinky worker process execution
- `internal/mcp/` — MCP stdio/HTTP/SSE server, tool registry, phase watcher
- `internal/pack/` — embedded scaffold bundles
- `internal/schema/` — JSON Schema generation and validation
- `internal/spec/` — spec status constants, parsing
- `internal/integration/` — host agent detection and registration
- `internal/obs/` — telemetry/metrics recording
- `internal/context/` — context budget management

**Key conventions:**
- Exit codes: `0=ok`, `1=gate`, `2=usage`, `3=not-found` (`E1` from `main.go`)
- State stored in `.specd/specs/<slug>/state.json` with advisory file lock + revision CAS
- Schema version currently `5` (`E2` from `internal/core/state.go`)
- All state mutations atomic via `WithSpecLock` (`E3` from `internal/core/lock.go`)

**Existing specs (in `specs/`):**
| Spec | Focus | Status |
|------|-------|--------|
| `cicd-build-hardening` | CI/CD pipeline hardening | Active |
| `code-quality-readability` | Lint, complexity, doc comments | Active |
| `documentation-hygiene` | Doc accuracy and completeness | Active |
| `observability` | Metrics, telemetry, reporting | Active |
| `performance-optimization` | Latency, throughput, memory | Active |
| `security-hardening` | Threat model, isolation, auth | Active |
| `testing-reliability` | Test coverage, determinism, stress | Active |

**Tests/build/deployment:**
- `Makefile`: `build`, `test`, `lint`, `cover-check`, `perf-gate`, `stress*`, `ci`
- `.github/workflows/ci.yml`: lint, govulncheck, test (ubuntu/macOS × go1.22/stable), coverage floor, stress suite, build (ubuntu/macOS/windows)
- `.github/workflows/release.yml`: tag-triggered, runs `make ci`, then GoReleaser with SBOM
- `.golangci.yml`: staticcheck, errcheck, govet, ineffassign, unused, errorlint, gosec, bodyclose, gocritic, unconvert, misspell, gocyclo (min 20), revive (exported rule)
- Coverage floors: overall ≥79%, `internal/core` ≥80%, `internal/cmd` ≥71%, `internal/worker` ≥50% (`E4` from `scripts/coverage-check.sh`)

**Invariants:**
- `INV1`: stdlib-only — no external Go dependencies
- `INV2`: Atomic state writes — every `SaveState` under advisory lock with revision bump
- `INV3`: Deterministic exit codes — same input → same exit code
- `INV4`: Fail-closed sandboxing — missing isolation backend refuses rather than silently falls back
- `INV5`: Backward-compatible state.json — newer schema versions rejected, older versions migrated

---

## 4. Requirements and Evidence

| ID | Requirement or Fact | Evidence/Source | Priority | Acceptance Signal |
|----|---------------------|-----------------|----------|-------------------|
| R1 | All 13 CLI commands must have deterministic exit codes | `main.go` switch, `internal/cmd/*_test.go` | Critical | `go test ./internal/cmd/...` passes |
| R2 | State atomicity (CAS + advisory lock) must prevent lost updates under contention | `internal/core/lock.go`, `internal/core/state.go`, `scripts/stress.sh` | Critical | `make stress` passes |
| R3 | DAG scheduling must correctly compute runnable frontier with no cycles/orphans | `internal/core/dag.go`, `internal/core/gates.go:GateDAG` | Critical | `go test ./internal/core/... -run DAG` passes |
| R4 | Verify execution must support shell, bwrap, and container backends with fail-closed behavior | `internal/runner/runner.go`, `internal/cmd/verify.go`, `SECURITY.md` | Critical | `go test ./internal/runner/...` passes + manual bwrap/container test |
| R5 | MCP server must handle stdio, HTTP/SSE with correct JSON-RPC framing and auth | `internal/mcp/server.go`, `docs/mcp-guide.md` | Critical | `go test ./internal/mcp/...` passes |
| R6 | Agent onboarding (`init`) must be idempotent and produce byte-identical receipts | `internal/cmd/init.go`, `make perf-gate` | High | `make perf-gate` passes (count=2) |
| R7 | Report generation must produce valid Markdown, HTML, and PR summaries | `internal/core/report.go`, `internal/cmd/report.go` | High | Golden file tests pass |
| R8 | Install script must verify checksums and fail closed on mismatch | `scripts/install.sh`, `SECURITY.md` | High | `shellcheck scripts/install.sh` + manual test |
| R9 | Cross-platform builds must compile for linux/darwin/windows × amd64/arm64 | `.goreleaser.yml`, `ci.yml` build matrix | High | `make build` on all platforms |
| R10 | Security boundaries must prevent path traversal, env leakage, and unauthorized MCP access | `internal/core/slug.go` (inferred), `SECURITY.md`, `internal/mcp/server.go` | Critical | `go test ./... -run 'Slug|Security|Auth'` passes |
| R11 | Custom gates must run with scrubbed env and timeout but without sandbox isolation | `internal/core/gates.go:runCustomGates`, `SECURITY.md` | High | Custom gate integration test passes |
| R12 | Documentation must accurately reflect command behavior, flags, and config keys | `docs/command-reference.md`, `docs/user-guide.md` | Medium | Doc lint passes + spot-check against `--help` output |
| R13 | Performance must not regress from established baselines | `docs/agent-harness-baselines.md` (referenced), `make bench` | Medium | `make bench` results within ±10% of baseline |
| R14 | Coverage floors must not drop below current thresholds | `scripts/coverage-check.sh` | High | `make cover-check` passes |
| R15 | CI must complete in <15 minutes and all stress tests must pass | `.github/workflows/ci.yml`, `Makefile` | High | CI green on PR |

---

## 5. Findings and Impact

| ID | Finding/Constraint | Evidence | Impact | Recommendation |
|----|--------------------|----------|--------|----------------|
| F1 | `main.go` has `//nolint:gocyclo` on `run()` with 2200+ functions in codebase, 93 exceeding complexity 15 | `main.go`, `.golangci.yml` | Medium — complexity debt in entry point increases regression risk for new commands | Add targeted unit tests for `run()` branching; consider refactor post-v1.0 |
| F2 | Windows self-update is known-limited; brain/pinky orchestration is POSIX-only on Windows | `README.md`, `TESTING.md` | Medium — Windows users get degraded experience | Document limitation clearly; add Windows-specific test skips |
| F3 | `specd doctor` command was removed; no pre-check for bwrap/container availability | `SECURITY.md` | Low — fail-closed at verify time is sufficient per design | Add `doctor` back as non-blocking diagnostic, or document `verify --sandbox` as the check |
| F4 | Custom gates run unisolated on host — documented but high-risk if gate commands are malicious | `SECURITY.md` | High — custom gates are a trust boundary | Add warning on custom gate registration; consider opt-in `--trust-custom-gates` |
| F5 | MCP HTTP defaults to loopback but warns on non-loopback; no TLS built-in | `SECURITY.md`, `internal/mcp/server.go` | Medium — operator error can expose workflow control | Add `--tls-cert`/`--tls-key` flags or document reverse-proxy requirement more prominently |
| F6 | Coverage floors were recently re-baselined after subsystem removal; some packages lack floors | `scripts/coverage-check.sh` | Medium — `internal/worker` at 50% is thin | Raise `internal/worker` floor to 70%; add floors for `internal/mcp`, `internal/runner` |
| F7 | Stress tests are shell-based, not Go-based; harder to debug and less hermetic | `scripts/stress*.sh` | Medium — shell tests can fail due to environment differences | Port critical stress paths to Go tests with `t.Parallel()` |
| F8 | No fuzz testing for parsers (tasks.md, requirements.md, state.json) | Not observed in test files | Medium — parser crashes on malformed input are a production risk | Add fuzz targets for `ParseTasks`, `LoadState`, `LintEars` |
| F9 | `go.mod` declares `go 1.22` with `toolchain go1.22.0`; CI tests with `stable` Go which may be newer | `go.mod`, `.github/workflows/ci.yml` | Low — Go has strong backward compatibility | Add `go mod tidy` check in CI to ensure no accidental toolchain drift |
| F10 | SBOM generation requires `syft` on release runner; artifact signing is deferred | `.goreleaser.yml` | Medium — supply chain security gap | Document signing deferral; add issue to track |
| F11 | Existing optimization specs may have incomplete tasks or unverified claims | `specs/*/tasks.md` (not fully inspectable via API) | Unknown — requires local validation | Coding agent must re-inspect each spec and verify task completion |

---

## 6. Implementation Vision

**Design direction:** Create a regression spec suite that exercises every production-critical path with automated validation. The suite is organized by concern (not by existing spec) to ensure no gaps.

**Affected modules:**
- `internal/cmd/` — command regression tests
- `internal/core/` — state, lock, DAG, gate regression tests
- `internal/runner/` — sandbox backend regression tests
- `internal/mcp/` — server protocol regression tests
- `internal/worker/` — worker execution regression tests
- `scripts/` — install and stress script regression tests
- `.github/workflows/` — CI pipeline validation
- `docs/` — documentation accuracy checks

**Interfaces/contracts:**
- CLI commands must maintain backward-compatible flag surfaces
- `state.json` schema version must remain at 5 or follow migration path
- MCP tool list must remain stable for existing agent integrations
- Report output formats must remain structurally compatible

**Data/configuration:**
- `config.json`/`config.yml` precedence must be preserved
- `SPECD_*` env var overrides must continue to work
- Embedded pack templates must remain byte-identical

**Security/failures:**
- Sandbox isolation must remain fail-closed
- Env scrubbing must remain comprehensive
- Path traversal prevention must remain effective
- MCP auth must remain constant-time

**Observability:**
- `obs.RecordDuration` and `obs.RecordCounter` must not panic or leak
- Report telemetry sections must remain accurate

**Compatibility:**
- Go 1.22+ must remain supported
- POSIX shell scripts must remain POSIX-compliant
- Windows builds must continue to compile

**Rollout:**
- Regression specs are development artifacts; they don't ship with the binary
- Validation is CI-gated; no production rollout risk

**Rollback:**
- Not applicable — this is a planning and testing effort

---

## 7. Specification Map

| Spec | Responsibility | Requirements | Dependencies | Validation |
|------|----------------|--------------|--------------|------------|
| S1: CLI Command Regression | Verify all 13 commands produce correct exit codes and output | R1, R12 | None | `go test ./internal/cmd/...` |
| S2: State Atomicity Regression | Verify CAS, lock, and concurrent write safety | R2 | None | `make stress`, `go test ./internal/core/... -run CAS\|Lock` |
| S3: DAG & Scheduling Regression | Verify frontier, cycle detection, wave ordering | R3 | S2 | `go test ./internal/core/... -run DAG\|Wave` |
| S4: Verify & Sandbox Regression | Verify shell/bwrap/container execution and isolation | R4, R11 | None | `go test ./internal/runner/...`, manual sandbox test |
| S5: MCP Server Regression | Verify stdio/HTTP/SSE protocol, auth, tool dispatch | R5 | S1 | `go test ./internal/mcp/...` |
| S6: Onboarding Regression | Verify `init` idempotency, pack application, host detection | R6 | S5 | `make perf-gate` |
| S7: Reporting Regression | Verify Markdown/HTML/PR summary output correctness | R7 | S3 | Golden file tests |
| S8: Install Integrity Regression | Verify checksum verification and script correctness | R8 | None | `shellcheck scripts/install.sh` + manual test |
| S9: Cross-Platform Regression | Verify builds on all target platforms | R9 | None | CI build matrix |
| S10: Security Boundary Regression | Verify path traversal, env scrubbing, auth, isolation | R10, R4, R5 | S2, S4, S5 | `go test ./... -run 'Slug\|Security\|Auth\|Scrub'`, `gosec` |
| S11: Performance Baseline Regression | Verify no performance degradation | R13 | S1-S10 | `make bench` |
| S12: Coverage Floor Regression | Verify coverage thresholds maintained | R14 | S1-S10 | `make cover-check` |
| S13: CI Pipeline Regression | Verify CI completes reliably and quickly | R15 | S1-S12 | CI run on PR |
| S14: Documentation Accuracy Regression | Verify docs match code behavior | R12 | S1 | Doc lint + spot-check |
| S15: Fuzz & Parser Regression | Verify parser robustness against malformed input | R1-R5 | None | `go test -fuzz` targets |

---

## 8. Execution and Validation Plan

**Phase 1: Foundation (parallel)**
- P1.1: Re-inspect repository locally and validate all assumptions in this plan
- P1.2: Run existing test suite and record baseline (`make ci`)
- P1.3: Verify existing spec completion status against `specs/progress.md`

**Phase 2: Spec Creation (parallel by concern)**
- P2.1: Create S1-S5 specs (core functional areas)
- P2.2: Create S6-S10 specs (integration, security, platform)
- P2.3: Create S11-S15 specs (performance, coverage, CI, docs, fuzz)

**Phase 3: Task Implementation (dependency-ordered waves)**
- Wave 1: S1, S2, S4, S8, S15 (no dependencies)
- Wave 2: S3 (depends on S2), S5 (depends on S1), S10 (depends on S2, S4)
- Wave 3: S6 (depends on S5), S7 (depends on S3)
- Wave 4: S9, S11, S12, S13, S14 (depend on earlier specs)

**Phase 4: Validation & Gate**
- Run full `make ci` and verify all new tests pass
- Verify coverage floors still met
- Verify stress tests pass
- Verify no new lint violations

**Phase 5: Documentation & Handoff**
- Update `specs/progress.md` with regression spec status
- Update `TESTING.md` with regression test documentation
- Create summary report of covered vs. uncovered areas

**Repository commands:**
- `make ci` — full validation gate
- `make stress` — concurrency stress
- `make cover-check` — coverage floor
- `make perf-gate` — deterministic output
- `go test ./... -race -count=1` — unit tests with race detector
- `go test ./... -count=2` — order-dependence guard
- `golangci-lint run` — static analysis
- `govulncheck ./...` — vulnerability scan

**Rollback checkpoints:**
- After Phase 1: if baseline tests fail, fix before proceeding
- After Phase 3 each wave: if tests fail, fix before next wave
- After Phase 4: if CI fails, iterate before declaring done

---

## 9. Risks and Unknowns

| ID | Risk/Unknown | Consequence | Resolution/Mitigation | Gate |
|----|--------------|-------------|-----------------------|------|
| U1 | Existing specs in `specs/` may have incomplete or outdated tasks | Regression plan may miss already-identified issues | Re-inspect each spec locally; update progress tracking | Phase 1 |
| U2 | Some internal files could not be fetched via API (rate limiting/404) | Analysis may have gaps in `internal/core/` sub-packages | Coding agent must clone repo and verify all paths | Phase 1 |
| U3 | Windows stress tests cannot run in current CI (no Windows shell stress) | Windows-specific race conditions may go undetected | Add Windows runner to CI for build; document WSL requirement for full stress | S9 |
| U4 | bwrap/container sandbox backends require host tools not available in CI | Sandbox paths may not be exercised in CI | Add bwrap to CI runner or document manual validation requirement | S4 |
| U5 | Fuzz testing is new to this codebase; may find many issues | Scope creep if fuzz reveals deep parser bugs | Time-box fuzz targets to 1 hour; file issues for non-critical findings | S15 |
| U6 | Performance baselines (`docs/agent-harness-baselines.md`) were not fetched | Cannot verify exact baseline values | Coding agent must read file locally and establish current baselines | S11 |
| U7 | The `specd doctor` command removal may leave users without diagnostics | Support burden increases | Consider adding `doctor` back as informational only | F3 |

---

## 10. Definition of Done

- [ ] All 15 regression specs (S1-S15) created under `specs/` with `spec.md` and `tasks.md`
- [ ] Every spec has acceptance criteria mapped to at least one requirement (R1-R15)
- [ ] Every task has a validation command and completion evidence
- [ ] `specs/progress.md` tracks all regression specs with status
- [ ] `make ci` passes with all new tests
- [ ] Coverage floors maintained or raised
- [ ] No new lint violations introduced
- [ ] Stress tests pass
- [ ] Documentation (`TESTING.md`, `docs/`) updated with regression test references
- [ ] Gap analysis document identifies any uncovered areas with rationale

---

## 11. Traceability

| Requirement | Evidence | Spec | Validation | Done Criterion |
|-------------|----------|------|------------|----------------|
| R1 | `main.go`, `internal/cmd/*` | S1 | `go test ./internal/cmd/...` | All command exit-code tests pass |
| R2 | `internal/core/lock.go`, `internal/core/state.go` | S2 | `make stress`, CAS tests | Turn == successes, no torn writes |
| R3 | `internal/core/dag.go`, `internal/core/gates.go` | S3 | DAG tests | Frontier/orphan/cycle tests pass |
| R4 | `internal/runner/runner.go`, `internal/cmd/verify.go` | S4 | Runner tests, manual sandbox | All backends execute correctly |
| R5 | `internal/mcp/server.go` | S5 | `go test ./internal/mcp/...` | Protocol/auth tests pass |
| R6 | `internal/cmd/init.go` | S6 | `make perf-gate` | Byte-identical receipts on count=2 |
| R7 | `internal/core/report.go` | S7 | Golden file tests | Report output matches expected |
| R8 | `scripts/install.sh` | S8 | `shellcheck`, manual test | Checksum verification works |
| R9 | `.goreleaser.yml`, `ci.yml` | S9 | CI build matrix | All platform builds succeed |
| R10 | `SECURITY.md`, slug validation | S10 | Security tests, `gosec` | Path/env/auth tests pass |
| R11 | `internal/core/gates.go` | S4 | Custom gate integration test | Custom gates run with scrubbed env |
| R12 | `docs/command-reference.md` | S14 | Doc lint, spot-check | Docs match `--help` output |
| R13 | `docs/agent-harness-baselines.md` | S11 | `make bench` | Within ±10% of baseline |
| R14 | `scripts/coverage-check.sh` | S12 | `make cover-check` | All floors met |
| R15 | `.github/workflows/ci.yml` | S13 | CI run | CI green, <15 min |
