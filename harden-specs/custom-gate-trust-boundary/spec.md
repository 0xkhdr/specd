# Spec — Custom-Gate Trust Boundary & Optional Sandbox (A5)

**Priority:** P2 · **Wave:** 3 · **Domain:** sandbox parity / least privilege.

## Introduction

`verify` runs under a fail-closed sandbox (`bwrap`/container, `--network none`).
The custom-gate executor (`internal/core/customgate.go`) runs an operator-supplied
shell command on the host with a scrubbed env but **no sandbox**. Both consume
agent-authored spec content. The risk is lower than `verify` (custom gates are
operator-opt-in and operator-authored, per `docs/custom-gates.md`), but the
asymmetry is undocumented and surprising given the project's "untrusted until
validated" stance.

This spec documents the trust boundary explicitly and adds an opt-in `--sandbox`
for custom gates reusing the `verify` runner for parity.

## Current-state grounding

- `internal/core/customgate.go` — runs operator shell command, scrubbed env, no
  sandbox.
- `internal/spec/runner_sandbox.go` — the `verify` fail-closed sandbox runner
  (`bwrap`/container, `--network none`).
- `docs/custom-gates.md`, `docs/validation-gates.md` — gate docs.

## Requirements

### Requirement 1 — Document the trust boundary
**User story:** As an operator, I want the custom-gate trust model stated, so the
sandbox asymmetry is not surprising.

**Acceptance criteria:**
1. `docs/custom-gates.md` SHALL state the gate command is trusted operator input
   (NOT agent-authored), run on the host without a sandbox by default.
2. The doc SHALL contrast this with `verify`'s fail-closed sandbox.

### Requirement 2 — Opt-in sandbox for custom gates
**User story:** As a cautious operator, I want to run a custom gate sandboxed, so
I get parity with `verify` when I want it.

**Acceptance criteria:**
1. A `--sandbox` opt-in SHALL run the custom-gate command via the existing
   `verify` sandbox runner.
2. When unset, behavior SHALL be unchanged (host execution, scrubbed env).
3. When the sandbox backend is unavailable, the opt-in SHALL fail closed with a
   clear error (consistent with `verify`).

### Requirement 3 — Scrubbed env preserved in both modes
**User story:** As an operator, I want env scrubbing in sandboxed mode too.

**Acceptance criteria:**
1. The scrubbed-env guarantee SHALL hold in both host and sandboxed modes.
2. A test SHALL assert no secret-bearing env leaks into either execution path.

## Design

- Refactor `customgate.go` to route execution through a strategy: host runner
  (current) or the `verify` sandbox runner when `--sandbox` is set.
- Reuse `internal/spec/runner_sandbox.go` rather than duplicating sandbox logic.
- Add docs to `custom-gates.md` describing the boundary and the opt-in.

## Out of scope

- Sandboxing custom gates by default (would break operator workflows).
- Network policy beyond reusing `--network none` from the verify runner.

## Risks

- **Backend availability divergence:** reuse the verify runner's fail-closed
  behavior so the two paths stay consistent.
