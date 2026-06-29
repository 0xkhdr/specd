# specd — Spec Hardening Analysis & Production-Readiness Action Plan

**Date:** 2026-06-29
**Scope:** All implemented specs under `specs/` (commands, config, fusion, resilience) cross-checked against the `internal/` + `cmd/` implementation, CI, and best practice for each task's domain.
**Branch reviewed:** `main` @ `3d41aa5`

---

## 1. Executive summary

specd is in **strong shape**. Every spec is marked `complete` with substantive per-task notes, and the claims hold up under inspection:

- `go build ./...` clean, `go vet ./...` clean.
- **1502 tests pass** across 14 packages; the only skips are environment-conditional (POSIX-only shell, root-bypasses-perms, symlinks-unavailable, backend-unavailable) — all legitimate, none masking gaps.
- **Zero external runtime dependencies** (stdlib only) — minimal supply-chain surface.
- CI is comprehensive: `gofmt`, `go vet`, structural test-lint, shellcheck, `golangci-lint` (staticcheck/errcheck/gosec/errorlint/ineffassign/unused), **govulncheck**, `-race`, `-count=2` order-dependence, a ratcheting **coverage floor**, two **stress** jobs, and cross-platform (linux/macos/windows) builds.
- Security posture is deliberate: fail-closed `verify` sandbox (`bwrap`/container, `--network none`), scrubbed env, untrusted-input threat model in `SECURITY.md`, documented config precedence.

This is **not a rescue job**. It is a hardening pass: the findings below are edge-hardening, consistency, and coverage items — the difference between "works and is well-tested" and "production-grade across hostile/degraded conditions." Findings are prioritized P0→P2 by risk × effort.

**Verdict:** No P0 blockers found. A small set of P1 hardening items (chiefly around the HTTP transport surface and gate-execution symmetry) should land before advertising the networked/multi-tenant paths as production-ready. The local-first, single-agent path is already production-grade.

---

## 2. Methodology

1. Inventoried all specs (`specs/**/spec.md`, `tasks.md`, `progress.md`) and the program-level progress maps.
2. Built, vetted, and ran the full test suite + read coverage per package.
3. Read the security-critical code directly: sandbox runner, custom-gate executor, MCP stdio/HTTP transports, dashboard server, config cascade.
4. Cross-checked CI gates and lint config against the regression specs that claim to enforce them.
5. Grepped for deferred/out-of-scope markers, TODO/FIXME, skipped tests, and missing hardening primitives (timeouts, body limits, auth, sandbox parity).

What this review did **not** do: re-derive every EARS requirement to a line of code (368 source files). It sampled the highest-risk surfaces and the lowest-coverage packages. A full requirement-to-test traceability audit is itself an action item (§5, P2-1).

---

## 3. Per-domain findings

### 3.1 `config/` — config cascade, schema v2, migration, env precedence
**State:** Solid. Embedded defaults → global → project → `SPECD_*` env, then validation. Secret-bearing keys rejected. `docs-test-hardening` spec exists specifically to lock the invariants with regression tests. Byte-identical-output invariant for existing configs is enforced via `omitempty`.

**Findings:**
- **C-1 (P2):** Migration (`migrate-config`) and format precedence (`SPECD_CONFIG_FORMAT`) are well-tested for the happy path. Add an explicit **negative/corruption** matrix: truncated YAML mid-document, JSON with duplicate keys, mixed `config.json` + `config.yml` present simultaneously (which wins, and is the conflict *announced*?). Confirm `doctor` flags the dual-file case rather than silently picking one.
- **C-2 (P2):** Confirm env-override diagnostics never echo values for keys whose *name* matches a secret pattern (the code rejects secret keys, but the diagnostic path should be unit-tested to never print the offending value).

### 3.2 `fusion/` — session bootstrap, schema guardrails, mode sentinel, host/MCP adherence
**State:** Good. `command-schema-guardrails` and `host-mcp-adherence` both ship regression tests that "fail if future changes reintroduce" incomplete metadata / phase-incompatible tool defaults — exactly the right pattern (acceptance criteria encoded as guards, not prose).

**Findings:**
- **F-1 (P1):** `context-manifest-zero-overhead` asserts zero/false overhead when disabled. Verify there is a **measured** guard (a `make perf-gate`-style deterministic check), not only a structural one, so the "zero overhead" claim can't silently regress.
- **F-2 (P2):** Host/MCP adherence is keyed to the currently-supported hosts. Add a contract test that a *new/unknown* host capabilities payload degrades safely (no panic, conservative budget) — the negotiation code already clamps `maxContextTokens<0→0`, so extend that defensive posture to a full fuzz/garbage-input case.

### 3.3 `resilience/` — checkpoint, auto-resume, rate-limit lease, context-snapshot, progress waits, cross-spec recovery
**State:** The most impressive program. Determinism invariant is explicit (`DecideOrchestration` is pure over `(snapshot, policy)`; clock/state enter via `Sense*`). CAS lease ops, suspend cap, idempotent resume, two dedicated CI stress jobs (`stress.sh`, `stress-brain-recovery.sh`).

**Findings:**
- **R-1 (P1):** The stress jobs cover contention and retry/reclaim. Add a **fault-injection** stress variant: kill the host mid-`RecordCheckpoint` (between lease-clear and file-write) and assert no double-claim and no orphaned lease. The pure-decide design should make this safe — prove it under SIGKILL, not just clean cancel.
- **R-2 (P2):** `progress-weighted-waits` trusts a server-stamped `lastReport`. Add a test that a worker reporting progress timestamps *into the future* (clock skew / malicious worker) cannot extend its wait indefinitely — `MaxSteps` is the documented hard bound; assert it actually fires in that case.
- **R-3 (P2):** `cross-spec-recovery` resumes the program DAG. Confirm a resume where a child spec's on-disk state was hand-edited to an impossible status is **rejected with a clear error**, not silently coerced.

### 3.4 `commands/` — interactive-init, steering-console, spec-dashboard, pinky-brain-console, workflow-packaging
**State:** Good, including a dedicated `workflow-packaging-testing` spec for the slash wrappers (`/init`, `/steer`, `/spec`, `/pinky-brain`). Stub detection is deterministic (size + TODO/template markers).

**Findings:**
- **CMD-1 (P1):** `spec-dashboard` is served by `internal/cmd/serve.go` — see the cross-cutting HTTP-hardening findings (§4.1). The dashboard is read-only, which lowers risk, but it's still a long-running listener.
- **CMD-2 (P2):** `pinky-brain-console disable` "warns active sessions may need cancel." Verify the warning is emitted on the actual disable path (and tested), not just specified.

---

## 4. Cross-cutting hardening findings (prioritized)

### 4.1 HTTP transport hardening — **P1**
Both listeners set only `ReadHeaderTimeout: 10s`:

- `internal/mcp/transport_http.go:49` (the `--http` MCP server)
- `internal/cmd/serve.go:186` (the `serve` dashboard)

`grep` confirms **no `WriteTimeout` and no `IdleTimeout` anywhere** in the codebase.

**Risk:** slow-body and slow-read clients can pin a connection/goroutine indefinitely (a slowloris-class resource exhaustion). The `/rpc` body is already bounded (`maxRPCBody` via `ContentLength` check + `io.LimitReader`, good), but the *time* dimension is unbounded once headers are read.

**Action:**
- Set `IdleTimeout` (e.g. 60s) on both servers.
- Set `WriteTimeout` on `serve.go` (static responses, safe to bound).
- On the MCP server, set `WriteTimeout` for `/rpc` but **not** the `/sse` long-lived stream — split handlers or use per-response deadlines so SSE isn't killed mid-stream.

### 4.2 MCP `--http` exposure model — **P1**
`loopbackAddr()` defaults an empty/host-less address to loopback (good default), but an explicit `--http 0.0.0.0:port` binds externally with **no authentication token and no TLS**. `SECURITY.md` documents the sandbox and config-precedence boundaries but does **not** mention the HTTP transport's auth posture.

**Risk:** anyone who binds the MCP server to a non-loopback interface exposes full workflow control (dispatch, phase transitions) unauthenticated.

**Action (pick one, minimum first):**
1. **Document** in `SECURITY.md` + `mcp-guide.md` that `--http` is loopback-only-by-design and binding externally is unsupported/at-operator-risk. (Cheapest, ships now.)
2. Add a **bind-guard warning** (or hard refusal behind a flag) when `--http` resolves to a non-loopback address.
3. Add optional **bearer-token auth** (`SPECD_MCP_TOKEN`) checked on `/rpc` and `/sse` when bound non-loopback.

### 4.3 Gate-execution sandbox asymmetry — **P2**
`verify` runs under a fail-closed sandbox (`bwrap`/container, `--network none`). The **custom gate** executor (`internal/core/customgate.go`) runs an operator-supplied shell command on the host with a scrubbed env but **no sandbox**. Both consume agent-authored spec content.

**Risk:** lower than `verify` (custom gates are operator-opt-in and operator-authored, documented in `custom-gates.md`), but the asymmetry is undocumented and surprising given the project's "untrusted until validated" stance.

**Action:** Document the trust boundary for custom gates explicitly (gate command = trusted operator input), and consider an **opt-in** `--sandbox` for custom gates reusing the `verify` runner, for parity.

### 4.4 HTTP MCP request serialization — **P2 (note / decide-and-document)**
`transport_http.go` wraps dispatch in a single process-wide `sync.Mutex` (`internal/mcp/transport_http.go:61`), serializing **all** `/rpc` and `/sse` handling to one in-flight request.

**Assessment:** likely intentional — preserves the determinism invariant and matches the single-agent local-first model. But it is an undocumented throughput ceiling.

**Action:** Add a one-line comment + a `mcp-guide.md` note stating the server is intentionally single-flight, so it isn't mistaken for a bug or load-tested as if concurrent.

### 4.5 Coverage floors don't cover every package — **P2**
`scripts/coverage-check.sh` floors OVERALL (77), `core` (86), `cmd` (71), `worker` (88), `mcp` (88), `testharness` (80). Measured low spots sit **outside** the per-package floors: `internal/spec` (~50%, only 4 tests; `role.go` is the substantive untested file), plus `internal/runner`, `internal/pack`, `internal/obs`, `internal/context`, `internal/schema` have no individual floor.

**Risk:** regression in an unfloored package can pass CI as long as the 77% overall holds.

**Action:** Add `internal/spec` (role/phase/status logic) to the floor and raise its coverage; add modest floors for `runner`/`pack`/`schema`. This is on the documented ratchet path toward the 85/90/95 targets in `TESTING.md` — just extend the ratchet to the unguarded packages.

---

## 5. Action plan

Ordered for execution. Each item is small enough to be its own spec/PR; suggested gate is "tests/CI prove it."

| # | Item | Priority | Effort | Domain best-practice tie-in |
|---|------|----------|--------|------------------------------|
| A1 | HTTP `IdleTimeout` + scoped `WriteTimeout` on both listeners; SSE exempt (§4.1) | **P1** | S | Web service hardening (slowloris) |
| A2 | `--http` non-loopback: doc + bind-warning, optional `SPECD_MCP_TOKEN` auth (§4.2) | **P1** | S→M | Network service auth/exposure |
| A3 | Fault-injection stress: SIGKILL mid-checkpoint, assert no double-claim (§R-1) | **P1** | M | Crash-consistency / idempotency |
| A4 | Measured zero-overhead guard for context-manifest-disabled (§F-1) | **P1** | S | Perf regression gating |
| A5 | Document custom-gate trust boundary; optional sandboxed custom gates (§4.3) | P2 | S→M | Sandbox parity / least privilege |
| A6 | Config corruption/dual-file matrix + secret-name diagnostic test (§C-1, C-2) | P2 | M | Config robustness / secret hygiene |
| A7 | Garbage host-capabilities fuzz; future/skewed progress timestamp bound (§F-2, R-2) | P2 | M | Untrusted-input defensiveness |
| A8 | Extend coverage floor to `spec`/`runner`/`pack`/`schema`; raise `internal/spec` (§4.5) | P2 | M | Test-coverage ratchet |
| A9 | Comment + doc the single-flight MCP server design (§4.4) | P2 | S | API contract clarity |
| P2-1 | Full EARS-requirement → test traceability audit (one spec at a time) | P2 | L | Requirements verification |
| A10 | Resume rejects impossible hand-edited child status with clear error (§R-3); pinky disable warning test (§CMD-2) | P2 | S | Fail-loud state validation |

**Suggested sequencing:**
- **Sprint 1 (close the networked-path gaps):** A1, A2, A4. Makes the HTTP/MCP surface advertisable as production-ready.
- **Sprint 2 (crash & input hardening):** A3, A6, A7, A10.
- **Sprint 3 (parity & coverage debt):** A5, A8, A9, then the rolling P2-1 traceability audit.

---

## 6. What's already right (keep doing)

- Determinism-as-invariant in the orchestration layer (pure decide, sensed state).
- Acceptance criteria encoded as "fails if future change regresses" guards.
- Fail-closed sandbox + scrubbed env + body-size limits + loopback default.
- Stdlib-only, govulncheck-gated, race + order-dependence + cross-platform CI.
- Ratcheting coverage floor with documented end-state targets.
- Every spec has an explicit `## Out of scope` — scope discipline is real.

The codebase already practices most of what "production hardening" usually has to retrofit. The plan above closes the remaining edges, chiefly on the networked transport and gate-execution-parity surfaces.
