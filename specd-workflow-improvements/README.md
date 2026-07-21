# Specd workflow improvement analysis

## Executive summary

Specd's valuable core is intact: deterministic gates, immutable evidence, explicit authority,
atomic state changes, and no LLM in enforcement paths. Practical use fails around that core.
Templates disagree with parsers, `check` omits readiness failures, guidance advertises closed
routes, production orchestration can deadlock, repair work cannot reopen completed tasks, and the
generated agent guide treats repository presence as consent to use Specd for every request.

Target workflow keeps enforcement but makes activation explicit, states recoverable, and each
refusal actionable. Three decisions drive all recommendations:

1. Ordinary coding is the default. Specd-managed work starts only through explicit request,
   session attachment, or a declared repository enforcement rule.
2. Undo and reopen are append-only transitions. History and old evidence remain immutable; new
   attempts and artifact revisions receive new identities and fresh evidence requirements.
3. `pending` remains a narrow task activity state. Eligibility is a separate readiness value, so
   reports distinguish `pending (ready)` from `pending (waiting on T2)`.

Configuration moves to `.specd/config.yaml`. Repository evidence shows the real legacy filename is
`project.yml`, despite the input request naming `project.yaml`; migration therefore supports both
legacy spellings, never loads both silently, and treats the canonical file as configuration only.

## Main findings

- Contracts drift across templates, parsers, gates, help, guidance, dispatch, and tests.
- `specd check` is not the go/no-go command users reasonably infer; readiness failures can exist
  beside a clean exit and empty JSON array.
- Agent control is mostly prose. Human-only operations and role tool restrictions need host-backed
  authority, with advisory assurance disclosed when enforcement is unavailable.
- Current state models initial construction better than repair. Completed tasks, approved artifacts,
  stale review reports, abandoned missions, and cross-cutting fixes lack legal recovery paths.
- Production posture is assembled from individually tested components but lacks one end-to-end
  invariant test that stays in production mode.
- Configuration is visible but costly: strict-subset parsing, inconsistent delimiters, root-level
  placement, and no source/precedence explanation.
- Unattended approval is already possible by convention and unauditable. Safe automation needs
  scoped delegation, not gate skipping.

## Most critical workflow failures

1. **False green:** `check` can exit 0 while approval, task completion, criteria, or review is
   blocked.
2. **Closed legal route:** guidance can route an agent to a command whose authority preconditions
   cannot be satisfied on that transport.
3. **No repair transaction:** post-completion fixes either bypass task history or collide with old
   scope baselines.
4. **Uncontrolled activation:** generated repository instructions do not distinguish ordinary
   coding from Specd-managed work.
5. **Prompt-only authority:** an agent can invoke human-only operations or violate read-only roles
   when the host exposes those tools.
6. **Ambiguous configuration:** current `project.yml` location, YAML subset, environment overrides,
   and prospective `.specd/config.yaml` migration lack one auditable resolution rule.

## Proposed target workflow

1. Router resolves `general`, `consult`, or `managed` from explicit request/session/config inputs and
   reports its decision. Content classification may recommend but never silently activates managed
   mode.
2. Managed intake creates or selects a spec, captures clarification as first-class records, then
   authors requirements, design, and tasks through previewable readiness checks.
3. `specd check --readiness` and approval use the same gate plan. A successful check prints gate,
   revision, config digest, and artifact digests.
4. Execution dispatches only `pending + ready` tasks. Context treats missing declared outputs as
   prospective write targets, while missing declared inputs fail closed.
5. Interactive approval requires operator authority. Unattended approval consumes a scoped,
   expiring delegation grant; all gates still run.
6. Failures expose typed causes and one legal recovery command. Pause, clarification, block, retry,
   cancel, and reopen are distinct.
7. Reopen creates a new lifecycle or task attempt revision, marks downstream records stale, and
   requires fresh evidence. Released or archived work stays immutable and moves to a linked successor.

## Domain map

- [Workflow state management](workflow-state-management.md): normalized lifecycle and transition engine.
- [Undo and reopen](undo-and-reopen.md): compensating undo, revisioned reopen, descendant recovery.
- [Pending and blocked states](pending-and-blocked-states.md): precise task activity/readiness model.
- [Configuration and project discovery](configuration-and-project-discovery.md): canonical path,
  resolution, validation, and repository root rules.
- [Coding-agent routing](coding-agent-routing.md): ordinary-work separation and enforcement modes.
- [Specification authoring](specification-authoring.md): intake, clarification, templates, design and
  task contracts.
- [Task generation and execution](task-generation-and-execution.md): scope, context, frontier,
  verification, repair, and concurrency.
- [Approvals and unattended execution](approvals-and-unattended-execution.md): actor authority,
  delegation, and automation safety.
- [Context management and enforcement](context-management-and-enforcement.md): input/output lanes,
  budget, host controls, and authority packets.
- [Debugging and failure recovery](debugging-and-failure-recovery.md): typed refusals, recovery,
  checkpoints, and repair paths.
- [User experience and steering](user-experience-and-steering.md): steering ownership, guidance,
  inspection, and concise next actions.
- [Migration and backward compatibility](migration-and-backward-compatibility.md): state/config/API
  compatibility and rollout.
- [Testing and observability](testing-and-observability.md): contract parity, production journeys,
  audit events, and metrics.
- [Implementation roadmap](implementation-roadmap.md): dependency graph, rollout slices, and exit gates.

Primary deliverables: [workflow improvement specification](specification.md) and
[dependency-ordered implementation tasks](implementation-tasks.md).

## Recommended priorities

### P0: restore truthful control

- Separate ordinary agent work from managed mode.
- Make check/readiness/approval share one gate plan.
- Enforce human-only metadata at dispatch when actor identity is available.
- Add legal task and spec reopen transactions.
- Fix canonical configuration resolution without dual-file ambiguity.

### P1: remove workflow deadlocks

- Validate complete authority routes before advertising them.
- Make prospective output context legal.
- Align task capability, routing capability, evidence, and template vocabularies.
- Add mission release/retry and single-worktree-safe orchestration behavior.

### P2: improve authoring and diagnosis

- Validate scaffolds against their consumers.
- Add first-class clarification records.
- Surface gate inputs/digests, readiness, review provenance, and exact recovery actions.
- Make steering editable, inspectable, and machine-enforceable where rules are structured.

## Dependency map

```text
transition/event model
├── pending/readiness split
├── undo/reopen transactions
├── approval revocation and delegation
└── history/report projections

config source resolver
├── .specd/config.yaml migration
├── deterministic root discovery
└── routing/enforcement policy

operation route planner
├── agent mode disclosure
├── guidance/dispatch parity
├── context/authority packets
└── unattended orchestration

shared contract parsers
├── scaffold conformance
├── check/readiness parity
└── production journey tests
```

## Suggested implementation phases

1. **Truthful read paths:** readiness plan, config-source report, route disclosure, actionable
   blockers, and regression journeys. Mostly additive and immediately reduces bad decisions.
2. **Canonical configuration and routing:** `.specd/config.yaml`, explicit mode resolver, generated
   guide changes, and migration command.
3. **Versioned transitions:** event schema, task attempts, approval requests, undo/reopen engine,
   artifact revisions, and stale-record projection.
4. **Execution repair:** context lane split, mission release, capability parity, single-worktree
   policy, review restamp, and cross-task remediation.
5. **Delegated unattended flow:** actor enforcement, scoped grants, controller integration, and
   production end-to-end proof.
6. **Removal:** after the published compatibility window, stop reading legacy config names and old
   response shapes.

## Evidence base

Analysis uses all four requested sources:

- [Specd workflow feedback](../WORKFLOW-FEEDBACK.md)
- [Greenfield context analysis](../specd-context-greenfield-debug-analysis.md)
- [Unattended approval analysis](../unattended-approval-analysis.md)
- [AIDO workflow feedback](../AIDO-WORKFLOW-FEEDBACK.md)

Current behavior was checked against `internal/core`, `internal/cmd`, `internal/context`,
`internal/orchestration`, generated templates, and normative docs. Feedback describes observed
failures; repository code decides whether a failure remains current. One important example is
greenfield context: current `SelectRequiredLanes` now skips missing declared outputs, so that
specific defect is fixed, but its input/output distinction remains a required invariant.
