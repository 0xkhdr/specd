# Domain 05 — Orchestration, Multi-Agent Work, and Model Routing

## Purpose

Define how `specd` should coordinate one or more coding agents, preserve role and evidence boundaries across handoffs, and support cost/complexity-aware model selection without placing an LLM inside the deterministic decision path.

## Paper position

The comparison document associates this domain with four paper concepts:

- **Conductor mode:** a developer directs an agent and retains detailed control.
- **Orchestrator mode:** a developer designs and supervises a system of delegated workers.
- **MCP and A2A:** tools and cross-agent handoffs need interoperable, explicit contracts.
- **Intelligent model routing:** expensive, capable models should be reserved for reasoning that needs them; routine work should use cheaper models.

The paper treats multi-agent operation as a production system, not merely multiple prompts. Identity, authority, state, context, retries, cost, and evidence must survive every handoff. It does not require that routing itself be non-deterministic; `specd` can preserve its stronger rule that deterministic policy chooses an eligible route while an external provider performs model inference.

## Current `specd` handling

### Strong foundations

- `internal/core/dag.go` and `internal/core/frontier.go` derive safe concurrent waves from task dependencies.
- Tasks carry a role, file scope, dependencies, verify command, and acceptance criteria in the byte-stable `internal/core/tasksparser.go` format.
- `internal/core/agents.go` and `internal/core/scaffold.go` install and inspect Pinky role artifacts for Codex and Claude.
- `internal/orchestration/decide.go` chooses a stable first frontier task, honors dispatch authority, evaluates brakes, and surfaces escalation.
- `internal/cmd/brain_run.go` requires opt-in configuration and explicit `--authority`, withholds live-leased/escalated tasks, persists session state with CAS, writes a checkpoint before dispatch, and prevents duplicate deterministic mission ids.
- `internal/orchestration/acp.go` provides an append-only ledger with dispatch, claim, and report shapes; it has attempt, HEAD, changed-files, verify-reference, and telemetry fields.
- Resume logic reconciles checkpoint, ledger, and lease state; race tests assert at-most-once dispatch identity.
- `internal/cmd/brain_worker.go` defines useful report acceptance invariants: live matching lease, passing evidence, resolvable HEAD, and report/evidence HEAD equality.

### Material gaps between the documented and executable surface

1. **Brain records dispatch; it does not launch a worker.** `runBrainStep` appends an ACP dispatch and a lease whose holder is `brain`. There is no provider/host adapter invocation in that path.
2. **Claim/report validation is not wired into a public workflow.** `AppendClaim`, `ACPKindReport`, and `acceptWorkerReport` exist, but repository search shows no non-test command path that accepts a worker claim/report and appends the complete lifecycle.
3. **The current lease holder is not a real worker identity.** A `brain` holder is sufficient for controller bookkeeping but cannot prove which external agent received or performed the mission.
4. **Role prompts are installed, not dispatched.** Pinky configuration proves artifacts exist; it does not prove that the role matching the task was invoked with the matching mission/context/lease.
5. **Cost/deadline brakes are latent.** `DecisionLimits` supports maximum cost and deadline, but `brain_run.go` currently supplies only `MaxRetries: 1`; no production configuration connects budget/deadline to the run.
6. **Model routing is absent.** No task risk/complexity class, model capability policy, provider adapter, route reason, or model identity is selected and recorded.
7. **A2A interoperability is absent.** ACP is an internal append-only ledger, not an A2A-compatible envelope or transport.
8. **Parallelism is computed but operational ownership is incomplete.** Brain can lease multiple frontier tasks over repeated steps, yet there is no end-to-end worker registration, claim, heartbeat, result, lease renewal, cancellation, or lost-worker recovery protocol exposed to hosts.

The current implementation should therefore be described precisely as a deterministic dispatch ledger/controller foundation, not yet a hands-off production multi-agent executor.

## Common contract and fields

| Field | Paper-side purpose | `specd` support | Target rule |
|---|---|---|---|
| `protocol_version` | Interoperable handoff | ACP shape is implicit | Version mission, claim, heartbeat, and report envelopes. |
| `session_id` / `mission_id` | Correlation and idempotency | implemented | Mission id is stable and unique per session/step/task. |
| `spec_slug` / `task_id` | Work identity | partial in session/ACP | Include both in every envelope and evidence reference. |
| `role` | Capability boundary | task row and prompts | The claimed worker capability must satisfy exactly the assigned role. |
| `authority` | Human/system delegation | Brain `--authority` | Carry scope, issuer, expiry, allowed actions, and revocation status. |
| `declared_files` / `acceptance` / `verify` | Bounded mission | task row/context | Snapshot or digest in the mission so drift invalidates stale work. |
| `context_ref` / `context_digest` | Reproducible dynamic context | manifest exists | Pin the exact versioned context envelope used by the worker. |
| `worker_id` / `host` | Accountability | lease field exists | Stable registered identity; never substitute controller identity. |
| `lease_id` / `issued_at` / `expires_at` | Exclusive ownership | task/worker/expiry exist | Add unique lease identity, heartbeat/renewal policy, and revocation reason. |
| `attempt` / `retry_policy` | Bounded recovery | claim counting, escalation | Define which failures are retryable and who can clear escalation. |
| `risk` / `complexity` / `capabilities_required` | Routing input | absent | Deterministic metadata, human-reviewable before execution. |
| `model_policy` / `route` / `route_reason` | Economic routing | absent | Policy chooses an eligible class; provider/model used is recorded. |
| `token_budget` / `cost_budget` / `deadline` | Operational brakes | partial types | Enforced limits with stable units and explicit unknown-data behavior. |
| `changed_files` / `git_head` / `verify_ref` / `eval_refs` | Result integrity | partial ACP/report validation | Server-computed where possible and gated before task completion. |
| `status` / `blocker` / `next_action` | Supervision | distributed | Typed state transition for operator and driver recovery. |

## Gaps and failure modes

- An operator reads “Brain dispatches Pinky workers,” starts `brain run`, and assumes code is being implemented when only ledger entries and leases were written.
- A real worker performs work without a public claim path; its identity, attempt, role compatibility, and lease ownership are not durably established.
- A report validator function exists but is bypassed because the external integration marks completion through the ordinary task path.
- A stale task packet is executed after requirements, design, task scope, or configuration changed.
- Two heterogeneous workers receive equal tasks, but there is no capability policy to prevent a low-capability/low-context route from taking architecture-critical work.
- Cost annotations are self-reported and optional; a cost brake cannot be trusted when missing data silently behaves as zero.
- Lease expiration detects staleness but there is no heartbeat or cancellation delivery, so a slow valid worker and a dead worker look alike.
- A2A conversion loses `verify_ref`, role authority, or context digest, weakening `specd` evidence semantics at the interoperability boundary.

## Target best-practice workflow

1. **Plan:** approved tasks declare risk, complexity, required capabilities, context class, and evidence requirements in addition to role/scope.
2. **Sense and route:** deterministic Brain code computes the frontier and evaluates authority, retries, deadline, cost, and capability policy. It emits a mission; it does not ask an LLM which gate to bypass.
3. **Deliver:** a host adapter transports the versioned mission to a registered worker. Local CLI, MCP host, and A2A are adapters over the same envelope.
4. **Claim:** the worker submits identity, capabilities, selected provider/model class, task/context digests, and requested lease. The harness atomically validates and records the claim.
5. **Work:** the worker loads bounded context, performs exactly one role-scoped task, and emits heartbeat/telemetry events without mutating harness state directly.
6. **Report:** the harness validates live lease, mission/task/role match, actual changed-file scope, evidence and eval references, and git HEAD. Provider/model and cost are facts attached to the run, not completion proof.
7. **Complete or recover:** passing evidence permits the normal completion gate. Retryable failures consume the attempt policy; timeout, policy violation, or exhausted budget escalates to a human.
8. **Audit:** the session ledger reconstructs dispatch-to-completion causality and exports an interoperable redacted view.

## Recommended action plan

### P0 — Make orchestration claims match executable behavior

1. Update public documentation to distinguish **recorded dispatch** from **worker execution** until a host adapter is wired. **Acceptance:** a black-box Brain test asserts the exact files/events written and docs never claim a process/model was launched.
2. Add public, versioned `brain claim`, `brain heartbeat`, and `brain report` (or equivalent) command/MCP envelopes backed by the existing ACP and report checks. **Acceptance:** no report reaches completion without a matching live claim, correct role/task/mission, passing current-HEAD evidence, and an allowed diff.
3. Replace the controller-as-worker lease with a pending dispatch followed by a real worker lease. **Acceptance:** every active lease names a registered worker and unique lease id; undispatched/unclaimed work is distinguishable.
4. Include role, scope, acceptance, verify, context digest, palette digest, and config digest in the mission contract. **Acceptance:** modifying any pinned input causes a stale claim/report to fail closed.
5. Add an end-to-end fixture that installs Pinky artifacts, dispatches a fake host worker, claims, verifies, reports, completes, and resumes after injected crashes. **Acceptance:** the fixture proves at-most-once mission identity and at-least-once safe recovery without duplicate completion.

### P1 — Add deterministic capability and economic routing

1. Add vendor-neutral routing policy to `project.yml` or a dedicated versioned artifact: role/risk/complexity to capability class, budget, and fallback. Avoid a provider-specific core format. **Acceptance:** identical policy/task inputs produce identical eligible route classes and reasons.
2. Add task metadata for `risk`, `complexity`, and `capabilities`; lint values and require human approval with tasks. **Acceptance:** high-risk tasks cannot route to a class lacking required review/eval/sandbox capabilities.
3. Wire cost/deadline/retry limits from validated config into `DecisionLimits`. Define missing telemetry as `unknown`, never zero. **Acceptance:** exceeded or unobservable required budgets brake before another dispatch.
4. Let optional host adapters resolve a capability class to a provider/model and record the actual route. **Acceptance:** provider failure follows declared fallback/escalation policy and never changes gate semantics.
5. Server-compute changed files and git identity when the worker is local. **Acceptance:** worker-reported scope disagreement is retained as a finding and completion is refused.

### P2 — Add portable multi-agent interoperability

1. Define A2A import/export mappings for mission, claim, progress, cancel, and report while preserving `specd` digests and evidence references. **Acceptance:** canonical round-trip fixtures lose no required field and unknown protocol versions fail closed.
2. Add adapter conformance tests for local subprocess, MCP host, and A2A transport. **Acceptance:** all transports produce the same ACP semantic event stream for the same fixture.
3. Support bounded parallel claims per wave with declared shared-file conflict policy. **Acceptance:** tasks with overlapping write scopes cannot hold concurrent write leases unless an approved coordination rule exists.
4. Add revocation and cancellation acknowledgement. **Acceptance:** a revoked/expired lease can never submit a completion-eligible report.

## Production validation scenarios

| Scenario | Expected result |
|---|---|
| Brain without authority | Waits and writes no dispatch, lease, evidence, or worker event. |
| Brain with authority but no adapter | Records a pending mission and clearly reports that no worker was launched. |
| Valid worker lifecycle | Register → dispatch → claim → heartbeat → verify/eval → report → complete has one correlated mission and lease. |
| Worker role mismatch | Claim is rejected before work authority is granted. |
| Two workers race to claim | Exactly one lease wins; loser receives a typed conflict. |
| Controller crashes around dispatch | Resume reconciles checkpoint/ledger and never duplicates mission identity. |
| Worker dies | Lease expires, retry policy advances once, and stale reports are refused. |
| Context/config/task drift | Claim or report fails because its pinned digests are stale. |
| Budget or deadline exhausted | New dispatch is blocked; human escalation identifies the measured/unknown input. |
| High-risk task routed cheaply | Policy refuses a capability class that lacks required eval/review/sandbox features. |
| A2A round trip | Role, scope, authority, digests, attempt, and evidence references remain intact. |

## Context-safety considerations

- Missions should reference a bounded manifest rather than copy repository content into ACP/A2A ledgers.
- Routing policy should use structural metadata, not full prompts or source files.
- Workers receive only their task, role, declared files, relevant design/steering, and required tool schemas.
- Provider/model identity and token counts belong in telemetry; they should not be injected into future reasoning unless needed for routing or audit.
- Redacted interoperable exports must omit secrets, raw prompts, and source content while retaining hashes, policy decisions, and evidence identity.
- Retry should rebuild context only when a pinned input changed; unchanged retries can reuse the manifest digest.

## Non-goals and risks

- `specd` should not embed model SDKs or add runtime dependencies to core orchestration. Providers belong behind optional external adapters.
- Model routing optimizes capability/cost; it does not replace human approval for architecture, risk exceptions, or production release.
- A2A compatibility must not weaken fail-closed evidence or authority semantics to fit a lowest-common-denominator protocol.
- More agents do not automatically improve throughput. File conflicts, duplicated context, review load, and cost can outweigh parallelism.
- Self-reported worker identity, files, and cost are insufficient in adversarial settings; use server-observed facts where the deployment allows it.
