# Wave 2 — Seal the Trust Boundary

> **Order:** 3 / 7 · **Depends:** W1 (shares `lifecycle.go`/`registry.go`; mode enum from R1.3)
> **Findings:** F4 (MCP self-approval), F11 (brain half-closed; pinky unregistered)
> **Sources:** PROJECT.md §8 Wave P2, BUILD_REVIEW.md §5 Wave 2, specs/07 + 09, ADR-3/7
> **Files:** `internal/core/manifest_tools.go`, `internal/mcp/`, `internal/cmd/{brain,pinky,registry}.go`

P6 inverted: today an orchestrated agent can call `approve` over MCP — the one verb
that exists to record human judgment — and `brain start` creates sessions without
`orchestration.enabled`. This wave makes the agent-facing surface incapable of crossing
human gates, and the orchestration tier fail-closed as ADR-3 mandates.

## 1. Purpose & principles

- **Principles owned:** P6 (human gates), P5 (agent-agnostic surface with a hard floor).
- **Harness components:** guardrails (deny list), orchestration (fail-closed tier).

## 2. Requirements (EARS)

- **R2.1** When an MCP client lists or calls tools, the system shall exclude and refuse
  `approve`, `init`, `mcp`, and `brain` (and `task complete` unless evidence-backed —
  the R1.1 gate applies identically over MCP): `tools/list` shall not include them and
  `tools/call` shall return a policy error. The MCP↔CLI parity test shall assert the
  deny list itself, so removing an entry breaks CI.
- **R2.2** When `brain start` runs without both `orchestration.enabled: true` in config
  and spec `mode: orchestrated`, the system shall refuse to create a session (exit
  non-zero with the specific missing precondition) — session creation itself is inert,
  not merely dispatch (ADR-3 fail-closed).
- **R2.3** The system shall resolve the pinky surface by ADR: either register
  `pinky {claim|heartbeat|report|inbox|checkpoint}` in `core.Commands` per ADR-3, or
  record a superseding ADR deferring the worker CLI and delete `internal/cmd/pinky.go`.
  Dead unreachable surface shall not remain.
- **R2.4** When a malformed per-spec `manifest.json` tool policy is loaded, the system
  shall degrade to an empty policy (deny-by-default for optional tools), never open
  (spec 07 contract — re-asserted here because the deny list depends on it).

## 3. Design

- **Deny list (R2.1):** one exported set in `manifest_tools.go` (single source), consumed
  by both tool registration (list filter) and call-time policy check (defense in depth —
  a tool absent from the list must still refuse if called by name). Parity test iterates
  the set and asserts absence from `tools/list` + policy error from `tools/call`.
- **Fail-closed brain (R2.2):** precondition check at the top of `brain start` before any
  file is written; reuses W1's mode enum. Config read fail-loud (full fix in W5 R5.2;
  here the orchestration block specifically must not default-enable on parse error).
- **Pinky decision (R2.3):** default recommendation — register the verbs (ADR-3 is the
  governing ADR; the code exists). If deferring instead, the superseding ADR goes in
  `docs/charter.md`'s decision log and the file is deleted; either way surface == ADR.

## 4. Invariants preserved

- ADR-3: compiled always, inert unless enabled, fail-closed; disabled ⇒ zero
  orchestration behavior and byte-unchanged CLI/check output.
- Report/decision/memory stay non-tools over MCP (spec 07: the tool set is
  enforcement/query, not authoring).
