# Domain 11 — Workflow coherence

## Goal

Make one fresh-project workflow truthful end to end. Generated instructions, command metadata,
machine guidance, lifecycle state, authority, evidence, completion, docs, and task rollup agree.
Agent needs no tribal knowledge; harness never advertises illegal or falsely read-only action.

## Source

Derived from `docs/sdlc-paper-current-reference-analysis.md` findings F1-F9. Paper intent:
structure, tests plus evals, progressive skills, human architecture judgment, deterministic
guardrails, and observable production workflow.

## Ownership

| Area | Domain 11 owns | Existing domain retained |
|---|---|---|
| Lifecycle | exact one-step transition and approval UX | Domain 01 artifact/gate semantics |
| Operations | canonical actor/effect/authority metadata | Domain 03 rendering; Domain 06 enforcement |
| Completion | executable base-agent evidence-to-complete route | Domain 04 evidence validity |
| Skills/templates | shipped progressive core pack and current authoring shape | Domain 02 selection/budget/trust |
| Truth | normative docs, typed doctor result, rollup parity | Domains 03/08/09 projections |
| Proof | fresh default and production golden workflows | Domains 04-10 specialized contracts |

## Waves

| Wave | Result | Requires |
|---|---|---|
| W0 | Baseline fixtures and canonical coherence contract | — |
| W1 | Exact lifecycle and simple approval | W0 |
| W2 | Per-operation actor/effect/authority metadata | W1 |
| W3 | Complete executable base-agent task loop | W2 |
| W4 | Progressive skills and production-shaped templates | W3 |
| W5 | Documentation, diagnostics, and rollup truth | W4 |
| W6 | Fresh-project default/production release proof | W5 |

## Program rules

1. One wave per coding-agent turn. Tests first. Stop at verified wave boundary.
2. No LLM/network/provider call in gate, DAG, lifecycle, report, or trusted context decision.
3. No task completion without current passing verify evidence pinned to resolvable HEAD.
4. Human approval/exception stays human-only. Skill/template prose grants no authority.
5. Preserve old state/task/config decoding or fail with explicit migration. Zero runtime deps.
6. Derive CLI, MCP, handshake, guidance, examples, and docs from canonical contracts.
7. Never edit, import, build, or copy from `reference/`; use it only as analyzed history.

## Completion claim

A no-prior-knowledge agent can initialize fresh project and drive one default task from intent to
completion using only generated guidance. Production fixture additionally proves authority,
scope, sandbox, security, quality evidence, independent review, and exact human approvals.
Every emitted operation has truthful actor/effect metadata. Every example runs. Rollup equals
task truth. Clean doctor JSON is typed.
