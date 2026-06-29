# Progress — Spec Hardening Program

**Source:** `specd-spec-hardening-analysis.md` (2026-06-29, `main` @ `3d41aa5`).
**Verdict from analysis:** no P0 blockers. P1 items close the networked-path
gaps; P2 items are crash/input hardening + parity + coverage debt.

This file tracks the wave-by-wave status of all hardening specs. Update the
status column as tasks land.

## Wave map (sequencing from analysis §5)

### Wave 1 — Close networked-path gaps (P1) → advertisable HTTP/MCP surface
| Spec | Item | Priority | Status |
|------|------|----------|--------|
| [http-transport-timeouts](http-transport-timeouts/spec.md) | A1 | P1 | done |
| [mcp-http-exposure-auth](mcp-http-exposure-auth/spec.md) | A2 | P1 | done |
| [context-manifest-perf-gate](context-manifest-perf-gate/spec.md) | A4 | P1 | done |

### Wave 2 — Crash & input hardening (P1/P2)
| Spec | Item | Priority | Status |
|------|------|----------|--------|
| [checkpoint-fault-injection](checkpoint-fault-injection/spec.md) | A3 | P1 | not started |
| [config-corruption-matrix](config-corruption-matrix/spec.md) | A6 | P2 | not started |
| [untrusted-input-fuzz](untrusted-input-fuzz/spec.md) | A7 | P2 | not started |
| [state-validation-faillaud](state-validation-faillaud/spec.md) | A10 | P2 | not started |

### Wave 3 — Parity & coverage debt (P2)
| Spec | Item | Priority | Status |
|------|------|----------|--------|
| [custom-gate-trust-boundary](custom-gate-trust-boundary/spec.md) | A5 | P2 | not started |
| [coverage-floor-extension](coverage-floor-extension/spec.md) | A8 | P2 | not started |
| [single-flight-doc](single-flight-doc/spec.md) | A9 | P2 | not started |
| [ears-traceability-audit](ears-traceability-audit/spec.md) | P2-1 | P2 | rolling — not started |

## Per-spec task tallies

| Spec | Tasks | Done |
|------|-------|------|
| http-transport-timeouts | 4 | 4 |
| mcp-http-exposure-auth | 4 | 4 |
| context-manifest-perf-gate | 3 | 3 |
| checkpoint-fault-injection | 4 | 0 |
| config-corruption-matrix | 3 | 0 |
| untrusted-input-fuzz | 2 | 0 |
| state-validation-faillaud | 3 | 0 |
| custom-gate-trust-boundary | 3 | 0 |
| coverage-floor-extension | 3 | 0 |
| single-flight-doc | 2 | 0 |
| ears-traceability-audit | 3 (rolling) | 0 |
| **Total** | **34** | **11** |

## Notes / invariants to preserve while hardening

- Determinism: `DecideOrchestration` stays pure over `(snapshot, policy)`; clock
  and state enter via `Sense*`. Do not regress this in A3/A7.
- Zero runtime deps (stdlib only). Do not add a dependency for A2 auth — use
  `crypto/subtle`.
- SSE long-lived stream must survive A1 write bounds.
- Coverage floors only ratchet up, never down (A8).
- Each spec is independently shippable; docs-only sub-tasks (A2/T1, A5/T1, A9)
  can land ahead of code.

## Gate (per analysis §5)

Each item's gate = "tests/CI prove it." A spec is complete only when its tasks'
`verify` commands pass and (where applicable) CI wiring is in place.
