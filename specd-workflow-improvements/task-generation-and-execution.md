# Task generation and execution

## Domain definition

Owns task decomposition, scope, DAG readiness, dispatch, attempts, verification, completion,
concurrency, and repair execution.

## Current behavior

Tasks are Markdown rows with a DAG, role, files, verify, acceptance, and optional planning fields.
Frontier permits pending tasks whose dependencies complete. Completion requires current-HEAD passing
evidence and diff scope. Orchestration adds missions, leases, routing, and controller sessions.

## Evidence from feedback

- [Verify required undeclared test files](../WORKFLOW-FEEDBACK.md#2026-07-20--friction--task-files-cannot-express-the-test-file-its-own-verify-line-requires).
- [Zero selected tests recorded green](../WORKFLOW-FEEDBACK.md#2026-07-20--friction--the-trivial-verify-gate-catches-printf-ok-but-not-a--run-pattern-that-matches-no-tests).
- [Routing vocabulary rejected every craftsman task](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--brain-dispatch-refuses-every-task-routing-vocabulary-does-not-include-the-tasksmd-capability-words).
- [Parallel missions could not complete in one worktree](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--parallel-dispatch-cannot-complete-in-a-single-worktree).
- [Abandoned mission won baseline selection](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--an-abandoned-mission-outranks-the-claimed-one-when-resolving-a-tasks-scope-baseline).

## Main problems

Task rows describe desired scope but cannot amend it safely. Verify commands can be vacuous. Routing,
role, and task capabilities use different vocabularies. Parallel scheduling assumes isolation not
provided. Attempt/baseline identity is weak across retry and repair.

## Root-cause analysis

Planner, verifier, scope gate, and controller were built as adjacent layers without one typed task
contract or host-isolation contract. A task id is treated as permanent attempt identity.

## Desired behavior

Each task plan revision has typed outputs, inputs, capability, evidence, and verify contract. Every
execution is an attempt with fresh baseline/authority/evidence. Scheduler dispatches only work the
host can isolate and later complete.

## Recommended design

- Canonical task contract parses each field once.
- Task kinds: implement, fix, refactor, docs, verify-existing, review, deferred, repair.
- Validate named tests exist or declared test output can create them; reject vacuous selection output.
- Capability registry is shared by roles, tasks, routing classes, mission authority, and docs.
- Task attempt includes attempt number, plan revision, scope revision, baseline head, authority digest,
  status, evidence refs, and completion event.
- Repair scope amendment can include files from multiple original tasks, approved atomically with
  reopen.
- Without host-declared isolated worktrees, controller serializes missions. With isolation, merge
  results through explicit integration/reverify step.
- Mission release abandons one unclaimed mission immediately; live claimed mission always owns
  baseline over expired/abandoned records.
- Controller report consumes lease/session binding without discarding caller flags or requiring a
  redundant authority.

## Workflow implications

Plans fail earlier, execution routes remain legal, retries are clear, and cross-cutting repairs stay
inside evidence gates. “Wave” means actual concurrency only on capable hosts.

## Data-model implications

Add task plan/scope revision, attempt ledger, mission terminal status, abandonment reason, isolation
id/worktree, and evidence attempt id. Preserve old task rows as revision 1.

## CLI implications

`next` shows readiness and isolation decision. `reopen task` creates attempt. Add `brain release` and
clear `retry`. Verify output reports selected tests/check producer. Status shows current attempt and
stale evidence.

## Coding-agent implications

Agent receives one attempt packet and touches only its scope. It cannot claim concurrency support or
reuse old baseline. Repair agent gets explicitly amended scope rather than editing unrelated tasks.

## Compatibility implications

Existing tasks/evidence map to attempt 1. Current greenfield context behavior is retained. Hosts not
declaring worktree isolation get serialized execution, a safe behavioral tightening.

## Failure scenarios

Zero-test output records failing/invalid evidence; missing route fails at plan/start; expired mission
returns task to frontier; merge drift invalidates verification; scope amendment rejection leaves old
task complete and no new attempt.

## Edge cases

Read-only trivial verify remains allowed; manifest/lockfile companions require explicit ecosystem
rule or declaration, not universal magic; directory scope needs bounded selector; sibling task changes
are safe only in isolated worktrees or after fresh serial baseline.

## Testing strategy

Parser/gate conformance, attempt evidence freshness, named-test absence, capability matrices,
single-worktree and isolated concurrency journeys, mission release/expiry, baseline precedence,
report/session binding.

## Implementation recommendations

Serialize by default rather than teaching diff scope to ignore sibling changes; this is smaller and
correct. Add real parallel integration only after host isolation proof exists.

## Trade-offs

Serialization reduces throughput on simple hosts but removes impossible waves. Typed task metadata
adds columns yet saves repair and diagnosis turns.

## Risks

Automatically detecting zero tests is command-specific. Implement reliable Go detection now and a
producer protocol for other runners, not output heuristics for every tool.

## Acceptance criteria

- Task cannot pass with zero selected tests.
- One capability vocabulary routes all valid roles/tasks.
- Attempt 2 requires attempt 2 evidence.
- Single-worktree controller dispatches completable sequence.
- Mission release and baseline selection need no TTL wait/source reading.

## Open questions

- Standard host declaration for isolated worktrees.
- Ecosystem-specific manifest/lockfile companion registry scope.

