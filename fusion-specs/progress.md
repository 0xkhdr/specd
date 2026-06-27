# Fusion Program — Progress & Wave Plan

Source plan: [`specd-coding-agent-fusion-analysis.md`](../specd-coding-agent-fusion-analysis.md)

This program makes specd agent-ready from first turn: bootstrap the session, expose a machine-readable command schema, align context loading with phase/mode, then harden host/MCP adherence.
Each child spec below has its own `spec.md` (requirements + design) and `tasks.md` (wave DAG).

## Spec map

| Gap / focus | Title | Spec | Priority |
|---|---|---|---|
| Session bootstrap oracle | Fusion Session Bootstrap | [session-bootstrap](session-bootstrap/spec.md) | P0 |
| Zero-error command discovery | Command Schema Guardrails | [command-schema-guardrails](command-schema-guardrails/spec.md) | P0 |
| Binding config + mode sentinel | Configuration and Mode Sentinel | [configuration-mode-sentinel](configuration-mode-sentinel/spec.md) | P0 |
| Phase-scoped context alignment | Context Manifest Zero-Overhead Alignment | [context-manifest-zero-overhead](context-manifest-zero-overhead/spec.md) | P1 |
| Host/MCP adherence protocol | Host and MCP Adherence Protocol | [host-mcp-adherence](host-mcp-adherence/spec.md) | P1 |

## Program waves

The program is staged so the startup oracle lands first, then the agent can discover schema/config safely, then the host surface is tightened.
Each wave starts once its listed specs are in flight; intra-spec waves live in each `tasks.md`.

### Wave 1 — Bootstrap foundation (P0) — **status: not-started**

| Spec | Status | Depends on | Notes |
|---|---|---|---|
| session-bootstrap | not-started | — | Add `specd fusion bootstrap` as the session-start oracle. |
| command-schema-guardrails | not-started | session-bootstrap/T3 | Add richer command metadata and `help <command> --json`; schema docs depend on bootstrap being available. |

### Wave 2 — Policy and context alignment (P0/P1) — **status: not-started**

| Spec | Status | Depends on | Notes |
|---|---|---|---|
| configuration-mode-sentinel | not-started | — | Strict config diagnostics plus `specd fusion policy`. |
| context-manifest-zero-overhead | not-started | session-bootstrap/T3 | Make `specd context` the load oracle with mode-aware items, selectors, and budget actions. |

### Wave 3 — Host and MCP adherence (P1) — **status: not-started**

| Spec | Status | Depends on | Notes |
|---|---|---|---|
| host-mcp-adherence | not-started | session-bootstrap/T3; configuration-mode-sentinel/T4; command-schema-guardrails/T3,T5 | Surface bootstrap/policy in MCP, document delegate-mode behavior, and harden phase-compatible exposure. |

## Status legend

`not-started` → `in-progress` → `verifying` → `complete` / `blocked`

## How to track

Each child `tasks.md` owns its checkbox DAG (flip with `specd task`, never by hand).
Update the per-wave status tables above as child specs advance.
The program is `complete` when all five child specs are `complete`.

## Open program-level decisions

- **Schema compatibility:** all new metadata should remain additive and `omitempty` so `specd help --json` and MCP mirrors stay backward-compatible.
- **Bootstrap budget:** the bootstrap oracle should stay small by default; inline schema remains opt-in.
- **Mode protocol:** `subagentMode` and `orchestration.enabled` are binding configuration, but the program should prefer read-only sentinels and advisory guidance over enforcement in host-facing surfaces.
