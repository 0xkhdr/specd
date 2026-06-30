# Analysis and Action Plan: specd Production-Grade Optimization Review

## 1. Intent and Success

**Task Class:** Optimization / Hardening / Refactor — comprehensive production-grade review.

**Requested Outcome:** Inspect the complete `specd` repository (an agent-agnostic, spec-driven coding harness CLI) to identify and plan optimizations across performance, code quality, readability, security, and all production-grade attributes.

**Measurable Definition of Success:**
- All 7+ internal Go packages reviewed for production-grade adherence.
- Performance bottlenecks identified with reproducible baseline/target methodology.
- Security surface audited (input validation, sandboxing, command execution, file I/O).
- Test coverage and reliability improved (race conditions, error paths, resource cleanup).
- Code readability and maintainability enhanced (naming, documentation, complexity).
- CI/CD and build pipeline hardened.
- Every finding maps to a spec, validation command, and done criterion.

---

## 2. Scope and Decisions

**In Scope:**
- All Go source under `internal/` (cli, cmd, context, core, integration, mcp, obs, pack, runner, schema, spec, testharness, worker).
- Root entry point (`main.go`, `main_test.go`).
- Build system (`Makefile`, `go.mod`, build scripts).
- Quality gates (`.golangci.yml`, `fmt-check`, `test-lint`, `shellcheck`, `coverage-check`).
- Test infrastructure (unit, integration, stress, orchestration, fault-injection).
- Security surface (sandboxed verify, bwrap, command execution, file paths, MCP integration).
- CI/CD (`.github/`, `.goreleaser.yml`).
- Documentation consistency and accuracy.
- Install/uninstall scripts (`scripts/install.sh`, `scripts/uninstall.sh`).

**Out of Scope:**
- External agent host implementations (Claude Code, Codex, Cursor, etc.) — these are consumers, not part of specd's code.
- User-authored spec content (the harness validates structure, not semantic correctness of user specs).
- Third-party dependencies (audited for usage patterns, but not source-reviewed).

**Assumptions:**
- A1: The repository uses Go 1.22+ (inferred from `go.mod` and modern Go idioms in `main.go`).
- A2: The target platform is Linux/macOS primarily (Windows support is best-effort per README).
- A3: The concurrency model uses advisory locking + CAS (documented in `docs/contributor-guide.md`).

**Decision Gates:**
- D1: Are there any external dependencies beyond stdlib? (affects supply-chain risk assessment).
- D2: Does the DAG computation use efficient algorithms? (affects performance optimization priority).
- D3: Are all file operations path-traversal-safe? (affects security hardening scope).
- D4: Is the bwrap sandboxing fail-closed as documented? (affects security verification).

---

## 3. Repository Context

**Stack:** Go CLI application, stdlib-focused (minimal `go.mod` ~104 bytes), single-module.

**Architecture:** Layered internal packages:
- `internal/cli` — CLI framework and command routing.
- `internal/cmd` — Command implementations (new, check, verify, approve, next, etc.).
- `internal/core` — Core business logic (spec lifecycle, state machine).
- `internal/spec` — Spec parsing and validation (EARS, design, tasks).
- `internal/schema` — JSON Schema validation.
- `internal/runner` — Task execution and verification runner.
- `internal/worker` — Brain/Pinky worker orchestration (concurrent execution).
- `internal/mcp` — Model Context Protocol server implementation.
- `internal/context` — Context engineering for agent prompts.
- `internal/obs` — Observability (reporting, dashboard, events).
- `internal/pack` — Spec pack scaffolding.
- `internal/integration` — Agent integration and host detection.
- `internal/testharness` — Test utilities.

**Key Flows:**
- Spec lifecycle: `new` → `check` → `approve` → `next` → `verify` → `task --status complete` → `approve` (close).
- DAG execution: Task dependency graph → wave computation → frontier dispatch.
- MCP handshake: `specd mcp` → stdio/HTTP/SSE → tools/list → tool execution.

**Conventions:**
- Go standard project layout (cmd/, internal/).
- Makefile-driven build/test/ci.
- EARS format for requirements.
- Markdown specs as source of truth.
- Evidence-gated status transitions.

**Analogous Implementations:** `make` (DAG execution), `git` (state machine + plumbing), `docker` (sandboxed execution).

**Tests/Build/Deployment:**
- `make test` — unit tests with `-race`.
- `make wrapper-test` — Python workflow harness.
- `make stress*` — concurrency and fault injection.
- `make perf-gate` — deterministic output validation.
- `make ci` — full CI pipeline locally.
- `.goreleaser.yml` — release automation.
- `.github/` — PR gates and GitHub Action.

**Invariants:**
- INV1: All state mutations are atomic and versioned (safe for concurrent agents).
- INV2: Exit codes are deterministic: 0=ok, 1=validation, 2=usage, 3=not found.
- INV3: `verify:` commands run under sandbox isolation (fail-closed if bwrap absent).
- INV4: Task completion requires passing verify record (never free-text).
- INV5: Spec status transitions are unidirectional (ratchet).

---

## 4. Requirements and Evidence

| ID | Requirement or Fact | Evidence/Source | Priority | Acceptance Signal |
|----|---------------------|-----------------|----------|-------------------|
| R1 | Go code follows production-grade idioms (error handling, context propagation, resource cleanup) | `main.go`, `internal/`, `.golangci.yml` | High | `make lint` passes with zero warnings; no `panic` in non-init paths |
| R2 | Performance: DAG computation and frontier dispatch are optimal for large spec graphs | `docs/contributor-guide.md` (concurrency model), `internal/worker/` | High | Benchmarks show O(V+E) DAG computation; frontier dispatch &lt;10ms for 100-task specs |
| R3 | Security: All file paths are validated (no traversal, no symlink attacks) | `SECURITY.md`, `internal/cmd/` (file operations), `scripts/install.sh` | Critical | Fuzz tests pass; `make test` includes path traversal cases |
| R4 | Security: Command execution (verify:, custom gates) is injection-safe | `internal/runner/`, `SECURITY.md` | Critical | Shell injection test suite passes; `bwrap` isolation verified |
| R5 | Security: MCP server input validation prevents malicious tool calls | `internal/mcp/`, `docs/mcp-guide.md` | High | JSON Schema validation on all inputs; fuzz testing |
| R6 | Code quality: Function complexity is bounded (cyclomatic &lt;15) | `.golangci.yml`, `internal/` | Medium | `gocyclo` or equivalent passes; no function exceeds 50 lines without comment |
| R7 | Code quality: All exported symbols are documented; internal packages have package docs | `internal/`, `docs/` | Medium | `go doc` renders complete package documentation; `make lint` includes doc check |
| R8 | Test quality: Race-free concurrency (advisory lock + CAS) | `main_test.go`, `TESTING.md`, `make test` with `-race` | High | `make test` with `-race` passes; stress tests pass |
| R9 | Test quality: Error paths and edge cases are covered | `TESTING.md`, coverage reports | Medium | Coverage floor enforced by `scripts/coverage-check.sh`; error path coverage &gt;60% |
| R10 | Observability: Structured logging, metrics, and tracing hooks | `internal/obs/`, `docs/dashboard.md` | Medium | Logs are structured (JSON); key operations have duration metrics |
| R11 | Resource safety: All file handles, goroutines, and network connections are properly closed | `internal/`, `main.go` | High | `go vet` passes; leak tests in `make stress` |
| R12 | CI/CD: Build reproducibility and supply chain security | `.goreleaser.yml`, `Makefile` | Medium | Reproducible builds (trimpath, buildid); SBOM generation |
| R13 | Documentation: All public-facing docs are accurate and complete | `docs/`, `README.md`, `AGENTS.md` | Low | `scripts/docs-lint.sh` passes; no stale references to removed features |

---

## 5. Findings and Impact

| ID | Finding/Constraint | Evidence | Impact | Recommendation |
|----|--------------------|----------|--------|----------------|
| F1 | `go.mod` is extremely minimal (~104 bytes) — appears to use mostly stdlib | `go.mod` (verified) | Positive: Low supply-chain attack surface; Risk: May reinvent wheels | Validate all imports; document rationale for any non-stdlib dependency |
| F2 | `scripts/install.sh` uses `curl \| bash` pattern | `scripts/install.sh` (verified) | Security risk: Supply chain and MITM concerns | Add checksum verification (SHA-256) of downloaded binary; document security model |
| F3 | `Makefile` has extensive stress targets but no resource limit enforcement | `Makefile` (verified) | Risk: Stress tests may exhaust local resources | Add `ulimit` controls and timeout wrappers to stress targets |
| F4 | Windows support is explicitly limited ("POSIX-only on Windows — run under WSL") | `README.md` (verified) | Compatibility constraint: Excludes native Windows users | Document Windows roadmap; consider `os/exec` abstraction for Windows compatibility |
| F5 | `bwrap` sandboxing is fail-closed (good) but requires external dependency | `README.md`, `SECURITY.md` (verified) | Operational risk: Verify fails if bwrap not installed | Add graceful degradation with warning; document bwrap installation requirement |
| F6 | `SPECD_JSON=1` enables structured output but logging strategy is unclear | `README.md` (verified) | Observability gap: No clear logging framework | Audit `internal/obs/` for structured logging; add `slog` or similar |
| F7 | `main.go` uses `flag` package (stdlib) rather than `cobra` or similar | `main.go` (partially verified) | Trade-off: Less bloat vs. less features | Evaluate if CLI complexity warrants migration; document decision |
| F8 | `make ci` runs everything sequentially | `Makefile` (verified) | Performance: Local CI takes longer than necessary | Add parallel job execution where safe; document dependency ordering |
| F9 | No `go.sum` observed in root (may be in subdirectories or not committed) | Root listing (verified) | Requires validation: Dependency integrity verification | Verify `go.sum` presence and Go module proxy configuration |
| F10 | `internal/testharness` exists but test coverage enforcement is script-based | `scripts/coverage-check.sh` (verified) | Maintainability: Shell scripts for coverage are fragile | Consider native `go test -cover` with profile-based enforcement |
| F11 | `AGENTS.md` and `TESTING.md` are large (15K+ and 13K+ bytes) | File sizes (verified) | Readability: Large markdown files may be hard to navigate | Add table of contents; split into focused sub-documents |
| F12 | `.golangci.yml` is present but version and linter list need validation | `.golangci.yml` (verified) | Code quality: May be using outdated linters | Update to latest `.golangci.yml` schema; enable additional linters (e.g., `gosec`, `revive`) |

---

## 6. Implementation Vision

**Design Direction:** Production-grade hardening with measurable improvements. Preserve existing architecture and invariants while elevating code quality, security posture, and operational observability.

**Affected Modules:**
- `internal/cmd/` — Input validation, path sanitization, error wrapping.
- `internal/runner/` — Command execution hardening, timeout enforcement, resource cleanup.
- `internal/core/` — State machine optimization, atomic operation validation.
- `internal/worker/` — Concurrency optimization, goroutine lifecycle management.
- `internal/mcp/` — Input validation, rate limiting, schema enforcement.
- `internal/obs/` — Structured logging, metrics emission, tracing hooks.
- `internal/spec/` — Parser performance, memory allocation optimization.
- `scripts/` — Install script hardening, stress test resource limits.
- `Makefile` — Parallel execution, reproducible build flags.
- `.golangci.yml` — Linter modernization, security linters.

**Interfaces/Contracts:**
- Preserve CLI interface (all commands, flags, exit codes).
- Preserve `config.json` schema.
- Preserve spec directory structure (`.specd/specs/&lt;name&gt;/`).
- Preserve state file format (`state.json`).
- Preserve MCP tool contract.

**Data/Configuration:**
- No breaking changes to spec format or state file.
- Add optional `.specd/config.json` keys for observability (metrics endpoint, log level).
- Add environment variables: `SPECD_LOG_LEVEL`, `SPECD_METRICS_ENDPOINT`.

**Security/Failures:**
- Fail-closed on sandbox absence (preserve INV3).
- All file operations use `filepath.Clean` + jail checks.
- Command execution uses `exec.Command` with explicit args (never shell string interpolation).
- Input size limits on all parsers (spec files, state files, MCP payloads).

**Observability:**
- Structured JSON logging via `log/slog` (Go 1.21+).
- Prometheus-style metrics for: command duration, DAG computation time, verify execution time, task completion rate, error rate by category.
- OpenTelemetry tracing hooks for frontier dispatch and worker orchestration.

**Compatibility:**
- Backward compatible with all existing specs and state files.
- Graceful degradation when new observability features are unavailable.
- No changes to external agent integration contracts.

**Rollout:**
- Phase 1: Static analysis and linting (no runtime changes).
- Phase 2: Security hardening (input validation, path sanitization).
- Phase 3: Performance optimization (DAG, parsing, allocation).
- Phase 4: Observability integration (logging, metrics).
- Phase 5: Documentation and CI/CD updates.

**Rollback:**
- All changes are source-level; rollback via `git revert`.
- No database migrations or state format changes.
- Feature flags for observability via environment variables.

---

## 7. Specification Map

| Spec | Responsibility | Requirements | Dependencies | Validation |
|------|----------------|--------------|--------------|------------|
| S1: Security Hardening | Input validation, path sanitization, command execution safety | R3, R4, R5 | None | Fuzz tests, `make test`, security audit |
| S2: Performance Optimization | DAG computation, frontier dispatch, parser efficiency | R2 | S1 (security first) | Benchmarks, `make perf-gate`, `make bench` |
| S3: Code Quality & Readability | Linting, documentation, complexity reduction | R1, R6, R7 | None | `make lint`, `go vet`, `gocyclo` |
| S4: Testing & Reliability | Race detection, error paths, coverage, leak tests | R8, R9 | S1, S3 | `make test`, `make stress`, coverage report |
| S5: Observability | Structured logging, metrics, tracing hooks | R10 | S2 (performance baseline needed) | Log output validation, metrics endpoint test |
| S6: CI/CD & Build Hardening | Reproducible builds, supply chain, release automation | R12 | S3 | `make ci`, `.goreleaser.yml` validation |
| S7: Documentation & Repository Hygiene | Doc accuracy, install script security, organization | R13, F11 | S3, S6 | `scripts/docs-lint.sh`, manual review |

---

## 8. Execution and Validation Plan

**Phase 1: Foundation (Parallel)**
- P1.1: Update `.golangci.yml` — add `gosec`, `revive`, `errname`, `containedctx`; update to latest schema.
- P1.2: Audit `go.mod`/`go.sum` — verify integrity, document dependency rationale.
- P1.3: Run full `make ci` to establish baseline — record duration, coverage, lint warnings.
- Validation: `make lint` passes; baseline metrics recorded.

**Phase 2: Security Hardening (Sequential)**
- P2.1: Audit all `os.Open`, `ioutil.ReadFile`, `filepath.Join` in `internal/` — add path sanitization.
- P2.2: Harden `internal/runner/` — use `exec.Command` with explicit args; add timeout and kill-switch.
- P2.3: Harden `scripts/install.sh` — add SHA-256 checksum verification; document trust model.
- P2.4: Add fuzz tests for spec parsing and MCP input handling.
- Validation: `make test` passes; new security tests pass; `gosec` linter passes.

**Phase 3: Performance Optimization (Parallel where safe)**
- P3.1: Profile DAG computation in `internal/worker/` — optimize wave frontier algorithm.
- P3.2: Profile spec parsing in `internal/spec/` — reduce allocations, use streaming parsers.
- P3.3: Optimize `internal/obs/` reporting — reduce file I/O, add caching.
- Validation: `make bench` shows improvement; `make perf-gate` passes; no regression in `make test`.

**Phase 4: Testing & Reliability**
- P4.1: Add race-free tests for all concurrent paths (advisory lock, CAS, worker orchestration).
- P4.2: Add error path tests (malformed specs, missing files, permission errors).
- P4.3: Add resource leak tests (goroutines, file handles, temp directories).
- P4.4: Raise coverage floor in `scripts/coverage-check.sh`.
- Validation: `make test -race` passes; `make stress` passes; coverage report meets floor.

**Phase 5: Observability**
- P5.1: Integrate `log/slog` for structured logging across all packages.
- P5.2: Add metrics collection points (command duration, DAG time, verify time).
- P5.3: Add OpenTelemetry tracing hooks (optional, compile-time).
- Validation: `SPECD_JSON=1` output remains valid; metrics are emitted; no performance regression.

**Phase 6: CI/CD & Documentation**
- P6.1: Harden `Makefile` — add parallel execution, reproducible build flags (`-trimpath`).
- P6.2: Update `.goreleaser.yml` — add SBOM generation, checksum signing.
- P6.3: Refactor large docs (`AGENTS.md`, `TESTING.md`) — add TOCs, split sections.
- P6.4: Update `SECURITY.md` with new hardening measures.
- Validation: `make ci` passes; release dry-run succeeds; docs render correctly.

**Gates:**
- G1: After Phase 2 — Security review complete before performance changes.
- G2: After Phase 3 — Benchmark baseline recorded and validated.
- G3: After Phase 4 — All tests pass with `-race` before observability integration.
- G4: After Phase 5 — No performance regression &gt;5% before documentation updates.

**Rollback Checkpoints:**
- Each phase is a separate commit/branch; rollback via `git revert`.
- Phase 2+ changes are behind feature flags where possible (env vars).

---

## 9. Risks and Unknowns

| ID | Risk/Unknown | Consequence | Resolution/Mitigation | Gate |
|----|--------------|-------------|-----------------------|------|
| U1 | Internal package implementations not fully inspected (network timeouts prevented full source reading) | Findings may miss critical issues | Coding agent must re-inspect all `internal/` source; validate every claim | G1 |
| U2 | Dependency list in `go.mod` may be incomplete or outdated | Supply chain risk, build failures | Verify `go mod tidy` produces clean output; check `go.sum` | P1.2 |
| U3 | Windows compatibility limitations may be deeper than documented | User confusion, bug reports | Audit all POSIX-specific code (`syscall`, `os/exec` shell mode); document gaps | P2.2 |
| U4 | Performance optimizations may break concurrency invariants | Data races, state corruption | Strict `-race` testing; property-based tests for state machine | G3 |
| U5 | Observability integration may add overhead unacceptable for CLI tool | Degraded user experience | Benchmark before/after; make metrics optional via env var | G2 |
| U6 | `bwrap` sandboxing behavior on different Linux distributions | Inconsistent security posture | Test on Ubuntu, Debian, Fedora, Alpine; document requirements | P2.2 |
| U7 | MCP protocol version compatibility | Integration breakage with agent hosts | Verify against MCP spec version; add protocol negotiation | P2.4 |

---

## 10. Definition of Done

- [ ] All 13 requirements (R1–R13) have corresponding specs and tasks.
- [ ] `make lint` passes with zero warnings using updated `.golangci.yml`.
- [ ] `make test` passes with `-race` flag on all packages.
- [ ] `make stress` passes without resource exhaustion or goroutine leaks.
- [ ] `make perf-gate` passes and shows no regression from baseline.
- [ ] Security fuzz tests pass for spec parsing, MCP input, and file operations.
- [ ] `scripts/install.sh` includes checksum verification and documented trust model.
- [ ] All file operations in `internal/` use sanitized paths (verified by `gosec`).
- [ ] Structured logging is available via `log/slog` with JSON output.
- [ ] Metrics are emitted for command duration, DAG computation, and verify execution.
- [ ] Documentation is accurate: no stale references, all commands documented.
- [ ] Coverage floor is met or raised (verified by `scripts/coverage-check.sh`).
- [ ] CI pipeline (`make ci`) completes successfully in under baseline duration +10%.
- [ ] No breaking changes to CLI interface, spec format, or state file format.

---

## 11. Traceability

| Requirement | Evidence | Spec | Validation | Done Criterion |
|-------------|----------|------|------------|----------------|
| R1 | `main.go`, `.golangci.yml` | S3 | `make lint`, `go vet` | Zero lint warnings; no `panic` in non-init |
| R2 | `docs/contributor-guide.md` | S2 | `make bench`, `make perf-gate` | Benchmark improvement; no regression |
| R3 | `SECURITY.md`, `internal/cmd/` | S1 | Fuzz tests, `gosec` | Path sanitization on all file ops |
| R4 | `internal/runner/`, `SECURITY.md` | S1 | Injection tests, `make test` | Explicit args only; no shell interpolation |
| R5 | `internal/mcp/`, `docs/mcp-guide.md` | S1 | Fuzz tests, schema validation | JSON Schema on all MCP inputs |
| R6 | `.golangci.yml`, `internal/` | S3 | `gocyclo`, `make lint` | All functions &lt;15 cyclomatic |
| R7 | `internal/`, `docs/` | S3 | `go doc`, `make lint` | Complete package docs |
| R8 | `main_test.go`, `TESTING.md` | S4 | `make test -race`, `make stress` | Race detector clean |
| R9 | `TESTING.md`, coverage reports | S4 | `make test`, coverage report | Error path coverage &gt;60% |
| R10 | `internal/obs/`, `docs/dashboard.md` | S5 | Log validation, metrics test | JSON logs emitted; metrics available |
| R11 | `internal/`, `main.go` | S4 | `go vet`, leak tests | No resource leaks in stress |
| R12 | `.goreleaser.yml`, `Makefile` | S6 | `make ci`, release dry-run | Reproducible build; SBOM present |
| R13 | `docs/`, `README.md` | S7 | `scripts/docs-lint.sh` | No stale references; TOCs present |
