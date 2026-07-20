# Coding Agent ↔ specd Communication and Enforcement Analysis

> Status: Architecture analysis and recommendations.
> Scope: Discovery, guidance, authority, execution, evidence, and reporting between coding agents and `specd`.

## Executive summary

`specd` already has the right philosophical foundation: **the agent reasons; the harness enforces**. Its strongest mechanisms—forward-only lifecycle transitions, human approval boundaries, digest-pinned handshakes, task authority, DAG frontiers, commit-pinned verification, and evidence-gated completion—correctly move trust away from agent claims and into deterministic state.

The relationship is not yet completely native, however. Some controls are mandatory only when a host uses the production profile, MCP authority packets, and sandbox support. Other expectations still depend on the agent reading `AGENTS.md`, choosing to bootstrap, loading referenced context, respecting role prose, and avoiding unrestricted shell or file-edit tools.

The target should not be “the model can never say something false.” No probabilistic model can provide that guarantee. The enforceable target is:

> In a governed session, an agent may misunderstand, request an invalid operation, or write an inaccurate report; none of those actions can modify trusted state, exceed granted scope, advance the lifecycle, or create completion without fresh deterministic evidence.

The highest-priority improvement is to turn the current collection of excellent primitives into a mandatory **specd driver session** owned by the host rather than the model.

## 1. The intended relationship

The agent and harness have different responsibilities.

| Concern | Coding agent | specd |
|---|---|---|
| Problem understanding | Reasons over requirements and code | Validates required structure and traceability |
| Design | Proposes boundaries, interfaces, and tradeoffs | Gates artifact completeness and approval state |
| Task planning | Decomposes work | Validates IDs, roles, paths, dependencies, DAG, and verification |
| Work selection | Executes an assigned task | Computes the runnable frontier |
| Authority | Requests capabilities | Grants explicit, bounded, expiring authority |
| Implementation | Edits allowed source files | Enforces declared scope through host-integrated controls |
| Verification | Requests verification and interprets failures | Executes/records deterministic evidence |
| Completion | Reports its work | Decides whether completion is permitted |
| Approval | May request a decision | Reserves semantic approval for a human |
| Reporting | Provides a narrative | Produces authoritative state and evidence reports |

This separation is the product. `specd` must avoid becoming an AI planner itself; it should remain the deterministic control plane around AI reasoning.

## 2. Current discovery path

### 2.1 Repository discovery

`specd init` installs a managed `AGENTS.md` region. It tells an agent to bootstrap a handshake, request actor-aware guidance, load task context, perform one role-scoped task, verify, complete, and check coherence.

`specd agents`, `specd agents doctor`, and `specd agents guide <slug>` provide additional discovery and diagnostics. MCP exposes tools from the same canonical command palette used by CLI dispatch and documentation.

### 2.2 Strengths

- The repository carries its own workflow instructions.
- Managed guidance has a digest, allowing drift detection.
- The command palette is a single source of truth.
- MCP and CLI project the same underlying operations.
- The active spec can be pinned through configuration or environment.
- Doctor output provides deterministic findings and recovery actions.

### 2.3 Weaknesses

- A host may not load `AGENTS.md`.
- An agent can begin editing before running discovery.
- MCP may not be installed or may start in the wrong root.
- Ordinary shell and editing tools can bypass the preferred route.
- Discovery communicates policy but does not universally activate enforcement.

Therefore discovery is currently strong guidance, but not always a compulsory gateway.

## 3. Current guidance and freshness model

The bootstrap handshake binds the agent session to:

- Binary version and commit
- State, context, template, and operation schemas
- Canonical workspace root
- Active spec, status, and revision
- Command-palette digest
- Effective-configuration digest
- Managed-guidance digest
- Policy digest
- Tool contracts
- Current legal next commands
- Trusted harness instructions versus untrusted repository data

`status --guide` calculates legal commands, human-only actions, required artifacts, and gate-derived blockers from current state.

This is a strong design because the agent does not reconstruct the workflow from prose. The remaining problem is optionality: unless the host requires a fresh handshake and guide before mutable work, the model can act using stale or incomplete context.

## 4. Current execution authority

`AuthorityV1` is the main enforcement primitive. It binds:

- Actor and worker IDs
- Spec and task IDs
- Phase and role
- Read-only or write mode
- Allowed and denied tools
- Declared read and write paths
- Network policy
- Sandbox profile
- Baseline revision
- Issue and expiration times
- Policy digest
- Packet digest

Under the production profile, task operations can require the authority packet. MCP can reject missing, expired, mismatched, or out-of-scope authority and can refuse mutable execution when the host does not advertise sandbox support.

### Boundary of this guarantee

Authority governs operations that participate in the protocol. It cannot independently prevent an agent from using an unrestricted host shell or file editor. The host must convert authority into real filesystem, tool, process, and network permissions.

This means `specd` currently has the data model needed for native enforcement, but the final guarantee depends on host integration.

## 5. Current evidence and completion model

This is the strongest part of the relationship.

A task cannot complete merely because the agent says it is finished. Completion requires:

- A verification command defined by the task
- Exit code zero
- Evidence pinned to a resolvable Git HEAD
- Fresh quality evidence for every declared evidence class/check
- Consistency between task Markdown and machine state

There is no completion bypass. A human override may clear an escalation ratchet, but it does not manufacture evidence or complete a task.

The result is an essential distinction:

| Statement | Trust level |
|---|---|
| “I implemented it” | Untrusted narrative |
| “Tests passed” | Trusted only if a matching evidence record exists |
| “Task is complete” | Trusted only after the completion transaction succeeds |

This should be preserved without compromise.

## 6. Cooperation-dependent behavior

The following behavior is not universally enforced today:

- Reading managed guidance before work
- Running a handshake before mutable actions
- Refreshing guidance after state or policy changes
- Loading all required context references
- Staying inside declared paths when ordinary host tools remain available
- Preserving read-only roles outside a sandbox
- Performing exactly one task per invocation
- Avoiding direct changes before acquiring authority
- Reporting verification failures verbatim
- Avoiding unsupported natural-language claims
- Using `specd` rather than an independent todo or workflow system

These should not be addressed by longer prompts. They should be converted into host-checked protocol preconditions.

## 7. Identified inconsistencies

### 7.1 Craftsman decision authority

The craftsman role currently instructs the agent to record a deviation through `specd decision`. However, `decision` is human-only and explicitly denied by task authority.

Recommendation: replace this instruction with a structured decision request. The agent proposes a deviation; a human accepts or rejects it.

### 7.2 Read-only validator terminology

The validator is described as read-only but `verify` writes a harness evidence record. This is read-only with respect to the source workspace, not with respect to all state.

Recommendation: model effects explicitly:

- `workspace-read`
- `workspace-write`
- `harness-evidence-write`
- `harness-state-write`
- `external-write`

### 7.3 Assurance levels are unclear

Default-profile role enforcement is partly conventional, while production MCP execution provides stronger authority and scope checks.

Recommendation: every relevant output should declare:

- `advisory`: guidance only
- `gated`: lifecycle/evidence gates active
- `sandboxed`: host-enforced tool, path, process, and network authority

An agent and operator must never infer production guarantees from default-profile behavior.

## 8. Target architecture: the specd driver session

The agent should not merely call selected `specd` commands. It should operate inside an explicit driver session.

Recommended sequence:

1. Host detects `.specd` before exposing mutable tools.
2. Host opens a driver session for the active spec.
3. `specd` returns a fresh handshake and assurance level.
4. `specd` returns the next legal action or human boundary.
5. For a task, `specd` issues expiring authority from a baseline revision.
6. Host loads required context and returns an acknowledgement receipt.
7. Host converts authority into real tool, filesystem, sandbox, and network restrictions.
8. Agent performs one operation or one atomic task.
9. Host and `specd` compare the complete diff against declared scope.
10. Verification records evidence at the current revision.
11. Completion consumes fresh evidence.
12. The prior action token is invalidated and the session requests the next action.

## 9. Prioritized recommendations

### P0 — Canonical `drive` command

Add a canonical agent entry point:

`specd drive <slug> --json`

It should combine the information currently spread across handshake, guidance, frontier selection, and dispatch:

- Session identity
- Current revision
- Assurance level
- Actor allowed to proceed
- Legal operation envelope
- Human-only action, when required
- Selected task
- Authority packet
- Context-manifest digest
- Deterministic blockers
- Exact recovery operation

Existing granular commands should remain, but adapters should use `drive` by default.

### P0 — Stateful driver sessions

Add session operations such as:

- `specd session open <slug> --driver <host> --json`
- `specd session action <session-id> --json`
- `specd session close <session-id>`

Every mutable operation should require the session ID, expected state revision, handshake digest, authority digest, context receipt, baseline revision, and a single-use operation nonce.

This prevents stale guidance replay after state changes.

### P0 — Host-enforced authority

Provide a standard host contract requiring:

- Mutable tools hidden until bootstrap succeeds
- Workspace writes permitted only for declared paths
- Shell processes executed inside the mission sandbox
- Network access derived from authority
- Human-only tools omitted from the agent surface
- Harness-owned files denied to ordinary editors
- Authority expiration checked at tool invocation time

When a host cannot provide these controls, `specd` must label the session advisory and must not present it as fully governed.

### P0 — Universal diff-scope gate

Before verification and completion, compare the mission baseline with the complete current diff. Reject:

- Undeclared modified files
- Undeclared created, deleted, or renamed files
- Changes that predate the mission baseline
- Direct task-marker or state manipulation
- Changes overlapping another active lease

This must be a core invariant, not only a transport-specific production feature.

### P1 — Required context acknowledgement

Require a host receipt before activating mutable authority. The receipt should bind:

- Manifest digest
- Required items supplied
- Missing items
- Host-reported token count
- Host and driver identity

It cannot prove comprehension, but it proves the required context was supplied. Missing required lanes must block execution.

### P1 — Machine-readable role capabilities

Project each role into a structured capability contract. Role Markdown should explain behavior, but must not define authority.

The contract should identify workspace effects, harness effects, allowed operations, completion authority, human authority, path scope, network policy, and sandbox requirements.

### P1 — Structured decision requests

Add an operation such as:

`specd request-decision <slug> <task> --reason <text> --proposed-change <text> --impact <text>`

It should suspend the task and create a human action without recording approval. Work resumes only after a human decision.

### P1 — Self-locating machine output

Every machine response should include:

- Active spec
- Phase and status
- State revision
- Actor class
- Assurance level
- Authority state
- Legal next operations
- Human-only boundary
- Advisory versus authoritative status

### P1 — Typed blocked protocol

All refusals should use one structured form containing:

- Stable error code
- Exact mismatch or blocker
- Whether authority was consumed
- Whether retry is safe
- Actor required to unblock
- Exact recovery command

This prevents agents from improvising after a failure.

### P2 — Conformance telemetry

Record deterministic protocol events, including:

- Work attempted without bootstrap
- Stale action replay
- Tool attempted without authority
- Undeclared path attempted
- Human-only operation attempted by an agent
- Required context not acknowledged
- Completion claimed before completion
- Direct `.specd` mutation detected

These events should improve adapters and guidance without weakening enforcement.

### P2 — First-class host adapters

Provide maintained adapters for major coding-agent hosts. Each adapter should install repository discovery, mandatory bootstrap, MCP configuration, pre-tool authorization, post-write scope checks, context loading, structured result rendering, and human-approval handoff.

MCP should remain the common protocol, while adapters implement host-specific filesystem and shell enforcement.

## 10. Communication design principles

### 10.1 Prefer one authoritative next action

Do not make the model synthesize a workflow from a long list of commands. Return the smallest legal action set, ideally one recommended operation.

### 10.2 Separate instruction from data

Harness policy, authority, and operation contracts are trusted instructions. Requirements, code, tests, tool output, skills, and memory are untrusted inputs. Every context item should carry this distinction.

### 10.3 Make failures self-unblocking

Every refusal must explain what failed, who owns the next action, and the exact safe recovery route.

### 10.4 Never use prose as authority

Role files, agent summaries, requirements, and skills must never grant tools, expand scope, approve decisions, or manufacture evidence.

### 10.5 Preserve narrative without confusing it with state

Agent reports are useful, but UIs and reports must visibly distinguish narrative claims from harness truth.

Example:

- Agent report: implemented
- Harness state: executing
- Evidence: passed at commit `a1b2c3d`
- Completion record: absent
- Authoritative result: not complete

## 11. Adoption roadmap

### Phase A — Correctness and clarity

- Fix the craftsman/decision contradiction.
- Introduce explicit effect terminology.
- Surface assurance level everywhere.
- Standardize typed refusal and recovery responses.
- Document the exact boundary between default and production enforcement.

### Phase B — Unified driver protocol

- Implement `specd drive`.
- Add session identity, revision pinning, and single-use action nonces.
- Bind context acknowledgement to task authority.
- Make every agent adapter consume the unified envelope.

### Phase C — Host enforcement

- Deliver at least one reference adapter with filesystem and shell mediation.
- Enforce authority-derived paths, tools, sandbox, and network rules.
- Add the universal diff-scope gate.
- Deny direct mutation of harness-owned state.

### Phase D — Multi-agent hardening

- Bind leases and driver sessions.
- Detect overlapping task scopes before dispatch.
- Reject baseline drift and stale mission reports.
- Add structured human decision handoff.
- Record conformance telemetry and use it to improve the workflow.

## 12. Acceptance criteria for a native relationship

The relationship can be considered native when all of the following are true:

1. Opening a managed repository automatically triggers discovery before mutable tools are available.
2. Every mutable operation belongs to a fresh driver session.
3. Every task operation carries valid authority tied to spec, task, phase, role, policy, and baseline.
4. The host enforces tool, path, sandbox, and network restrictions.
5. Required context is acknowledged before authority activation.
6. Human-only operations are unavailable to agents.
7. The entire diff is validated against task scope.
8. Stale revisions, manifests, policy, guidance, and action tokens fail closed.
9. Verification evidence is commit-pinned and completion remains a separate transaction.
10. Natural-language claims never alter or visually override authoritative state.
11. Every blocker identifies the responsible actor and exact recovery action.
12. Multi-agent leases prevent overlapping authority and stale completion.

## Final recommendation

`specd` should evolve from a CLI that disciplined agents can drive into a protocol that coding-agent hosts must enter.

The existing primitives are already unusually strong. The next step is consolidation and host enforcement, not more prompt text. Handshake, guidance, context manifests, authority, sandbox negotiation, diff scope, evidence, and completion should become one continuous driver protocol with no ungoverned gap between them.

If that is achieved, the agent does not need to be perfectly reliable. The system remains reliable even when the agent is not.