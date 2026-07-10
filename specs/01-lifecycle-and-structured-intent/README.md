# 01 — Lifecycle and Structured Intent

## Intent

Google SDLC alignment means controlled feedback loops, not fast artifact bypass.
Human owns intent + architecture. Harness proves structure, authority, freshness, scope, evidence.

Current strength: forward lifecycle, human approval, task DAG/frontier, atomic/CAS state, evidence pinned to git HEAD.

Current gap: harness proves artifacts exist; cannot prove approved intent covers design/tasks, detect major-change staleness, or guide planning-phase agents with exact legal actions.

## Program boundary

Included: requirements/design/task contract, traceability, planning coverage, lifecycle amendments, phase-native guidance, profile/risk policy, bounded spikes, lifecycle conformance/metrics.

Excluded: worker dispatch, model routing, external adapters, deployment, runtime telemetry. Those domains consume this domain contracts later.

## Deliverable specs

| Wave | Slug | Result | Requires |
|---|---|---|---|
| W0 | `01a-lifecycle-contract-foundation` | Versioned requirement/criterion parsing + migration | — |
| W0 | `01b-planning-guardrail-baseline` | Known role enforcement; role-aware verify-quality baseline | — |
| W1 | `01c-design-trace-and-decision` | Deterministic design trace + human architecture disposition | 01a |
| W1 | `01d-phase-native-guidance` | Actor-aware legal next-action contract; scaffold parity | 01b |
| W2 | `01e-task-trace-and-risk` | Task refs/risk/context/evidence declarations | 01a,01c |
| W3 | `01f-planning-coverage-gates` | Intent/design/task coverage + boundary checks | 01e |
| W4 | `01g-amendment-staleness` | Change-impact graph, stale records, dispatch pause | 01f |
| W5 | `01h-production-profile` | Default/production evidence policy | 01f,01g |
| W6 | `01i-bounded-spikes` | Prototype records cannot bypass approval/evidence | 01h |
| W7 | `01j-lifecycle-conformance-metrics` | Fresh-project recovery/conformance suite + reports | 01c-01i |

## DAG

```text
01a ─┬─> 01c ─┐
     │        ├─> 01e ─> 01f ─> 01g ─> 01h ─> 01i ─┐
01b ─┴─> 01d ┘                                      ├─> 01j
                                                      │
01c/01d conformance fixtures ────────────────────────┘
```

## Mandatory program rules

1. Contract parser/model first; gate second; CLI/state mutation third; docs/scaffold fourth.
2. Failing black-box/conformance test first for every changed public contract.
3. References/digests, not copied content, join records.
4. Existing specs load/migrate or fail with actionable migration error; never silently reinterpret.
5. No qualitative evaluator creates deterministic pass.
6. Finish W4 before treating profile/eval additions as authoritative.

## Completion claim

Domain complete only when fresh release binary drives clean repo through requirements, design, tasks, amendment, reapproval, execution evidence; rejects shallow/unknown/stale/illegal paths; reports deterministic coverage and staleness.
