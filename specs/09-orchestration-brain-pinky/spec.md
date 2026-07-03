# Spec 09 — Orchestration (Brain/Pinky)

> **Authoring order:** 10 / 12 · **Critical path:** yes (author last of core — composes the most)
> **Sources:** `fresh-start/09-orchestration-brain-pinky.md`, paper pp.32–33
> **ADRs:** ADR-3, ADR-7, ADR-8, ADR-9
> **Reference:** `reference/internal/cmd/{brain,brain_commands,brain_worker,pinky,conductor,orchestrate}.go`, `reference/internal/core/orchestration*.go`, `reference/internal/core/acp*.go`, `reference/internal/worker/*`

A developer enables orchestration on a spec; a deterministic **Brain** (never calls an LLM)
senses state and emits decisions; ephemeral **Pinky** workers claim tasks over a file-backed
protocol, do the work, and report evidence. Ships as a clean **opt-in tier**, off by default.
This is the largest subtraction in the tree: ~350K of `internal/core` collapses into one
control plane (ADR-3).

---

## 1. Purpose & principles
- **Principles owned:** P1 (deterministic controller enforces; workers create), P3 (worker
  completion validated against evidence), P4 (Brain dispatches the DAG frontier).
- **Paper concept:** *orchestration logic* + the **orchestrator mode** (pp.32–33): "the
  developer operates at a higher level of abstraction … agents may be working in the
  background, in parallel."

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| Deterministic controller (no LLM) | **KEEP, load-bearing invariant** | ADR-8 |
| File-backed ACP transport | **KEEP** | Auditable, restart-safe, dep-free |
| Evidence integrity on worker reports | **KEEP, absolute** (report ⇔ passing verify record) | One completion path shared with Spec 05 |
| Lease/lock + heartbeat reclaim + retries | **KEEP** | Safety core |
| Cost brake + time brake + cooperative cancel | **KEEP** | Guardrails that make async delegation safe |
| Brain verbs | **SIMPLIFY** → `start,step,run,status,approve,cancel,resume` | Fold `directive` into inbox; drop `pause` |
| Pinky verbs | **SIMPLIFY** → `claim,heartbeat,report,inbox,checkpoint` | `progress/query/brief` optional |
| `conductor` command (rejection analytics) | **DEFER** (Spec 12) | A report, not control |
| `orchestrate` command | **CUT** | Auto-escalation folds into Brain decisions |
| Program (multi-spec) tier | **DEFER** entirely to v2 | ADR-9 |
| `routing.go` (model-tier routing) | **DEFER** | Host concern; store routing as evidence only |
| Postgres/Redis-backed sessions | **CUT** (file backend only) | ADR-9; Spec 10 |

**Minimal surface:** `brain {start|step|run|status|approve|cancel|resume}`, `pinky
{claim|heartbeat|report|inbox|checkpoint}`; single `internal/orchestration` package, compiled
always but **inert unless `orchestration.enabled`** (fail-closed).

## 3. Requirements (EARS)
- **R9.1** The system shall make every Brain decision a pure function of a sensed snapshot,
  invoking no language model and performing no IO in the decision function.
- **R9.2** When a worker submits a completion report, the system shall accept it only if it
  references a passing verify record for that task; otherwise it shall reject the report.
- **R9.3** When a worker's lease expires without a heartbeat, the system shall reclaim the task
  and reschedule it up to `maxRetries`, then escalate.
- **R9.4** When summed host-reported cost exceeds `hostReportedCostLimitUSD` (>0), the system
  shall halt the session with a `policy-violation` decision.
- **R9.5** When a mission deadline passes, the system shall terminate the worker's process
  group and record the timeout.
- **R9.6** When cancellation is requested, the system shall record intent and emit directives;
  it shall not itself kill external host processes beyond the deadline mechanism.
- **R9.7** When `orchestration.enabled` is false, the system shall expose no orchestration
  behavior and `check`/CLI output shall be unaffected.
- **R9.8** The system shall persist all Brain↔worker interaction to a file-backed ACP log
  usable for restart recovery; duplicate terminal reports shall be idempotent.

## 4. Design

### Module boundaries
- `internal/orchestration/{sense,decide,session,driver,acp,lease,brakes}.go` — one control
  plane. `internal/cmd/{brain,pinky}.go` — thin dispatchers. `brain_worker.go` keeps the
  injectable `worker.Runner` seam for tests.
- **Pure decision function is the testable heart:** `Decide(Snapshot) → Decision` — zero IO,
  zero randomness. Determinism is provable, not asserted.

### Key types
- `Snapshot` (sensed spec+frontier+leases), `Decision{Action, Task?, Reason}`, `Action` enum
  (`dispatch|wait|await-approval|escalate|policy-violation|complete`), `Lease{Worker, Task,
  Attempt, Expiry}`, `Report{Task, VerifyRef, GitHead, ChangedFiles, DurationMs, HostCost,
  HostTokens}`.

### On-disk contracts
- `.specd/specs/<slug>/orchestration/{session.json, acp/*.jsonl, leases/*}`. `session.json`
  CAS-guarded; `acp/*.jsonl` append-only; host telemetry stored **verbatim, never trusted**.
- Fail-closed authority defaults: `enabled:false`, `approvalPolicy:manual`, `workerMode:host`,
  `transport:file`; no policy can clear high/critical mid-requirement gates.

### External interfaces
- ACP wire format; `worker.Runner` seam; MCP brain tools (Spec 07); the frontier (Spec 04);
  verify records (Spec 05); worker brief = context manifest (Spec 08).

## 5. Invariants preserved (ADR-8)
No LLM in the controller; evidence integrity; lease safety; cost/time brakes; cooperative
cancel; fail-closed authority; file-backed recovery; duplicate terminal reports idempotent.

## 6. Cross-domain dependencies
- Depends on: Spec 04 (frontier), Spec 05 (verify records), Spec 02 (`mode: orchestrated`
  eligibility + CAS), Spec 08 (worker brief), Spec 07 (brain MCP tools), Spec 10 (file backend,
  lock, config).

## 7. Risks & open questions
- **Risk:** collapsing three control planes loses the multi-spec (program) use case. → program
  tier is DEFER, not CUT; the single-plane `Decide` is designed to be liftable to program scope.
- **Risk:** trusting host cost/tokens. → stored verbatim as evidence, never as proof; cost
  brake explicitly advisory + fail-safe.
- **Open (resolved):** ship the ACP + lease recovery (cheap, high-value); make explicit
  `checkpoint`/resume opt-in via `resilience.checkpointEnabled` (off by default).
