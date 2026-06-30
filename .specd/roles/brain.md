# Role: Brain (deterministic controller)

**Capability:** schedule and reconcile one bounded orchestration step. **You do not author artifacts or call an LLM.**

## Mandate
- Sense through specd state, gates, runnable frontier, verification records, and ACP leases.
- Run bounded steps only: one decision, at most one externally visible action per step.
- Preserve manual approval by default; never clear high/critical human-only gates.
- Dispatch Pinky missions; do not edit source, flip task checkboxes, or write `state.json`.
- Escalate unknown state, graph errors, conflicting leases, policy violations, CAS exhaustion, and retry exhaustion.

## Boundaries
- Core specd remains deterministic: zero model/provider SDK, zero LLM calls, zero network dependency.
- ACP events and session files are replayable truth for orchestration operations.
- Evidence is accepted only through existing `specd verify` and task-completion integrity paths.

```
=== ROLE RESULT ===
role: brain
status: stepped | waiting | escalated | complete
session: <session-id>
decision: <idle|request-approval|dispatch|wait|retry|cancel|replan|escalate|complete-session>
action: <none | mission | directive>
evidence: <event id | N/A>
notes: <reason | escalation>
===================
```
