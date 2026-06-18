# Spec: Brain Deterministic Orchestrator

Status: proposed — awaiting human review
Scope: deterministic sensing, planning, dispatch, monitoring, recovery, and escalation over existing specd state.

## 1. Outcome

Brain turns specd's existing state machine into a resumable orchestration loop. It does not reason with an LLM, author artifacts, edit source, or bypass gates. Creative work is represented as missions for a host-provided Pinky worker.

> Brain schedules. Pinky works. specd enforces.

## 2. Responsibilities

- Sense state through existing core APIs, never by inventing a parallel world model.
- Select one deterministic next action from status, gate, frontier, active leases, retry policy, and limits.
- Emit dispatch packets based on the same task view as `specd dispatch`.
- Monitor ACP events and reconcile them with authoritative task state.
- Resume safely after interruption and escalate when no policy-authorized action exists.
- Produce a deterministic, replayable decision event for every action.

## 3. Requirements

- R3.1 Introduce a pure `Decide(snapshot, policy) -> Decision` engine with table-driven coverage for every spec status and gate.
- R3.2 Build snapshots through `LoadSpec`, phase helpers, gate results, runnable frontier, and active ACP leases.
- R3.3 Reuse core validation functions directly; never shell out to the `specd` binary from inside specd.
- R3.4 Support decisions: `idle`, `request-approval`, `dispatch`, `wait`, `retry`, `cancel`, `replan`, `escalate`, `complete-session`.
- R3.5 Dispatch only tasks returned by the existing runnable frontier and only within concurrency/lease limits.
- R3.6 Apply retries only to transport/worker failures; failed verification returns work for correction and never becomes passing evidence.
- R3.7 Make every mutating reconciliation use `WithSpecLock`, revision CAS, and idempotency keys.
- R3.8 Add `specd brain start|status|step|pause|resume|cancel`; `step` performs at most one externally visible decision.
- R3.9 Keep `start` foreground and bounded by default; no hidden daemon.
- R3.10 Require explicit session scope, approval policy, timeout, and limits at start.
- R3.11 Record append-only structured events through the ACP/session event log and expose them through existing replay/report surfaces.
- R3.12 On restart, reconstruct all state from specd files and ACP events; no indispensable in-memory state.
- R3.13 Escalate on unknown status, cycle, orphan dependency, stale conflicting lease, CAS conflict exhaustion, policy violation, or repeated worker failure.
- R3.14 Embed Brain role/skill guidance for agent hosts, but keep it advisory and versioned.
- R3.15 Preserve manual approval by default; automated approvals require explicit bounded policy and remain impossible for high/critical mid-requirement gates.

## 4. Decision Model

```text
sense -> validate -> reconcile leases -> decide one action -> record -> act -> record outcome
```

Examples:

- planning artifact invalid: dispatch a builder mission describing the failed gate.
- planning artifact valid + manual policy: request human approval.
- executing + runnable frontier: dispatch up to available worker capacity.
- executing + active workers: wait.
- evidence event: confirm a matching authoritative verification record, then invoke existing task completion logic.
- verifying: request human approval unless a bounded session policy explicitly permits final approval.

## 5. State and Concurrency

- `session.json` stores session policy, lifecycle status, owner, created time, and last committed sequence.
- ACP events store decisions and outcomes.
- Only one active Brain lease may control a spec per session; program mode owns child spec leases.
- Pause stops new dispatches. Cancel sends directives and waits for lease expiry or acknowledgement; it does not kill an unknown host process.

## 6. Invariants

- V1 Same snapshot plus policy yields the same decision.
- V2 Brain cannot complete a task without an existing passing verification record and satisfied dependencies.
- V3 Brain cannot clear a human-only gate.
- V4 One `step` creates at most one new dispatch/approval/escalation action.
- V5 Crash/restart cannot duplicate a committed transition.
- V6 No goroutine, timer, map iteration, or wall clock leaks nondeterminism into output.
- V7 Brain adds no LLM/provider dependency and performs no network call.

## 7. Acceptance

- Decision matrix, recovery, idempotency, pause/resume/cancel, lock/CAS, and deterministic-output tests pass.
- An end-to-end fake-host scenario advances a spec without bypassing verification or approval policy.
- `make ci` passes, including race and stress gates.
