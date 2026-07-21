# Workflow improvement specification

Status: proposed

Scope: Specd CLI, state model, configuration, agent integration, orchestration, docs, and tests

Evidence date: 2026-07-21

## Problem statement

Specd reliably protects evidence at task completion, but practical workflows can become hard or
impossible before that point. Users repeatedly meet mismatched templates and parsers, incomplete
guidance, silent checks, unreachable authority requirements, rigid completed-state handling, and
prompt-only agent constraints. Presence of `.specd/` also causes coding agents to treat unrelated
requests as managed work even when the user never selected that lifecycle.

Result is too much configuration and steering, false confidence from green commands, manual source
inspection to recover from refusals, and changes made outside Specd when repair cannot fit its
forward-only model.

## Background

Current code uses six forward-only statuses (`requirements`, `design`, `tasks`, `executing`,
`verifying`, `complete`) plus `blocked`; task statuses are `pending`, `running`, `complete`, and
`blocked`. `AdvanceStatus` permits only the next status. Current maintenance guidance explicitly
forbids reopening completed specs and requires linked successors. That immutability is correct for
released or archived history but too broad for unreleased repair.

Configuration currently resolves only root `project.yml` plus environment overrides. The requested
rename says `project.yaml`, but repository code, templates, tests, and docs use `project.yml`.
Canonical migration must therefore recognize both spellings and report which source was selected.

Generated `AGENTS.md` documents a mandatory Specd loop but no request router. `HumanOnly` command
metadata shapes guidance, yet direct CLI dispatch does not establish a human actor. Host contracts
correctly admit tool and filesystem enforcement is external.

## Goals

- Make ordinary coding remain ordinary unless managed mode is explicitly selected or enforced.
- Give every lifecycle stage truthful entry conditions, output, failure causes, and recovery.
- Support controlled undo and reopen without deleting or rewriting history.
- Give `pending` one precise meaning and separate it from readiness and waiting conditions.
- Move project configuration to `.specd/config.yaml` without silent precedence ambiguity.
- Make interactive and unattended approval explicit, scoped, auditable, and gate-preserving.
- Keep evidence integrity, deterministic gates, atomic writes, CAS, byte-stable tasks parsing, and
  zero runtime dependencies.
- Make machine and human surfaces derive from shared transition, gate, and operation plans.

## Non-goals

- No flag may bypass a gate, manufacture evidence, or mark a task complete without a current passing
  verify record.
- Specd will not infer user intent from natural-language content and silently activate managed mode.
- Specd will not claim to sandbox tools, paths, or networks when the host does not enforce them.
- Released, deployed, archived, or externally consumed revisions will not be edited in place.
- `.specd/config.yaml` will not store mutable workflow state, secrets, evidence, or approval grants.
- Initial rollout will not implement full YAML; it retains and clearly names the supported subset.
- No global file may weaken project gates, authority, evidence, or security policy.

## User personas

- **Developer:** wants normal edits by default and a clear opt-in for governed changes.
- **Specification author:** needs templates that pass their consumers and fast clarification loops.
- **Operator/approver:** needs exact gate inputs, review scope, delegation controls, and audit history.
- **Coding agent:** needs one mode, one legal next action, bounded authority, and typed recovery.
- **Auditor:** needs immutable prior revisions, current review provenance, deviations, and stale-record
  visibility.
- **Automation owner:** needs deterministic exit codes, resumable runs, and no false success.
- **Maintainer:** needs post-completion repair that preserves the original proof.

## Core use cases

1. Make an unrelated repository edit without any Specd command despite `.specd/` existing.
2. Ask Specd for read-only status or design advice without attaching the request to a lifecycle.
3. Explicitly attach a request to an existing spec or create a new managed spec.
4. Pause for clarification and resume after an answer without calling the item blocked.
5. Reopen an unreleased completed task, widen its approved repair scope, re-verify, and retain the
   original completion record.
6. Reopen an unreleased completed spec at design or execution, invalidate affected descendants, and
   produce a new revision.
7. Reject reopening immutable released history and create a linked successor instead.
8. Run an unattended spec using a pre-authorized, expiring approval delegation while every gate
   remains active.
9. Migrate `project.yml` or `project.yaml` to `.specd/config.yaml` with a preview and backup.
10. Diagnose a failed command from its typed cause, inspected inputs, and one legal recovery action.

## Functional requirements

### FR-1: request routing

- **FR-1.1** The agent integration contract shall resolve each request to `general`, `consult`, or
  `managed` mode before invoking a mutable Specd operation.
- **FR-1.2** Resolution precedence shall be: explicit per-request directive, active session binding,
  repository enforcement rule, project default, compiled default.
- **FR-1.3** Compiled and freshly scaffolded project default shall be `general`.
- **FR-1.4** Natural-language classification may recommend managed mode but shall not activate it.
- **FR-1.5** `consult` shall expose read-only Specd operations only.
- **FR-1.6** `managed` shall bind one spec slug or an intake operation before mutable work.
- **FR-1.7** Every machine envelope shall report mode, source of the decision, enforcement level,
  selected spec, and legal operations.
- **FR-1.8** A repository enforcement rule shall be explicit, path/branch bounded, and host-enforced;
  absent host support shall lower assurance and refuse claims of containment.

### FR-2: shared workflow planning

- **FR-2.1** `check --readiness`, `status --guide`, `approve`, `drive`, and MCP handoff shall consume
  the same deterministic transition plan for a given state/config revision.
- **FR-2.2** The plan shall list armed gates, input paths and digests, target transition, blockers,
  permitted actor, required authority, and recovery operations.
- **FR-2.3** Plain `check` shall state whether readiness was evaluated. `check --readiness` shall exit
  non-zero whenever the planned transition is blocked.
- **FR-2.4** A successful readiness check shall emit a non-empty summary containing slug, source
  status, target status, state revision, config digest, and gate counts.
- **FR-2.5** Guidance shall not advertise an operation whose dispatch preconditions cannot be met on
  the current transport and authority path.

### FR-3: normalized state

- **FR-3.1** Spec state shall separate `stage` from `condition`. Stages are `requirements`, `design`,
  `tasks`, `execution`, `verification`, and `complete`. Conditions are `active`, `waiting_approval`,
  `waiting_clarification`, `paused`, `blocked`, `cancelled`, and `superseded` where valid.
- **FR-3.2** Artifact state shall be versioned independently as `draft`, `in_review`, `approved`,
  `rejected`, or `superseded`.
- **FR-3.3** Task activity shall be `draft`, `pending`, `in_progress`, `paused`, `blocked`, `failed`,
  `completed`, `cancelled`, or `superseded`.
- **FR-3.4** Task readiness shall be separate: `ready`, `waiting_dependency`, `waiting_approval`,
  `waiting_clarification`, or `waiting_schedule`.
- **FR-3.5** Approval requests, execution runs, and clarification requests shall have their own
  closed state sets defined below.
- **FR-3.6** Invalid stage/condition/entity combinations shall fail before mutation.

### FR-4: pending semantics

- **FR-4.1** `pending` shall apply only to an accepted task with no active attempt and no terminal
  disposition.
- **FR-4.2** `pending` shall not mean draft, blocked, paused, deferred, waiting for clarification,
  or waiting for approval; those facts live in distinct activity/readiness/disposition fields.
- **FR-4.3** Every non-ready pending task shall carry deterministic reason code and references. A
  dependency reason may be derived; a manually imposed schedule or approval reason requires text.
- **FR-4.4** Only `pending + ready` tasks enter the frontier.
- **FR-4.5** Any pending task blocks parent completion. Cancellation, deferment, or supersession
  stops blocking only after acceptance coverage is explicitly reassigned or waived by an authorized
  approval.
- **FR-4.6** Reports shall render both activity and readiness, for example `pending (ready)` and
  `pending (waiting_dependency: T2)`.

### FR-5: undo and reopen

- **FR-5.1** Undo and reopen shall be separate operations.
- **FR-5.2** Undo shall compensate only the latest reversible event when it has no consumed child
  event, external effect, passing completion evidence, release, deployment, or archive dependency.
- **FR-5.3** Undo shall append a compensation event; it shall never delete a record or decrement a
  revision.
- **FR-5.4** Reopen shall create a new lifecycle cycle, artifact revision, task attempt, approval
  request, or execution attempt as applicable.
- **FR-5.5** Every reopen requires actor authority, reason, target, expected revision, and impact
  preview. Mutable reopen operations use lock plus CAS.
- **FR-5.6** Reopening a completed task shall preserve prior evidence and completion, increment the
  task attempt, issue a fresh scope baseline, and require fresh evidence before recompletion.
- **FR-5.7** Reopening requirements or design shall mark dependent approvals, task plans, task
  completions, criteria, reviews, missions, and submission records stale according to declared
  dependency edges.
- **FR-5.8** Completed descendants shall become `completed_stale`, a projection requiring explicit
  revalidation, reopen, cancel/supersede, or approved retention; they shall not silently return to
  pending.
- **FR-5.9** Reopening an unreleased completed spec shall start a new spec revision at the selected
  stage. Released, deployed, archived, or externally published revisions shall refuse in-place
  reopen and direct creation of a linked successor.
- **FR-5.10** Rejected or cancelled work may reopen into a new revision/attempt when immutable
  consumption rules allow it. The old rejection/cancellation remains in history.
- **FR-5.11** Approval undo shall be called `revoke`; it shall append a revocation and invalidate
  downstream consumers. Resubmitting rejected work creates a new approval request.

### FR-6: configuration and discovery

- **FR-6.1** Canonical project configuration shall be `.specd/config.yaml`.
- **FR-6.2** Runtime state, workflow metadata, evidence, grants, and histories shall not be stored in
  the config file.
- **FR-6.3** Repository root discovery shall walk upward to the nearest `.specd` directory for all
  non-`init` operations and shall report the resolved root.
- **FR-6.4** During compatibility window, resolver shall recognize legacy root `project.yml` and
  `project.yaml`.
- **FR-6.5** If canonical and legacy files coexist with different normalized values, all governed
  operations shall fail with `CONFIG_CONFLICT` and list differing keys.
- **FR-6.6** If files coexist with equal normalized values, canonical shall load and a deprecation
  warning shall name ignored legacy paths.
- **FR-6.7** `specd config migrate [--dry-run]` shall atomically create canonical config, validate it,
  and rename the legacy source to a timestamp-free `.specd-v1.bak` path. It shall never migrate
  silently during a read command.
- **FR-6.8** Config diagnostics shall identify source path, line, key, expected grammar, and recovery.
- **FR-6.9** Config JSON projections and handshakes shall include source path and digest.
- **FR-6.10** Global configuration shall not control gates, authority, evidence, security, or
  enforcement. Initial release shall omit global workflow config; future global presentation
  defaults must remain lower precedence and non-governing.
- **FR-6.11** Supported syntax shall accept `.yaml` and `.yml`, quoted scalars, two-space mappings,
  and unquoted trailing comments consistently; unsupported YAML features shall fail explicitly.

### FR-7: clarification and authoring

- **FR-7.1** Clarification requests shall be first-class records with id, question, requester,
  owner, affected artifact/task, created time, state, answer, answer actor, and resolved time.
- **FR-7.2** Clarification states shall be `open`, `answered`, `withdrawn`, or `expired`.
- **FR-7.3** An open blocking clarification shall set affected work to
  `waiting_clarification`, not `blocked` or generic pending.
- **FR-7.4** Every scaffolded artifact shall pass its parser and structural gates after placeholders
  are replaced as instructed.
- **FR-7.5** Shared parsers shall define evidence, capabilities, review verdict, and configuration
  list grammar; independent consumers shall not split those fields differently.
- **FR-7.6** Task approval shall validate declared output paths, required input paths, verify/test
  reachability, evidence producer compatibility, and routing capability compatibility.
- **FR-7.7** Approved artifact edits shall create an amendment/revision and shall make affected
  approvals visibly stale.

### FR-8: execution and evidence

- **FR-8.1** Context construction shall keep required readable inputs separate from authorized
  outputs. Missing outputs remain authorized prospective paths; missing required inputs fail.
- **FR-8.2** Verification that executes zero selected tests shall not produce passing task evidence.
- **FR-8.3** Evidence failures shall distinguish missing, failing, stale, malformed, and
  producer-incompatible records, each with a non-bypass recovery.
- **FR-8.4** A repair attempt may use an approved scope amendment spanning original task boundaries;
  old task rows and scopes remain preserved in prior plan revisions.
- **FR-8.5** Parallel missions shall require declared isolation. Without isolated worktrees, the
  controller shall serialize tasks rather than dispatch an uncompletable wave.
- **FR-8.6** Operators shall be able to release or cancel one unclaimed mission and retry it at a
  fresh baseline without cancelling the whole session or waiting for TTL.
- **FR-8.7** Completed tasks shall be excluded from pre-execution context-budget blockers unless
  they are selected for reopen or revalidation.
- **FR-8.8** Review provenance shall use the evidence subject revision; restamping shall preserve the
  report body. Scaffold commands shall never overwrite an existing report without explicit force.

### FR-9: approvals and unattended operation

- **FR-9.1** Dispatcher shall enforce operation actor metadata when the transport supplies an actor
  class. Agent actor invoking operator-only operation without delegation shall fail.
- **FR-9.2** Lack of host actor enforcement shall be disclosed as advisory; CLI shall not pretend a
  TTY or username proves humanity.
- **FR-9.3** Unattended approval shall use a scoped delegation grant, not `--force`, `--skip-gates`,
  environment identity alone, or controller self-approval.
- **FR-9.4** A grant shall bind project, allowed specs/patterns, allowed transitions, issuer, actor,
  issue/expiry times, maximum uses, production allowance, reason requirement, and policy digest.
- **FR-9.5** Every delegated approval shall consume a valid grant use, run the normal readiness plan,
  and record grant id, actor class, source/config digests, gate result digest, and reason.
- **FR-9.6** Delegation shall not authorize evidence fabrication, security exceptions, release,
  deployment, archive, or config-policy weakening unless separately and explicitly supported.
- **FR-9.7** `brain` may request or consume delegated approval but shall not mint its own grant.
- **FR-9.8** Revocation shall prevent future uses without rewriting prior delegated approvals.

### FR-10: failures and observability

- **FR-10.1** Every refusal shall have stable code, subject, observed state, expected state,
  inspected inputs, retryability, responsible actor, and legal recovery operation when one exists.
- **FR-10.2** Blocked controller halt before any progress shall exit non-zero and machine output
  shall distinguish `complete`, `waiting`, `blocked`, and `failed`.
- **FR-10.3** Commands with write effects shall declare them in operation metadata; unknown flags
  shall fail closed before mutation.
- **FR-10.4** State-changing commands shall append audit events containing event id, entity id,
  previous/new version, actor, authority/delegation, reason, timestamp, git head, config digest, and
  source digests.
- **FR-10.5** History shall project events in stable order and expose supersession, compensation,
  stale descendants, and current effective state.

## Non-functional requirements

- **NFR-1 Determinism:** state, routing, transition, gate, report, and recovery decisions are pure
  functions of explicit inputs and on-disk state.
- **NFR-2 Integrity:** no reopened attempt reuses old passing evidence as current proof.
- **NFR-3 Atomicity:** multi-entity transitions use per-spec lock, CAS, and recoverable write-ahead
  records; partial mutations fail closed.
- **NFR-4 Compatibility:** schema and response changes follow version policy; additive projections
  precede removals.
- **NFR-5 Security:** authority is least-privilege, time-bounded, actor-aware where hosts support it,
  and honest about advisory enforcement.
- **NFR-6 Offline operation:** all core workflows remain local and stdlib-only.
- **NFR-7 Explainability:** users can identify why a transition passed or failed without reading Go
  source.
- **NFR-8 Performance:** readiness planning remains linear in task, record, and gate counts within
  documented scale limits.

## Workflow rules

1. General mode performs requested repository work under normal repository instructions and never
   creates or mutates Specd records.
2. Consultation may read config, specs, status, check plans, and reports but cannot attach authority
   or mutate lifecycle state.
3. Managed mode requires selected spec/intake and follows the transition plan.
4. Artifact review and approval are separate: authors submit, approvers approve/reject, rejection
   returns a new draft revision.
5. Execution selects only ready tasks, issues one attempt authority, verifies, then completes.
6. Pause is operator intent; blocked is an objective failed condition; clarification is a question;
   failed is a concluded attempt. None are aliases.
7. Completion requires every acceptance obligation to be completed, explicitly reassigned, or
   governed by an approved disposition.
8. Repair never changes old proof. It creates a new attempt or successor.

## State-transition rules

### Specification

```text
active authoring -> waiting_approval -> next stage active
active <-> paused
active -> waiting_clarification -> active
active -> blocked -> active
active|waiting_* -> cancelled -> reopened(new cycle) | superseded
verification/waiting_approval -> complete
complete(unreleased) -> reopened(new cycle at selected stage)
complete(released/archived) -> superseded by linked successor only
```

Invalid: skip stages without transition plan; complete with blocking tasks; direct mutation of
complete; reopen immutable consumption; move backward without reopen event.

### Task

```text
draft -> pending
pending + ready -> in_progress
pending|in_progress -> paused -> pending
pending|in_progress -> waiting_clarification -> pending
in_progress -> blocked -> pending(new attempt or cleared condition)
in_progress -> failed -> pending(new attempt) | cancelled
in_progress + passing current evidence -> completed
completed -> completed_stale -> revalidated | pending(new attempt) | superseded
pending|failed|blocked -> cancelled -> pending(new attempt, authorized reopen)
```

Invalid: completed without evidence; old attempt evidence satisfies new attempt; pending enters
frontier when readiness is not ready; child completion remains current after invalidating ancestor.

### Approval request

```text
draft -> requested -> approved | rejected | withdrawn | expired
approved -> revoked | superseded
rejected -> new requested revision
revoked -> new requested revision
```

Invalid: edit approved record; approve stale request; delegated approval outside grant.

### Execution run

```text
created -> queued -> running -> waiting | paused | failed | cancelled | completed
waiting -> running | failed | cancelled
paused -> running | cancelled
failed -> queued(new attempt)
```

### Clarification request

```text
open -> answered | withdrawn | expired
answered -> superseded by a new clarification only
```

## Agent-interaction rules

- Agent states resolved mode before mutable work and when mode changes.
- Agent does not infer managed mode from `.specd` presence, issue wording, or available commands.
- In general mode, agent may mention Specd only as an optional route when materially useful.
- In consultation mode, agent cannot edit artifacts or call state-changing Specd verbs.
- In managed mode, agent follows machine legal operations, one task authority, and declared files.
- Agent stops on authority/config digest mismatch; it never works around by editing ledgers.
- Agent answers open clarification only when user or authorized source provides the missing fact.
- Reopened work loads current attempt, stale descendants, original reason, scope changes, and fresh
  verification obligations.
- Agent never invokes operator-only operations without a valid delegated operation surfaced by the
  host.

## Configuration changes

Canonical file is `.specd/config.yaml`; it contains only stable project policy/configuration.
Recommended additions:

```yaml
schema_version: 2
agent:
  routing_default: general
  managed_prefix: specd:
enforcement:
  paths: ""
approval:
  delegation: off
```

Empty enforcement paths mean no ordinary request is forced into Specd. Runtime grants live under
`.specd/authority/` or external host secret storage, never in config or git by default.

## Migration requirements

- Detect canonical and both legacy spellings before loading.
- Preview normalized changes and unsupported keys.
- Back up legacy source on explicit migration.
- Preserve effective values and emit before/after digests.
- Keep legacy read support for at least two minor releases, warning once per invocation in text and
  through structured diagnostics in JSON.
- Provide upgrade tests for absent config, each legacy spelling, canonical only, equal duplicates,
  conflicting duplicates, malformed input, env overrides, and nested working directory.
- Migrate state v1 through an explicit pure upgrader; preserve old bytes until new state passes
  validation, then atomic replace with backup.

## Error-handling requirements

- Errors distinguish usage, policy refusal, gate failure, transient conflict, failed verify, and
  internal fault through stable codes and exit classes.
- No error suggests deleting evidence declarations, weakening policy, or changing profiles as the
  only recovery.
- Recovery names exact actor and command, or explicitly states that no in-place recovery exists.
- Mutating failure reports whether durable state changed and which recovery record exists.

## Auditability requirements

- Effective state must be reproducible from append-only events plus immutable artifact/evidence
  content.
- Every state projection carries schema, entity revision, lifecycle cycle, config digest, and last
  event id.
- Reopen history links old and new revisions, reason, scope impact, stale descendants, and actor.
- Delegated approvals are visibly distinct from interactive approvals in status, report, and
  history.
- Gate plans record inputs and result digest so pass/fail changes are attributable.

## Compatibility requirements

- Existing state v1 remains readable throughout migration window.
- Existing `pending` tasks map to activity `pending`; readiness is derived from dependencies and
  approvals.
- Existing complete specs remain immutable until explicitly reopened under v2 and only when no
  release/archive consumption exists.
- Existing untyped actor records remain `actor_class: unknown`, never inferred human.
- Existing `project.yml` values remain effective during deprecation when no canonical conflict
  exists.
- Machine response shape changes use versioned envelopes or additive fields before removal.

## Security and safety considerations

- Human identity is not provable from username, environment variable, or TTY alone. Assurance must
  reflect host controls.
- Delegation tokens are secrets: store outside tracked files, hash identifiers in ledgers, bound
  scope/expiry/use, and compare in constant time where applicable.
- Reopen must not let users discard failing evidence, expand scope without authority, or rewrite
  released history.
- Config conflict fails closed because selecting the weaker file silently is a policy bypass.
- General mode must not become a bypass for explicit enforcement rules; enforcement requires host
  path control and must be visible before edits.

## Acceptance criteria

1. In a repo containing `.specd`, a plain coding request executes in general mode without a Specd
   command or lifecycle artifact mutation.
2. Explicit managed directive binds a slug and emits mode source and legal operations.
3. `check --readiness` and `approve` return identical blocker sets at one revision.
4. A completed task can reopen into attempt 2, old evidence remains queryable, and attempt 2 cannot
   complete until fresh current-HEAD evidence exists.
5. Reopening a parent task marks completed descendants stale and prevents spec completion until each
   is resolved.
6. Reopening unreleased complete spec creates a new cycle; reopening archived/released spec refuses
   with successor command.
7. Reports distinguish pending-ready from pending-waiting and neither from paused/blocked.
8. Canonical config loads from nested directories; conflicting legacy config fails with differing
   keys.
9. Config migration dry-run writes nothing; real migration validates canonical output and preserves
   a backup.
10. Agent actor cannot call operator-only approval through governed transport without valid grant.
11. Delegated approval runs all gates and records grant identity; failing gate remains a refusal.
12. Missing output files do not block context, while missing required context inputs do.
13. Zero-test verify cannot become passing evidence.
14. Controller never dispatches parallel single-worktree missions that scope enforcement makes
   mutually uncompletable.
15. Production end-to-end test never changes profile and reaches completed state or fails at exact
   expected policy boundary.
16. Every scaffold/consumer conformance test passes.

## Open questions

- Which release/deployment/submission records make a spec revision externally immutable?
- Should a completed-but-unsubmitted spec reopen in place by default, or require `--in-place`?
- Does approved descendant retention require fresh verify only, or a human impact approval too?
- Which host mechanism supplies trustworthy actor class and delegated token on CLI-only use?
- Should general-mode enforcement path rules ship initially or wait until at least one host can
  enforce them?
- Can artifact history use Git blobs exclusively, or must Specd store content-addressed local
  snapshots for uncommitted authoring revisions?
- What compatibility duration applies to bare-array `check --json` consumers?

## Risks

- State model expansion can create another vocabulary mismatch unless one transition package owns
  parsing, validation, projection, and docs generation.
- In-place reopen may confuse immutable maintenance semantics; external-consumption guard must land
  first.
- Delegation may be mistaken for identity proof. UI and docs must label advisory transports.
- Supporting two config names during migration increases ambiguity; strict conflict refusal is
  mandatory.
- Event ledger plus state projection adds crash-recovery complexity; write-ahead tests must precede
  broad mutation migration.
- Explicit routing can be bypassed by hosts ignoring it; assurance disclosure and host conformance
  remain necessary.

## Recommended implementation sequence

1. Add shared read-only transition/gate/route planning and truthful outputs.
2. Add config source resolver, canonical config, migration command, and generated-guide routing.
3. Add event schema and v1-to-v2 state projection without enabling reopen.
4. Add pending/readiness projection and first-class clarification/approval request records.
5. Add task reopen, attempt evidence binding, stale-descendant handling, and repair scope amendments.
6. Add spec/artifact reopen with immutable-consumption guard and successor fallback.
7. Repair execution context, review, mission, capability, and concurrency paths.
8. Add host-enforced actor checks and scoped delegated approval.
9. Prove default and production journeys end to end, then remove legacy behavior after deprecation.
