# Dependency-ordered implementation tasks

Each task is independently reviewable. File names are likely scope, not an approved task table.
Every task preserves zero runtime dependencies and the evidence/CAS/atomic-write invariants.

## Phase 0: characterize and pin current behavior

### T01 — Add evidence-to-regression index

- **Purpose:** Turn each observed workflow failure into a named current, fixed, or superseded case.
- **Scope:** Table-driven inventory linking feedback headings to tests/issues; no runtime behavior.
- **Dependencies:** None.
- **Likely components:** `internal/cmd/*_test.go`, `internal/core/*_test.go`, `scripts/regress-domains.sh`.
- **Guidance:** Reproduce current code, not historical binaries; mark greenfield missing-output case
  fixed and retain it as invariant coverage.
- **Edge cases:** Historical issue already fixed; feedback correction supersedes earlier attribution.
- **Testing:** Index lint rejects evidence entry without a test, explicit deferral, or resolved note.
- **Acceptance:** Every feedback heading has one disposition and owner.
- **Migration:** None.
- **Docs:** Link index from contributor guide.

### T02 — Add full default and production workflow journeys

- **Purpose:** Catch integration deadlocks hidden by component tests.
- **Scope:** Fresh init through completion using installed-style binary; production test never changes
  profile.
- **Dependencies:** T01.
- **Likely components:** `internal/cmd/e2e_test.go`, `scripts/production-smoke.sh`, fixtures.
- **Guidance:** Assert exit codes, state, gate plan, authority route, task evidence, review, and history.
- **Edge cases:** No telemetry producer, no host sandbox, one and two tasks, failed verify.
- **Testing:** Tests are the deliverable; run twice for ordering flakiness.
- **Acceptance:** Default journey completes; production journey either completes under declared host
  capabilities or refuses before work with exact supported recovery.
- **Migration:** Existing smoke script remains until replacement passes.
- **Docs:** Testing guide names postures and guarantees.

## Phase 1: truthful read paths

### T03 — Introduce deterministic transition plan

- **Purpose:** Give check, approval, guidance, drive, and MCP one source of truth.
- **Scope:** Pure `TransitionPlan` builder with target, actor, authority, gates, inputs, blockers,
  recoveries, and digests.
- **Dependencies:** T02.
- **Likely components:** `internal/core/transition.go`, `internal/core/gates`, `internal/cmd/registry.go`.
- **Guidance:** Move decision logic, not filesystem reads, into core; caller assembles explicit input.
- **Edge cases:** Complete state, stale approvals, missing artifact, production authority unavailable.
- **Testing:** Matrix of stage, profile, mode, actor, and transport.
- **Acceptance:** Same inputs produce byte-stable plan; no mutation occurs.
- **Migration:** Additive structure.
- **Docs:** Document plan schema and gate/input digest semantics.

### T04 — Make check and approve consume the same plan

- **Purpose:** Eliminate false-green validation.
- **Scope:** `check --readiness`, success summary, JSON envelope, approval plan consumption.
- **Dependencies:** T03.
- **Likely components:** `internal/cmd/check.go`, `lifecycle.go`, `status.go`, command metadata.
- **Guidance:** Preserve plain check compatibility but explicitly report `readiness_checked`; version
  JSON envelope rather than silently changing bare array.
- **Edge cases:** Warnings only, schema-only, security-only, no successor from complete.
- **Testing:** Blocker-set equality between readiness check and immediate approve.
- **Acceptance:** Approval cannot reveal a blocker absent from same-revision readiness output.
- **Migration:** Deprecate bare-array JSON with compatibility flag/window.
- **Docs:** Regenerate command reference; update user guide.

### T05 — Validate guidance against dispatch routes

- **Purpose:** Stop advertising commands that current transport cannot execute.
- **Scope:** Operation-route planner and parity validation for status, drive, handshake, MCP.
- **Dependencies:** T03.
- **Likely components:** `internal/core/commands.go`, `internal/cmd/dispatch.go`, `drive.go`,
  `handshake.go`, `internal/mcp`.
- **Guidance:** Planner returns missing issuer/control as blocker, not a nominal legal command.
- **Edge cases:** Production mission packet, open driver session, stale lease, human handoff.
- **Testing:** Every advertised operation survives dispatcher preconditions in a state matrix.
- **Acceptance:** No agent-facing next action ends in structurally unavoidable authority denial.
- **Migration:** Additive machine fields; keep human-only handoff separate.
- **Docs:** Host and MCP guides explain route completeness.

### T06 — Standardize typed refusals and recoveries

- **Purpose:** Make failures actionable without source reading.
- **Scope:** Refusal fields, exit classes, inspected inputs, mutation status, recovery actor/command.
- **Dependencies:** T03.
- **Likely components:** `internal/core/refusal.go`, command handlers, MCP adapter.
- **Guidance:** Convert highest-cost messages first: evidence policy, authority, scope, telemetry,
  config, review, evidence missing/failing.
- **Edge cases:** No legal in-place recovery; partially written checkpoint; consumed authority.
- **Testing:** Structural lint requires required fields for governed refusals.
- **Acceptance:** Targeted feedback reproductions return stable code and legal recovery.
- **Migration:** Keep old text as detail where clients depend on it.
- **Docs:** Troubleshooting generated from refusal registry.

## Phase 2: configuration and request routing

### T07 — Add canonical config source resolver

- **Purpose:** Resolve `.specd/config.yaml` and legacy names without ambiguity.
- **Scope:** Source discovery, normalization, precedence, conflict diagnostics, source digest.
- **Dependencies:** T04.
- **Likely components:** `internal/core/config_loader.go`, `paths.go`, `doctor.go`, command config helper.
- **Guidance:** Recognize `project.yml` and `project.yaml`; canonical wins only when normalized values
  match, otherwise fail.
- **Edge cases:** Nested working directory, symlink root, unreadable file, equal duplicates,
  conflicting env override.
- **Testing:** Table covering every source combination and extension.
- **Acceptance:** Every config load reports one selected source/digest or typed conflict.
- **Migration:** Legacy reads warn for at least two minor releases.
- **Docs:** Configuration resolution page and deprecation schedule.

### T08 — Scaffold `.specd/config.yaml`

- **Purpose:** Move operator policy beside other Specd project assets.
- **Scope:** Embedded template rename, init/repair preview, doctor, examples, fixtures.
- **Dependencies:** T07.
- **Likely components:** `internal/core/embed_templates`, `scaffold.go`, `internal/cmd/init.go`, tests.
- **Guidance:** Config stays operator-owned and outside managed clobber regions.
- **Edge cases:** Existing `.specd` without config; existing legacy file; dry-run.
- **Testing:** Fresh scaffold parses; init never overwrites existing config.
- **Acceptance:** New project creates only canonical config and all docs/tests use it.
- **Migration:** Do not auto-write over legacy project during ordinary init.
- **Docs:** Regenerate references and snippets.

### T09 — Implement explicit config migration

- **Purpose:** Provide safe automated relocation.
- **Scope:** `specd config migrate [--dry-run]`, validation, atomic write, backup, result report.
- **Dependencies:** T07, T08.
- **Likely components:** new command handler, config core, command palette, tests.
- **Guidance:** Write canonical temp, validate effective equivalence, atomic rename, then rename source
  to `.specd-v1.bak`; report both digests.
- **Edge cases:** Existing backup, multiple legacy files, malformed source, interrupted migration.
- **Testing:** Fault injection around each filesystem step and idempotent replay.
- **Acceptance:** Dry-run writes nothing; migration loses no effective value and leaves recoverable
  source.
- **Migration:** This task is migration mechanism.
- **Docs:** Upgrade guide and warning lifecycle.

### T10 — Add request mode resolver

- **Purpose:** Separate ordinary coding from Specd-managed work.
- **Scope:** Pure `general|consult|managed` resolver and enforcement attribute.
- **Dependencies:** T07.
- **Likely components:** core routing package, handshake/drive envelopes, generated `AGENTS.md`.
- **Guidance:** Inputs are explicit directive, session, enforcement rule, project default; no natural
  language activation.
- **Edge cases:** Conflicting directive and mandatory enforcement; stale session; no slug.
- **Testing:** Precedence matrix; default is general with `.specd` present.
- **Acceptance:** Router always reports mode and source; general mode exposes no mutable Specd route.
- **Migration:** Existing managed sessions remain attached; new sessions default general.
- **Docs:** Agent integration and host contract.

### T11 — Update generated agent guide and host adapters

- **Purpose:** Make routing behavior available where agent action starts.
- **Scope:** General/consult/managed directives, disclosure line, host capability mapping.
- **Dependencies:** T10.
- **Likely components:** embedded `AGENTS.md`, integration snippets, MCP initialize/handshake.
- **Guidance:** Keep managed task loop unchanged once selected; do not burden general mode with
  bootstrap.
- **Edge cases:** Host cannot hide operator tools; explicit user switch mid-session.
- **Testing:** Conformance asserts ordinary request path contains no Specd command and managed path
  contains bootstrap.
- **Acceptance:** Repository presence alone never activates managed workflow.
- **Migration:** Managed-region repair updates only generated block.
- **Docs:** Examples for switching and consultation.

## Phase 3: versioned state and pending semantics

### T12 — Add append-only workflow event schema

- **Purpose:** Preserve undo/reopen history and make state transitions replayable.
- **Scope:** Event identity, entity version, actor/authority, reason, digests, append/replay.
- **Dependencies:** T03, T07.
- **Likely components:** `internal/core/events.go`, state loading/saving, history report.
- **Guidance:** State remains current projection; event append and projection CAS form one recoverable
  transaction.
- **Edge cases:** Torn final line, duplicate event id, future schema, checkpoint outruns projection.
- **Testing:** Crash injection, idempotent replay, deterministic order.
- **Acceptance:** Projection rebuilt from events equals saved state byte-for-field.
- **Migration:** v1 states synthesize one baseline event without changing semantics.
- **Docs:** Open spec format and version policy.

### T13 — Split spec stage and condition

- **Purpose:** Replace overloaded linear status/blocked state with valid orthogonal combinations.
- **Scope:** v2 state fields, validation, legacy projection, reports.
- **Dependencies:** T12.
- **Likely components:** `internal/core/state.go`, `phases.go`, report/status, gates.
- **Guidance:** Keep old status as compatibility projection during window; one validator owns allowed
  combinations.
- **Edge cases:** Legacy blocked state without prior stage; complete plus paused invalid.
- **Testing:** Transition matrix and v1 upgrade fixtures.
- **Acceptance:** Invalid combinations cannot save; all legacy states map or produce repair diagnostic.
- **Migration:** Backup then atomic v1-to-v2 upgrade.
- **Docs:** Concepts lifecycle diagram.

### T14 — Split task activity from readiness

- **Purpose:** Give `pending` precise meaning and expose why work is not runnable.
- **Scope:** Task activity/readiness projection, frontier, reports, guide.
- **Dependencies:** T12, T13.
- **Likely components:** tasks parser/state, DAG/frontier, status/report, gates.
- **Guidance:** Preserve byte-stable markers; readiness is derived from dependencies, approvals,
  clarification, and schedule records.
- **Edge cases:** Completed stale ancestor, cancelled dependency, manually paused pending task.
- **Testing:** All activity/readiness combinations and parent-completion blocking.
- **Acceptance:** Only pending-ready tasks run; every non-ready item has reason/ref.
- **Migration:** Current pending maps directly; readiness derives without artifact rewrite.
- **Docs:** Pending model and status output examples.

### T15 — Add clarification request records

- **Purpose:** Replace ad hoc decision text and generic blocking with answerable workflow objects.
- **Scope:** Open/answer/withdraw/expire commands, affected entities, guide projection.
- **Dependencies:** T12, T14.
- **Likely components:** core clarification ledger, command palette/handlers, status/context.
- **Guidance:** Answer creates immutable resolution; changed question creates new record.
- **Edge cases:** Multiple open questions, non-blocking question, stale answer after artifact revision.
- **Testing:** State transitions, authority, context inclusion, readiness restoration.
- **Acceptance:** Blocking clarification removes item from frontier and answer restores eligibility when
  no other blocker exists.
- **Migration:** Existing request-decision records remain history, not auto-converted.
- **Docs:** Author and agent workflows.

### T16 — Add approval request lifecycle

- **Purpose:** Separate submission, approval, rejection, revocation, expiry, and supersession.
- **Scope:** Versioned approval requests and current effective approval projection.
- **Dependencies:** T12, T13.
- **Likely components:** lifecycle, state records, gates, status/history.
- **Guidance:** Existing `approve` may create-and-approve a request interactively for compatibility;
  machine routes should expose request identity.
- **Edge cases:** Stale artifact after request, duplicate approval, revocation after downstream use.
- **Testing:** Closed transition matrix and stale digest rejection.
- **Acceptance:** No approved record is edited; downstream invalidation is explicit.
- **Migration:** Existing approvals become approved requests with unknown actor class.
- **Docs:** Approval state model.

## Phase 4: undo, reopen, and repair

### T17 — Implement impact preview engine

- **Purpose:** Show all records affected before undo/reopen.
- **Scope:** Dependency graph across artifacts, approvals, tasks, criteria, reviews, missions,
  submissions, releases, deployments, and archives.
- **Dependencies:** T12-T16.
- **Likely components:** new core impact package, transition plan, status/report.
- **Guidance:** Pure graph traversal; include immutable-consumption reason.
- **Edge cases:** Cross-spec links, completed descendants, cyclic malformed legacy data.
- **Testing:** Golden previews for each reopen target.
- **Acceptance:** Preview deterministically lists current, stale, reopened, retained, and forbidden
  entities.
- **Migration:** Unknown legacy dependencies resolve conservatively to stale.
- **Docs:** Impact semantics.

### T18 — Implement narrow undo compensation

- **Purpose:** Reverse accidental unconsumed transitions without pretending history vanished.
- **Scope:** Undo latest reversible event, compensation event, eligibility checks.
- **Dependencies:** T17.
- **Likely components:** core transition engine, command handler, history.
- **Guidance:** No arbitrary event selection initially; latest event only is smaller and safer.
- **Edge cases:** Child event consumed, external effect, completion evidence, release/archive.
- **Testing:** Allowed and forbidden event classes; atomic failure.
- **Acceptance:** Undo never deletes/decrements and effective state matches previewed prior state.
- **Migration:** None beyond event model.
- **Docs:** Exact undo eligibility.

### T19 — Implement task reopen and attempt binding

- **Purpose:** Bring post-completion fixes back inside evidence-gated work.
- **Scope:** New attempt, fresh baseline, optional approved scope amendment, old evidence retention.
- **Dependencies:** T17.
- **Likely components:** task command, evidence schema, scope gate, frontier, report/history.
- **Guidance:** Evidence key includes task id and attempt; no reuse of attempt 1 pass.
- **Edge cases:** Cross-task repair, failed/cancelled task, open lease, completed descendants.
- **Testing:** Attempt 2 cannot complete on attempt 1 evidence; scope amendment audit.
- **Acceptance:** Reopened task is pending with correct readiness and all impacted descendants stale.
- **Migration:** Existing evidence maps to attempt 1.
- **Docs:** Maintenance repair example.

### T20 — Implement artifact and spec reopen

- **Purpose:** Recover unreleased work while preserving immutable consumed revisions.
- **Scope:** Requirements/design/tasks/spec cycle reopen, artifact revisions, downstream staleness.
- **Dependencies:** T17, T19.
- **Likely components:** lifecycle commands, artifact storage, approvals, link/program model.
- **Guidance:** Content-addressed snapshots or Git-pinned blobs; decide open question before coding.
- **Edge cases:** Complete but submitted, released, deployed, archived, rejected, cancelled.
- **Testing:** In-place eligible cases and successor-only immutable cases.
- **Acceptance:** New cycle has new ids/digests; old cycle remains fully reportable.
- **Migration:** Current complete specs are attempt/cycle 1.
- **Docs:** Replace successor-only rule with precise boundary.

### T21 — Add completed-descendant revalidation

- **Purpose:** Resolve stale work without blindly reopening every descendant.
- **Scope:** Revalidate with fresh evidence, explicit reopen, approved retain, supersede/cancel.
- **Dependencies:** T19, T20.
- **Likely components:** evidence, criteria, gates, transition engine.
- **Guidance:** Retain requires impact approval and fresh evidence where behavior could change.
- **Edge cases:** Read-only task, unchanged file digest, criterion reassigned.
- **Testing:** Each resolution and parent completion gate.
- **Acceptance:** No completed-stale item silently becomes current.
- **Migration:** None.
- **Docs:** Impact resolution decision tree.

## Phase 5: execution-path repairs

### T22 — Consolidate task field parsers and scaffold contracts

- **Purpose:** Remove contradictory evidence/capability/review/template grammars.
- **Scope:** One parser per task field, schema introspection, scaffold conformance.
- **Dependencies:** T04.
- **Likely components:** tasks parser, evidence policy, quality contract, routing, templates, review.
- **Guidance:** Consumers use parsed types only; lint direct splitting of typed fields.
- **Edge cases:** Legacy delimiters, qualified review notes, deferred task.
- **Testing:** Every scaffold filled with canonical examples passes every armed consumer.
- **Acceptance:** No valid value has empty intersection across gates.
- **Migration:** Parse legacy values with warnings where unambiguous.
- **Docs:** Generated grammar reference.

### T23 — Harden context lane semantics

- **Purpose:** Preserve current greenfield fix and clarify directory/budget behavior.
- **Scope:** Required input, optional existing output, prospective output, directory query lanes.
- **Dependencies:** T14, T22.
- **Likely components:** `internal/context`, context gate, task schema.
- **Guidance:** Directories need explicit bounded selector or authoring-time refusal; never swallow
  `EISDIR` as missing.
- **Edge cases:** Symlink escape, unreadable input, mixed outputs, reopened task, completed task.
- **Testing:** Cases from greenfield analysis plus directory and terminal-task cases.
- **Acceptance:** Missing output succeeds; missing input fails precisely; completed tasks do not block.
- **Migration:** Existing `files` and `context` columns retain meanings.
- **Docs:** Context manifest schema.

### T24 — Strengthen verify evidence semantics

- **Purpose:** Prevent zero-test passes and misleading evidence recovery.
- **Scope:** Output inspection, attempt binding, missing/failing/stale distinctions.
- **Dependencies:** T19, T22.
- **Likely components:** verify executor, evidence loader, task completion, refusals.
- **Guidance:** Detect Go `[no tests to run]`; define extensible producer status without claiming
  generic semantic test counts for unknown commands.
- **Edge cases:** Legitimate package with some matching and some empty; external eval failure.
- **Testing:** Named-test absence, stale head, failing imported review, malformed envelope.
- **Acceptance:** No failing or zero-test evidence is reported as missing or passing.
- **Migration:** Existing records stay readable; new attempt field defaults to 1.
- **Docs:** Evidence status taxonomy.

### T25 — Repair mission lifecycle and concurrency

- **Purpose:** Make orchestrated execution claimable, releasable, and completable.
- **Scope:** Claim bootstrap, authority packet return, report binding, per-mission release, baseline
  selection, single-worktree serialization.
- **Dependencies:** T05, T14, T19.
- **Likely components:** `internal/orchestration`, brain handlers, diff scope, host contract.
- **Guidance:** Prefer serialization absent declared isolation; simplest correct behavior.
- **Edge cases:** Expired unclaimed mission, duplicate checkpoint, live claimed vs abandoned mission,
  open driver session.
- **Testing:** Dispatch-claim-verify-report journey and crash recovery.
- **Acceptance:** Documented orchestration path completes tasks without skipping claim/report or TTL
  waits.
- **Migration:** Reconcile old sessions; ambiguous sessions require cancel/restart.
- **Docs:** Brain state and isolation requirements.

### T26 — Preserve and restamp review reports

- **Purpose:** Stop destructive scaffolding and stale prose provenance.
- **Scope:** Refuse overwrite, `--restamp`, verdict note parsing, evidence revision authority.
- **Dependencies:** T22, T24.
- **Likely components:** review command/core, review gate, status JSON.
- **Guidance:** Evidence subject revision is normative; prose header is projection.
- **Edge cases:** Unresolved HEAD, multiple passes, existing stale report.
- **Testing:** Body byte-preserved on restamp; no overwrite without force.
- **Acceptance:** Review can advance to current HEAD without report loss.
- **Migration:** Existing reports parse unchanged.
- **Docs:** Auditor workflow.

## Phase 6: actor enforcement and unattended approval

### T27 — Enforce actor class at dispatch

- **Purpose:** Make `HumanOnly` operational where transport provides actor identity.
- **Scope:** Actor context, operator-only refusal, assurance downgrade for unknown actor.
- **Dependencies:** T05, T16.
- **Likely components:** dispatch, MCP/host handshake, command metadata, audit records.
- **Guidance:** Do not infer human from OS username/TTY; CLI-only remains advisory unless host token
  supplied.
- **Edge cases:** Legacy caller, operator via MCP, agent with delegated operation.
- **Testing:** Actor/operation matrix across CLI and MCP.
- **Acceptance:** Governed agent cannot invoke operator operation directly.
- **Migration:** Legacy unknown actor behavior warned before enforcement hardening.
- **Docs:** Honest assurance boundary.

### T28 — Add scoped delegation grants

- **Purpose:** Support unattended approval without bypassing gates.
- **Scope:** Grant create/revoke/inspect/consume, secret handling, scope/expiry/use policy.
- **Dependencies:** T16, T27.
- **Likely components:** core authority, command handlers, config validation, state/history.
- **Guidance:** Human/host creates grant; repository stores hash and policy, not bearer secret.
- **Edge cases:** Expired/revoked/exhausted grant, stale policy digest, production transition.
- **Testing:** Grant matrix, replay resistance, constant-time token comparison, gate failure.
- **Acceptance:** Delegated approval is distinct in audit and cannot exceed grant or skip readiness.
- **Migration:** Optional feature defaults off.
- **Docs:** Unattended operator guide and threat model.

### T29 — Integrate controller with approval requests

- **Purpose:** Let unattended runs request/consume authorized approvals without self-granting.
- **Scope:** Controller wait state, grant consumption, human handoff, resume.
- **Dependencies:** T25, T28.
- **Likely components:** brain decision engine, session state, status/drive.
- **Guidance:** Controller emits `waiting_approval` when no grant; never silently stops with exit 0.
- **Edge cases:** Grant expires mid-run, gate changes, operator revokes while waiting.
- **Testing:** Interactive handoff and unattended full journey.
- **Acceptance:** Same controller supports safe wait and delegated advance with identical gates.
- **Migration:** Existing sessions without request ids resume into explicit wait.
- **Docs:** Brain approval lifecycle.

## Phase 7: rollout and removal

### T30 — Add migration/compatibility telemetry and doctor checks

- **Purpose:** Measure readiness to remove legacy paths and shapes.
- **Scope:** Local warnings/counters, doctor findings, no network reporting.
- **Dependencies:** T09, T13, T27.
- **Likely components:** doctor, diagnostics, report metrics.
- **Guidance:** Report legacy config/state/actor/JSON use without collecting externally.
- **Edge cases:** CI noise, repeated warnings, read-only files.
- **Testing:** Stable diagnostic codes and suppression after migration.
- **Acceptance:** Operator can list every remaining deprecated surface locally.
- **Migration:** Supports removal decision.
- **Docs:** Deprecation dashboard commands.

### T31 — Regenerate docs and remove expired compatibility

- **Purpose:** Finish migration after published window and passing usage audit.
- **Scope:** Remove legacy config reads, old status projection, deprecated JSON, and stale examples.
- **Dependencies:** T30 plus release decision.
- **Likely components:** config/state loaders, command docs, tests, changelog.
- **Guidance:** Removal is separate release task; do not combine with feature landing.
- **Edge cases:** Archived v1 project; downgrade/upgrade.
- **Testing:** Upgrade matrix and future-schema refusal.
- **Acceptance:** No undocumented compatibility branch remains; old project gets precise upgrade command.
- **Migration:** Final breaking migration with version-policy compliance.
- **Docs:** Release notes and archival upgrade instructions.
