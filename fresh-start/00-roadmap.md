# 00 — Roadmap: spec-authoring order & cross-spec dependency DAG

Order in which the 12 domain analyses become `spec.md` + `tasks.md` and get built in
waves. Ordering is driven by the **cross-spec dependency DAG** below: a domain is
authored only after the domains it structurally depends on.

---

## Cross-spec dependency DAG

```
                 ┌─────────────────────────────┐
                 │ 01 Product & Philosophy Core │  (charter; constrains all)
                 └──────────────┬──────────────┘
                                │
                 ┌──────────────▼──────────────┐
                 │ 10 CLI Architecture &        │  io · lock · CAS · registry ·
                 │    Foundations               │  config · paths · slug
                 └──────┬───────────────┬───────┘
                        │               │
        ┌───────────────▼──┐         ┌──▼──────────────────┐
        │ 02 Spec Lifecycle │◄────────┤ (uses io/lock/CAS)  │
        │  & State Model    │         └─────────────────────┘
        └───┬───────┬───────┘
            │       │
   ┌────────▼──┐ ┌──▼────────────────┐
   │ 04 Task   │ │ 05 Evidence &      │
   │ DAG &     │ │    Verification    │
   │ Waves     │ └──┬─────────────────┘
   └────┬──────┘    │
        │           │
        │    ┌──────▼───────────────┐
        └───►│ 03 Validation Gates  │◄──── 08 (context-budget gate)
             │    Engine            │
             └──────┬───────────────┘
                    │
   ┌────────────────▼───────┐   ┌──────────────────────────┐
   │ 08 Context Engineering  │   │ 06 Agent-Agnostic         │
   │ (internal/context)      │   │    Integration            │
   └───────┬─────────────────┘   └───────────┬──────────────┘
           │                                  │
           │            ┌─────────────────────▼──┐
           │            │ 07 MCP & Handshake      │
           │            │    Surface              │
           │            └─────────┬───────────────┘
           │                      │
   ┌───────▼──────────────────────▼───────┐
   │ 09 Orchestration (Brain/Pinky)         │  (opt-in tier; needs 02/04/05/08/07)
   └───────────────────┬────────────────────┘
                       │
        ┌──────────────▼───────────┐   ┌───────────────────────────┐
        │ 11 Reporting &            │   │ 12 Flywheel (triage tier) │
        │    Observability          │   │  security gate only in v1 │
        └───────────────────────────┘   └───────────────────────────┘
```

**Edges (why):**
- 01 → everything (charter defines what may exist).
- 10 → 02 (state needs io/lock/CAS), 10 → 07 (registry drives tool list), 10 → 09 (config
  authority + file backend).
- 02 → 04, 05, 03, 09, 11 (state is the spine every other domain reads/writes).
- 04 → 03 (DAG gate), 04 → 09 (Brain dispatches the frontier).
- 05 → 03 (evidence gate), 05 → 09 (worker reports validated against records).
- 08 → 03 (context-budget gate), 08 → 07 (MCP `specd_context`), 08 → 09 (worker brief).
- 06 → 07 (handshake surfaces integration; MCP shares role asset map), 06 → 09 (workers
  role-bound).
- 03/07/08 → 09 (orchestration composes gates, tools, and context).
- 09 → 11/12 (reporting/flywheel observe orchestration output; both largely deferred).

---

## Authoring order (topological)

Author specs in this sequence; each is unblocked by the time it is reached.

| Order | Domain file | Rationale |
|---|---|---|
| 1 | `01-product-philosophy-core` | Sets the charter + keep/cut line every other spec cites. |
| 2 | `10-cli-architecture-foundations` | Primitives (io/lock/CAS/registry/config) all specs sit on. |
| 3 | `02-spec-lifecycle-state` | The state spine; unblocks 03/04/05/09/11. |
| 4 | `04-task-dag-wave-execution` | Parser + DAG; unblocks the gate engine and dispatch. |
| 5 | `05-evidence-verification` | Evidence records; unblocks evidence/scope gates + reports. |
| 6 | `03-validation-gates-engine` | Pluggable gates; composes 02/04/05 (+ 08 budget gate later). |
| 7 | `08-context-engineering` | Central manifest engine; feeds 07 and 09. |
| 8 | `06-agent-agnostic-integration` | Roles/steering/adapters; the P5 floor. |
| 9 | `07-mcp-handshake-surface` | Tools over 02–05 + 08; needs 06 + registry (10). |
| 10 | `09-orchestration-brain-pinky` | Opt-in tier; composes 02/04/05/07/08. Author last of core. |
| 11 | `11-reporting-observability` | Pure projections; can trail once state/evidence stable. |
| 12 | `12-flywheel-triage-tier` | Mostly deferred; v1 authors only the security gate module. |

---

## Build waves (implementation, cross-domain)

Authoring order ≠ build order at the task level: once specs exist, independent task waves
run concurrently. Suggested build waves (task ids reference the per-domain DAGs):

- **Wave A — foundations (parallel):** 10 (T10.1–T10.4), 01 (T1.1–T1.2).
- **Wave B — state & primitives close-out:** 10 (T10.5–T10.7), 02 (T2.1–T2.3), 01 (T1.3–T1.4).
- **Wave C — lifecycle & parser (parallel):** 02 (T2.4–T2.7), 04 (T4.1–T4.3), 05 (T5.1–T5.3).
- **Wave D — gates, evidence integrity, dispatch:** 03 (T3.1–T3.3), 05 (T5.4–T5.7),
  04 (T4.4–T4.6).
- **Wave E — context & integration (parallel):** 08 (T8.1–T8.4), 06 (T6.1–T6.7),
  03 (T3.4–T3.6).
- **Wave F — surfaces:** 07 (T7.1–T7.6), 08 (T8.5–T8.7), 11 (T11.1–T11.5).
- **Wave G — orchestration tier:** 09 (T9.1–T9.10).
- **Wave H — flywheel (minimal):** 12 (T12.1–T12.4).

**Critical path:** 01 → 10 → 02 → 05 → 03 → 08 → 09. Orchestration (09) is last because it
composes the most; everything it needs must be green first. Reporting (11) and the flywheel
(12) can slip a wave without blocking the core loop.

---

## Definition of done (per the brief's contract)
A domain spec is ready to build when: (a) its requirements are EARS-shaped and testable;
(b) design names module boundaries + on-disk contracts + preserved invariants; (c) its task
DAG has `id/role/files/depends-on/verify/acceptance` grouped into waves; (d) every claim
cites a specd reference file with a KEEP/SIMPLIFY/REDESIGN/CUT/DEFER verdict. All 12 domain
files in this directory meet (a)–(d); this roadmap sequences them.

**Guardrails re-affirmed for the build phase:** determinism first (no LLM in any
decision/gate/render path); evidence integrity absolute (no task done without a passing
verify record); hard invariants from ADR-8 preserved unless a new ADR changes them;
subtractive bias (default CUT/DEFER when core status is unproven).
