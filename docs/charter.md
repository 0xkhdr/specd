# Harness Charter

> **The charter rule (PROJECT.md §1).** Every shipped verb maps to exactly one **harness
> component** and at least one **principle**. Unmapped code does not merge. This file is the
> authoritative map and the future lint source (review-specs W1); its verb list must match the
> registry (`internal/core/commands.go`, `Commands`) exactly.

## The seven harness components

`Agent = Model + Harness`. The harness is the ~90% the team owns. Every verb is one of:

1. **instructions** — rule/role material handed to the agent.
2. **tools** — deterministic actions the agent invokes.
3. **sandboxes** — scrubbed, bounded execution environments.
4. **orchestration** — DAG/wave/controller logic that sequences work.
5. **guardrails** — gates that block a state change without proof or approval.
6. **observability** — truthful projections of `state.json`.
7. **context** — bounded, progressive-disclosure context assembly.

## The eight principles

P1 Foundational Split · P2 Specs as Source of Truth · P3 Evidence Gates Every State Change ·
P4 Waves, Not Lines · P5 Agent-Agnostic by Design · P6 Human Gates at Phase Boundaries ·
P7 Deterministic Reporting · P8 Steering as Constitution.

## Verb → component + principle

Every row below is a registered verb (`specd help --json` order). Each names its one **harness
component** and its governing principle.

| Verb | Harness component | Principle |
|---|---|---|
| `help` | instructions | P5 — one standardized interface across every host |
| `init` | tools | P8 — writes the steering constitution + roles |
| `new` | tools | P2 — the plan is created as versioned Markdown on disk |
| `approve` | guardrails | P6 — human approval at a phase boundary |
| `midreq` | guardrails | P6 — scoped mid-stream change is gated, not silent |
| `decision` | guardrails | P6 — an explicit human decision is recorded |
| `next` | orchestration | P4 — selects the ready DAG frontier, not a flat line |
| `status` | observability | P7 — a projection of `state.json`, never generated |
| `task` | observability | P2 — reads task truth from the spec/state |
| `check` | guardrails | P1 — the harness enforces the gate registry |
| `verify` | guardrails | P3 — records evidence before any completion |
| `context` | context | P2 — bounded manifest assembled from on-disk specs |
| `memory` | context | P8 — durable steering-memory patterns outlive sessions |
| `mcp` | tools | P5 — the same palette served to any MCP client |
| `handshake` | instructions | P5 — bootstrap/policy material for agent onboarding |
| `brain` | orchestration | P4 — opt-in deterministic wave controller |
| `report` | observability | P7 — evidence-backed reports computed, never authored |
| `triage` | orchestration | P4 — **deferred** (registered stub; review-specs W5 subtracts it, F7) |

### Notes on scope drift (audited)

- `triage` is a registered **deferred** stub (`Deferred: true`). It is the 18th verb against the
  §5 target of 16; W5/R5.1 resolves the count by subtraction, not by wiring the stub.
- `pinky` (orchestration worker CLI) is **deferred and unshipped** (W2/R2.3, superseding the
  earlier "register it" intent under ADR-5 subtractive bias). `internal/cmd/pinky.go` was deleted
  rather than wired: the worker verbs `{claim,heartbeat,report,inbox,checkpoint}` were dead,
  unreachable surface (never in `Commands`, never dispatched). ADR-3 keeps the orchestration
  *tier* compiled-and-inert; a worker CLI is not required for that tier to be fail-closed-correct,
  so shipping it now would be accreted mass. Re-entry seam: register the verbs in
  `internal/core/commands.go` `Commands` (mapping to **orchestration** / P4, added to this table)
  when a real driver consumes them — no re-architecture. `TestSurfaceMatchesADR` pins that the
  registry carries no `pinky` surface until then.
