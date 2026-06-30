# Spec — Security Hardening (S1)

## Introduction

specd executes user-authored shell commands (`verify:` lines, custom gates) and
exposes an MCP server that turns arbitrary JSON-RPC `arguments` maps into CLI
invocations. A live-evidence pass (see `../discrepancies.md` D1–D4, D12)
confirmed that most of the security surface assumed risky by the original
analysis plan is already correctly implemented: checksum-verified installs,
fail-closed sandboxing with no bypass, and complete path-traversal guards via
`ValidateSlug`. This spec narrows to the one confirmed gap — MCP
argument-shape validation at the transport boundary — plus regression tests
that pin the already-correct behaviors so future changes can't silently
regress them.

**Out of scope (do not implement):** removing `sh -c` from `internal/runner`
(intentional design per `SECURITY.md`, see discrepancy D1); adding checksum
verification to `scripts/install.sh` (already present, D2); new path
sanitization code (already complete, D3); graceful degradation/fallback for
missing bwrap/container runtimes (would weaken INV3, D4).

## Requirement 1 — MCP argument-shape validation gate

**User story:** As an operator running `specd mcp --http` on a shared host, I
want malformed or unexpected MCP tool-call arguments rejected before they
reach command dispatch, so that a malicious or buggy MCP client cannot smuggle
unexpected flags/values into a command invocation.

**Acceptance criteria:**
1. WHEN an MCP tool-call request's `arguments` map contains a key not declared
   in that tool's parameter schema THE SYSTEM SHALL reject the call with a
   JSON-RPC error (no argv construction, no command dispatch).
2. WHEN an MCP tool-call request's `arguments` map contains a value whose type
   does not match the declared parameter type (e.g., a number where a string
   is expected, an array where a scalar is expected) THE SYSTEM SHALL reject
   the call with a JSON-RPC error.
3. THE SYSTEM SHALL perform this validation in `buildArgv()` (or an
   equivalent gate invoked before it), before any per-command validation
   (e.g. `ValidateSlug`) runs — defense in depth, not a replacement for it.
4. THE SYSTEM SHALL NOT change the JSON-RPC error code/shape for requests that
   already fail downstream per-command validation — this requirement only
   adds a new rejection path for shape violations, it does not alter existing
   behavior for shape-valid-but-semantically-invalid requests.

## Requirement 2 — Regression guards for already-correct security behavior

**User story:** As a maintainer, I want the security properties already
proven correct by this review (fail-closed sandboxing, slug validation,
checksum-required install) to be pinned by tests, so a future refactor can't
silently reintroduce a bypass.

**Acceptance criteria:**
1. THE SYSTEM SHALL have a test asserting `ValidateSlug` rejects every input
   containing `..`, `/`, or a leading character outside `[a-z0-9]`, covering
   the full `SlugRE` pattern.
2. THE SYSTEM SHALL have a test asserting `SelectRunner` returns an error (not
   a usable runner) when bwrap is requested but `exec.LookPath("bwrap")`
   fails, and equivalently for the container backend when the
   docker/podman binary or `SPECD_SANDBOX_IMAGE` is missing.
3. THE SYSTEM SHALL have a test asserting `scripts/install.sh` exits non-zero
   on checksum mismatch and only skips verification when `--no-verify` is
   explicitly passed.
4. IF any of the above three tests fails THEN CI SHALL block merge (already
   true via `make ci` — no new CI wiring needed, only the tests themselves).

## Requirement 3 — Sandbox-unavailability diagnostics

**User story:** As a developer running `specd verify` for the first time on a
new machine, I want to know *why* it refused to run before I hit a raw Go
error, so I can fix my environment instead of filing a false bug report.

**Acceptance criteria:**
1. WHEN `specd doctor` runs AND `verify.sandbox` is configured to `bwrap` or
   `container` AND the required binary is not on `PATH` THE SYSTEM SHALL
   report the specific missing dependency and the install command for the
   current OS, as a `doctor` finding (not a fatal error — `doctor` always
   completes).
2. THE SYSTEM SHALL NOT change `SelectRunner`'s fail-closed behavior — this
   requirement only adds an earlier, friendlier diagnostic surface.

## Design

### Overview
Add one new validation function at the MCP boundary, three regression tests
for already-correct behavior, and one new `doctor` check. No runtime behavior
changes for already-passing requests.

### Architecture
`internal/mcp/server.go`'s `buildArgv()` (lines ~387-417) currently performs a
single type check (`args` is `[]any`) before converting the `arguments` map to
a CLI argv. A new `validateToolArgs(tool string, args map[string]any) error`
function, called at the top of `buildArgv()`, checks the map's keys/types
against a per-tool schema (a small Go struct/map literal per tool, not a
third-party JSON Schema dependency — stays stdlib-only per `go.mod`'s
documented constraint).

### Components and interfaces
- `internal/mcp/argschema.go` (new) — per-tool argument schema definitions
  and `validateToolArgs`.
- `internal/mcp/server.go` — `buildArgv()` calls `validateToolArgs` first;
  on error, returns the existing JSON-RPC error path (reuse, don't invent a
  new error shape).
- `internal/core/slug_test.go` — extend with the property/fuzz test from
  Requirement 2.1 (file likely already exists per discrepancy D3 evidence —
  builder task must confirm before adding, not assume).
- `internal/runner/runner_sandbox_test.go` — extend with Requirement 2.2
  tests if not already covered (builder task must grep first).
- `scripts/install_test.sh` or equivalent shell test harness — Requirement
  2.3 (check `TESTING.md` for the existing shell-test convention before
  choosing a location).
- `internal/cmd/doctor.go` — add the sandbox-availability check.

### Data models
No persisted schema changes. `validateToolArgs`'s per-tool schema is an
in-memory Go literal, not a new file format.

### Error handling
Schema-violation errors use the same JSON-RPC error envelope already returned
by `buildArgv()` for its existing type check — no new error code namespace.

### Verification strategy
- Unit: `internal/mcp/argschema_test.go` — table-driven, one case per tool ×
  (valid, unknown-key, wrong-type).
- Integration: extend `internal/mcp/integration_test.go` with one
  end-to-end malformed-arguments case.
- Regression: the three tests from Requirement 2, run under `make test -race`.

### Risks and open questions
- Per-tool schema must stay in sync with each command's actual flags as new
  commands/flags are added — mitigate with a drift test similar to
  `TestSchemaConformance` (compare declared MCP schema against
  `cmd.Registry`/`CommandMeta`).
- Open question: should `validateToolArgs` schemas be hand-written or derived
  from `internal/core/commands.go`'s `CommandMeta`? Hand-written risks drift;
  derived risks coupling MCP validation to CLI flag parsing internals. Builder
  must record the decision taken in a short comment, not silently pick one.
