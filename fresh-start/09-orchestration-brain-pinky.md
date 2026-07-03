# Domain: Orchestration (Brain/Pinky)

## 1. Purpose & value mapping
- **Principles served:** P1 (deterministic controller enforces; workers create), P3
  (worker completion validated against evidence), P4 (Brain dispatches the DAG frontier).
- **Paper concept realized:** *orchestration logic* + the **orchestrator mode** (pp.32–33):
  "the developer operates at a higher level of abstraction … agents may be working in the
  background, in parallel." The Brain is the deterministic scaffolding that makes async,
  multi-agent delegation safe.
- **Core use case:** a developer enables orchestration on a spec; a **Brain** (deterministic
  state machine, **never calls an LLM**) senses spec state and emits decisions
  (`dispatch`/`wait`/`await-approval`/`complete`); ephemeral **Pinky** workers claim tasks
  over a file-backed protocol, do the work, and report evidence. The harness guarantees
  evidence integrity, lease safety, cost/time brakes, and cooperative cancel — the "system
  design" skill the paper says the orchestrator needs (p.33).
- **If none → CUT:** not core to the MVP's *default* path, but core to the product thesis.
  **Ships as a clean opt-in tier**, off by default.

## 2. Current-state analysis (from specd)
- **Reference files read:** `docs/agent-integration.md` (Brain/Pinky sections),
  `internal/cmd/{brain.go,brain_commands.go,brain_policy.go,brain_worker.go,pinky.go,
  conductor.go,orchestrate.go}`, `internal/core/orchestration*.go` (11 files),
  `internal/core/acp*.go` (5 files), `internal/core/program*.go` (9 files),
  `internal/core/{escalation,routing,guardrails,cost_brake,trajectory,session_replay}.go`,
  `internal/worker/*`.
- **What exists today; key contracts/invariants:**
  - **Two layers:** Brain (deterministic decisions — invariant: *"The Brain never calls an
    LLM and never executes unsafe code directly"*) and Pinky (ephemeral workers).
  - **Brain verbs:** `start`, `step`, `run` (driver loop), `directive`, `cancel`, `pause`,
    `resume` (`--list`), `checkpoint`, `status`. **Decision actions:** `dispatch`, `wait`,
    `awaiting-approval`, `escalate`, `policy-violation`, `complete-session` (+ authoring:
    `dispatch-authoring`, `advance-phase`; `resume-from-checkpoint`).
  - **Pinky verbs:** `claim`, `heartbeat`, `progress`, `query`, `report`, `inbox`, `brief`,
    `checkpoint`.
  - **Safety machinery:** per-spec lock at claim; lease keepalive via heartbeat, reclaim
    after `leaseSeconds`, reschedule up to `maxRetries`; **advisory cost brake** (sums
    untrusted `host-cost`, halts with `policy-violation` over `hostReportedCostLimitUSD`);
    **time brake** (mission deadline, process-group kill); **evidence integrity** (report
    accepted only if it matches a passing `specd verify` record); **cooperative cancel**
    (records intent, workers self-stop). All interaction is file-backed via the ACP.
  - **Program tier:** an entire second control plane for multi-spec orchestration
    (`program*.go`, ~55K core).
  - **Supporting mass:** `escalation.go` (12K), `routing.go` (11K, model-tier routing),
    `guardrails.go` (10K), `trajectory.go`, `session_replay.go`, `replay.go`.
- **Redundancy / complexity / drift found (evidence):**
  - This is the tree's dominant complexity: orchestration ~133K + ACP ~65K + program ~55K +
    pinky ~42K + support ~50K ≈ **350K of `internal/core` source**, vs ~120K for the entire
    lifecycle+gates+parser core. The command surface alone is brain (4 files) + pinky +
    conductor + orchestrate.
  - Overlapping surfaces: `conductor` (rejection clustering analytics) and `orchestrate`
    (auto-escalation resolution) are separate commands adjacent to `brain`; the program tier
    duplicates session/lease/decide logic at a higher level.

## 3. Fresh-start decision
- **Verdict per capability:**
  - Deterministic controller (no LLM) — **KEEP, as the load-bearing invariant.**
  - File-backed ACP transport — **KEEP** (auditable, restart-safe, dependency-free).
  - Evidence integrity on worker reports — **KEEP, absolute** (report ⇔ passing verify
    record; one completion path shared with domain 05).
  - Lease/lock + heartbeat reclaim + retries — **KEEP** (safety core).
  - Cost brake + time brake + cooperative cancel — **KEEP** (the guardrails that make async
    delegation safe; small, high-value).
  - Brain verb surface — **SIMPLIFY** to the minimum: `start`, `step`, `run`, `status`,
    `approve`, `cancel`, `resume`. Fold `directive` into `run`/`step` inbox handling; drop
    `pause` (cancel+resume covers it); keep `checkpoint` only if resilience is enabled.
  - Pinky verb surface — **SIMPLIFY** to `claim`, `heartbeat`, `report`, `inbox`,
    `checkpoint`. Make `progress`/`query`/`brief` optional (telemetry/ergonomics, not
    safety).
  - `conductor` command (rejection analytics) — **DEFER** (domain 12; a report, not control).
  - `orchestrate` command — **CUT** (auto-escalation folds into Brain decisions).
  - Program (multi-spec) tier — **DEFER** entirely to v2.
  - `routing.go` (model-tier routing) — **DEFER** (host concern; specd stores routing as
    evidence but should not pick models in v1).
  - Postgres/Redis-backed sessions — **CUT** (file backend only; see domain 10).
- **Minimal accurate surface:**
  - Commands: `brain {start|step|run|status|approve|cancel|resume}`,
    `pinky {claim|heartbeat|report|inbox|checkpoint}`.
  - Modules: `internal/orchestration/{sense.go,decide.go,session.go,driver.go,acp.go,
    lease.go,brakes.go}` — a *single* control plane, compiled always but inert unless
    `orchestration.enabled`.
  - On-disk: `.specd/specs/<slug>/orchestration/{session.json,acp/*.jsonl,leases/*}`.
- **Architecture & flexibility improvements:**
  - **One control plane, not three.** Collapse orchestration + conductor + orchestrate +
    program into a single `internal/orchestration` package with a pure `Decide(snapshot) →
    Decision` core (already the shape of `DecideOrchestration`) and thin IO around it.
  - **Pure decision function** is the testable heart: `Decide` takes a `Snapshot` (built by
    `Sense`) and returns a `Decision` with zero IO and zero randomness — determinism is
    provable, not asserted.
  - **Fail-closed authority** (unchanged from today's model): default `enabled:false`,
    `approvalPolicy:manual`, `workerMode:host`, `transport:file`; no policy can clear
    high/critical mid-requirement gates.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. The system shall make every Brain decision a pure function of a sensed snapshot,
   invoking no language model and performing no IO in the decision function.
2. When a worker submits a completion report, the system shall accept it only if it
   references a passing verify record for that task; otherwise it shall reject the report.
3. When a worker's lease expires without a heartbeat, the system shall reclaim the task and
   reschedule it up to `maxRetries`, then escalate.
4. When summed host-reported cost exceeds `hostReportedCostLimitUSD` (>0), the system shall
   halt the session with a `policy-violation` decision.
5. When a mission deadline passes, the system shall terminate the worker's process group and
   record the timeout.
6. When cancellation is requested, the system shall record intent and emit directives; it
   shall not itself kill external host processes beyond the deadline mechanism.
7. When `orchestration.enabled` is false, the system shall expose no orchestration behavior
   and `check`/CLI output shall be unaffected.
8. The system shall persist all Brain↔worker interaction to a file-backed ACP log usable
   for restart recovery.

## 5. Design notes — seed for design.md
- **Module boundaries:** `internal/orchestration/{sense,decide,session,driver,acp,lease,
  brakes}.go`; `internal/cmd/{brain,pinky}.go` are thin dispatchers; `brain_worker.go`
  keeps the injectable `worker.Runner` seam for tests.
- **Key types:** `Snapshot` (sensed spec+frontier+leases), `Decision{Action,Task?,Reason}`,
  `Action` enum (`dispatch|wait|await-approval|escalate|policy-violation|complete`),
  `Lease{Worker,Task,Attempt,Expiry}`, `Report{Task,VerifyRef,GitHead,ChangedFiles,
  DurationMs,HostCost,HostTokens}`.
- **Data/on-disk contracts:** `session.json` (CAS-guarded), append-only `acp/*.jsonl`,
  `leases/*`; host telemetry stored verbatim, never trusted as proof.
- **Invariants to preserve:** no LLM in the controller; evidence integrity; lease safety;
  cost/time brakes; cooperative cancel; fail-closed authority; file-backed recovery;
  duplicate terminal reports idempotent.
- **External interfaces:** ACP wire format; `worker.Runner` seam; MCP brain tools (domain
  07); the frontier from domain 04; verify records from domain 05.

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — pure core (no IO)
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T9.1 | craftsman | `internal/orchestration/decide.go` | — | `go test ./internal/orchestration -run TestDecidePure` | Decide is pure, deterministic, no IO |
| T9.2 | craftsman | `internal/orchestration/sense.go` | — | `go test ./internal/orchestration -run TestSense` | snapshot built from state+frontier+leases |
| T9.3 | craftsman | `internal/orchestration/brakes.go` | — | `go test ./internal/orchestration -run TestBrakes` | cost>limit and deadline→halt/timeout |
### Wave 2 — transport & leases
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T9.4 | craftsman | `internal/orchestration/acp.go` | T9.2 | `go test ./internal/orchestration -run TestACPRoundtrip` | append-only; restart-recoverable |
| T9.5 | craftsman | `internal/orchestration/lease.go` | T9.2 | `go test ./internal/orchestration -run TestLeaseReclaim` | expiry reclaim + retries + escalate |
| T9.6 | craftsman | `internal/orchestration/session.go` | T9.4 | `go test ./internal/orchestration -run TestSessionCAS` | session.json CAS under lock |
### Wave 3 — commands & integrity
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T9.7 | craftsman | `internal/cmd/brain.go`, `internal/orchestration/driver.go` | T9.1,T9.5,T9.6 | `go run . brain run demo --worker-cmd ...` | driver loop dispatches frontier |
| T9.8 | craftsman | `internal/cmd/pinky.go`, `internal/cmd/brain_worker.go` | T9.4 | `go test ./internal/cmd -run TestReportRequiresVerify` | report rejected without passing record |
| T9.9 | craftsman | orchestration authority config | T9.6 | `go test ./internal/orchestration -run TestFailClosedAuthority` | disabled by default; can't clear high/critical gates |
| T9.10 | validator | `internal/orchestration/decide_test.go` | T9.1 | `go test ./internal/orchestration -run TestNoLLM` | grep proves no model/network import in decision path |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** collapsing three control planes into one loses a real multi-spec use case
  (program tier). Mitigation: program tier is DEFER, not CUT; the single-plane `Decide` is
  designed to be liftable to program scope later.
- **Risk:** trusting host-reported cost/tokens. Mitigation (retained): stored verbatim as
  evidence, never as proof; the cost brake is explicitly advisory and fail-safe.
- **Open question:** keep `checkpoint`/resume in v1 or defer resilience? Proposed: ship the
  ACP + lease recovery (cheap, high-value) but make explicit `checkpoint` opt-in via
  `resilience.checkpointEnabled` (off by default), matching today.
- **Cross-domain deps:** domain 04 (frontier to dispatch), domain 05 (verify records gate
  reports), domain 02 (`mode: orchestrated` eligibility + CAS), domain 08 (worker brief =
  context manifest), domain 07 (brain MCP tools), domain 10 (file backend, lock, config).
