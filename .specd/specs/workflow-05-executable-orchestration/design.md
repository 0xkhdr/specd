# Design — workflow-05-executable-orchestration

- references: R1, R2, R3, R4, R5, R6, R7
- boundaries: Canonical task contracts, typed context lanes, attempt-aware verification, mission lifecycle/concurrency, and non-destructive review.
- interfaces: `TaskContract`, versioned context lanes, evidence status, mission release/isolation contract, review restamp, and production orchestration journey.
- invariants: Parse once, missing output is legal, missing input fails, current attempt evidence only, serialization without isolation, and review body preservation.
- failure: Invalid grammar, lane escape, zero-test verification, abandoned mission, merge drift, or stale review returns typed recovery without manual ledger edits.
- integration: Extends current task parser, context manifest, verify executor, evidence ledger, authority, missions/leases, review parser, and command metadata.
- alternatives: Shared-worktree parallelism, placeholder outputs, unbounded directories, and universal test-output heuristics are rejected.
- disposition: accepted
- owner: project maintainers

## Boundaries

Owned code:

- `internal/core/tasksparser.go` and typed field parsers.
- `internal/context`: lane selection, resolution, budgeting, manifest schema, and HUD.
- Verify/evidence/completion paths in core and command layer.
- `internal/orchestration`: mission terminal states, release, baseline precedence, and isolation policy.
- Review scaffold/parser/command and production journey tests/docs.

Excluded: provider-specific model selection, worktree creation, generic test framework inference, and
delegated approval.

## Canonical task contract

Keep byte-stable `TasksMd` as source representation. Add pure `ParseTaskContract(TaskRow)` returning
typed kind, risk, output paths, required context, dependencies, capability ids, verify producer,
evidence requirements, checks, and acceptance refs. Gates and consumers use this result instead of
splitting raw cells independently.

Canonical parsers accept documented values and deterministic legacy aliases during the warning window.
Unknown or ambiguous values fail against task id and column. Capability identity comes from the existing
core registry and is reused by roles, routing classes, authority, and mission validation.

Scaffold examples are compiled through every armed parser/gate in default and production tests.

## Context lanes

Manifest schema v2 adds lane kind, required/existence state, normalized path/query, source digest when
loaded, trust/sensitivity, token cost, authority effect, omission reason, and assurance.

- `required_input`: readable bounded file required.
- `optional_existing_output`: loaded only when present.
- `prospective_output`: authorized path with no content or digest.
- `directory_query`: explicit include/exclude/max-files/max-bytes; stable sorted matches.
- `managed_policy`: selected role, steering, skill, and config pinned by metadata/digest.

Symlink resolution must remain inside repository. Bare directory is an authoring error. Only required and
loaded lanes count budget. Terminal tasks are ignored unless selected for reopen/revalidation.

## Verification and evidence

Evidence records add task attempt, plan/scope revision, baseline, producer id/version, selected-check
status, and context receipt digest. Loader projects missing, failing, stale, malformed, incompatible, or
passing separately.

Initial zero-test protection is deliberately narrow and reliable:

- Go commands with a test selector treat the standard `no tests to run` result as invalid when no package reports selected-test execution.
- Multi-package or other runner ambiguity requires a declared producer status envelope.
- Unknown commands keep exit-code evidence and make no claimed test-count inference.

Read-only roles retain documented trivial verify. Write roles remain subject to verify lint and current
attempt evidence.

## Mission lifecycle and concurrency

Mission gains explicit pending, claimed, released, expired, failed, and completed projection derived from
ledger/lease events. `brain release <slug> <mission-id>` appends one terminal release reason and returns
its task to readiness immediately. Baseline selection considers only the current live claimed mission;
expired/released/abandoned records cannot win.

Host handshake declares isolation capability and identity. Without proven isolation, controller permits
one active write mission per shared worktree/spec and serializes the frontier. With isolation, each
mission pins worktree identity; integration creates a new serial step that merges and revalidates before
completion authority moves.

Dispatch, claim, context acknowledgment, heartbeat, report, and resume preserve session/lease/authority
pins. Zero-progress permanent halt returns non-success with checkpoint and recovery.

## Review

`review` refuses whenever report path exists unless explicit force. `review --restamp` parses machine
metadata and replaces only Git-head/subject fields while preserving all human body bytes. Verdict parser
reads one strict token and stores any following note separately. Evidence subject revision is normative.

Unknown flags fail before mutation through shared command metadata.

## Failure and recovery

- Missing required input: exact lane/path; missing output: normal prospective lane.
- Directory or symlink escape: authoring refusal, no partial manifest.
- Zero selected tests: invalid evidence, rerun with real selector/producer.
- Abandoned mission: immediate release; no TTL wait.
- Merge/baseline drift: fresh integration baseline and verify.
- Existing review: refuse or restamp; never implicit overwrite.

## Verification

- Parser/scaffold/consumer conformance and direct-raw-cell lint.
- Greenfield, mixed output, directory query, symlink, budget, and completed-task context tests.
- Evidence status/attempt freshness and zero-test Go cases.
- Single-worktree serial and isolated integration journeys; mission release/baseline precedence/races.
- Review body byte-preservation, strict verdict note, unknown flag, and stale evidence tests.
- Full production dispatch-claim-context-verify-report-review-complete journey without profile change.

## Deployment and rollback

Land parsers, then lanes, evidence, mission serialization/release, and review. Serialization is safe
tightening and requires no migration. New fields are additive/versioned; old records default attempt 1.
