# Domain 03 — Agent-tool driving, native guidance

## Goal

Make `specd` self-describing, truthful, safe driver for any coding host. Agent reasons;
harness returns current state, exact legal action, required actor, bounded context, typed blocker.
No remembered commands, hidden active spec, stale prompt, or model prose may authorize work.

## Source

Derived from `docs/google-sdlc-alignment/README.md` and
`docs/google-sdlc-alignment/03-agent-tool-driving-and-native-guidance.md`.
Paper intent: Agent = Model + Harness. Model supplies judgment. Harness supplies legible tools,
selected context, deterministic limits, human boundaries, evidence distinction.

## Ownership

| Area | Domain 03 owns | Other domain owns |
|---|---|---|
| Bootstrap | root/spec discovery, compatibility, managed-guidance drift | Domain 02 context receipt/schema |
| Driver | phase/frontier/blocker projection; legal next actions | Domain 01 lifecycle/task metadata |
| Commands | active-spec resolution, exact generated examples, doctor | Domain 06 actual authority/scope enforcement |
| MCP | same driver semantics, typed CLI handoff | Domain 10 adapter/capability transport |
| Context | route and required-item semantics exposed to host | Domain 02 manifest selection/budget |
| Completion | status/blocker/evidence reference display only | Domain 04 evidence/eval gate authority |

## External prerequisites

- Domain 01 must stabilize known roles, phase legality, task rows, and approval semantics.
- Domain 02 owns manifest V2 source/digest/budget contract. Domain 03 consumes it; does not
  duplicate resolver or selector policy.
- Domain 06 owns post-work role/tool/scope enforcement. Domain 03 labels authority only.
- Domain 04 owns evidence class/freshness validity. Domain 03 only projects immutable reference.

## Deliverable specs

| Wave | Slug | Result | Requires |
|---|---|---|---|
| W0 | `03a-guidance-contract-baseline` | Versioned envelope, finding codes, black-box defect fixtures | — |
| W1 | `03b-truthful-context-and-scaffold` | Resolvable paths; executable managed examples | 03a, Domain 02 V2 base |
| W1 | `03c-active-spec-and-agent-doctor` | Explicit/pinned spec resolution; read-only health findings | 03a |
| W2 | `03d-driver-next-action-projection` | Phase-valid actor-aware guide envelope | 03a,03b,03c |
| W3 | `03e-guidance-drift-and-mcp-handoff` | Guidance/schema digests; typed human/CLI handoff | 03d |
| W4 | `03f-host-conformance-and-capabilities` | Host-neutral fixture suite; capability negotiation | 03e, Domain 10 contract |
| W5 | `03g-remote-dispatch-envelope` | Pinned remote dispatch claim/report envelope | 03f, Domain 05/06 authority |

## DAG

```text
03a ─┬─> 03b ─┐
     ├─> 03c ─┼─> 03d ─> 03e ─> 03f ─> 03g
     │        │
Domain 02 V2 ┘
```

## Program rules

1. Pure versioned projection before CLI/MCP renderer. Single palette remains command authority.
2. Add failing fresh-fixture/conformance test before public contract repair.
3. Return IDs, paths, digests, codes, references. Never bulk-copy repo or hidden reasoning.
4. Explicit operand wins pinned host setting. Ambiguous selection fails closed.
5. Guidance may recommend action; never performs approval, exception, completion, or scope bypass.
6. Existing CLI/MCP output stays compatible behind versioned JSON or gets documented migration.
7. Keep core deterministic, stdlib-only, network/model-free. Never edit `reference/`.

## Completion claim

Fresh binary plus generated guidance can drive fresh project through first human approval and
one task lifecycle using CLI or MCP. Every emitted command/path resolves; every mutation has
phase/actor authority; stale guidance blocks mutation; host results match across conformance
fixture. Completion authority/evidence remains deterministic core gate.
