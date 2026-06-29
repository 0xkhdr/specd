# Spec — Untrusted-Input Defensiveness (A7)

**Priority:** P2 · **Wave:** 2 · **Domain:** untrusted-input defensiveness.

## Introduction

Two untrusted-input edges lack defensive coverage. (F-2) Host/MCP adherence is
keyed to currently-supported hosts; a new/unknown host capabilities payload is
not fuzzed for safe degradation. (R-2) `progress-weighted-waits` trusts a
server-stamped `lastReport`; a worker reporting progress timestamps *into the
future* (clock skew / malicious worker) is not proven unable to extend its wait
indefinitely.

This spec adds a garbage-host-capabilities fuzz and a future/skewed-timestamp
bound test.

## Current-state grounding

- `specs/fusion/host-mcp-adherence` — negotiation clamps `maxContextTokens<0→0`
  (existing defensive posture to extend).
- `specs/resilience/progress-weighted-waits` — trusts server-stamped
  `lastReport`; `MaxSteps` documented as the hard bound.
- Host capabilities negotiation code; progress-wait decision code (pure-decide).

## Requirements

### Requirement 1 — Garbage host-capabilities degrade safely
**User story:** As a host integrator, I want an unknown/garbage capabilities
payload to never panic, so a bad host cannot crash specd.

**Acceptance criteria:**
1. A new/unknown host capabilities payload SHALL NOT panic.
2. Garbage/missing fields SHALL yield a conservative budget (extend the existing
   `maxContextTokens<0→0` clamp to a full garbage-input case).
3. A fuzz or table of malformed payloads SHALL cover negative, oversized, nil,
   and type-mismatched fields.

### Requirement 2 — Future/skewed progress timestamp cannot extend wait
**User story:** As an operator, I want a worker stamping future timestamps unable
to wait forever, so clock skew or a malicious worker cannot stall the program.

**Acceptance criteria:**
1. A worker reporting `lastReport` in the future SHALL NOT extend its wait
   beyond the documented bound.
2. `MaxSteps` SHALL actually fire in that case (assert it terminates the wait).
3. The decision SHALL remain pure over `(snapshot, policy)` — skew enters via
   sensed state, not the decision.

## Design

- Add `internal/.../host_caps_fuzz_test.go` (Go native fuzzing + a seeded table)
  feeding malformed capabilities; assert no panic + conservative clamp.
- Add a progress-wait test stamping `lastReport` into the future and asserting
  `MaxSteps` bounds/terminates the wait.

## Out of scope

- Authenticating the host capabilities payload (negotiation, not auth).
- Trusting wall-clock for skew correction (bound is step-based by design).

## Risks

- **Fuzz nondeterminism:** keep a seeded corpus committed so CI is reproducible.
