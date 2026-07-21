# Debugging and failure recovery

## Domain definition

Owns typed failure diagnosis, retry/repair paths, controller recovery, baseline handling, failed
evidence, and greenfield/debugging-specific workflow behavior.

## Current behavior

Refusals range from typed `AUTHORITY_DENIED` to bare usage text. `status --guide` often knows blockers
that `check` omits. Brain has resume/cancel but no per-mission release. Failed verify records exist;
some completion errors label failing evidence missing. Debugging uses ordinary task model with no
distinct repair attempt.

## Evidence from feedback

- [Evidence policy error named no artifact or fix](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--evidence-policy-blocker-names-no-artifact-no-boundary-and-no-fix).
- [Failing evidence was reported missing with bypass-like advice](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--evidence_missing-says-evidence-is-absent-when-it-exists-and-is-failing).
- [Scope refusal hid baseline and actual recovery](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--outside_scope-recovery-text-does-not-name-the-verb-that-actually-clears-it).
- [Failed brain run wrote consequential checkpoint](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--correction--what-actually-re-pinned-the-baseline-was-a-failed-brain-run-not-brain-resume).

## Main problems

Errors collapse distinct causes, recoveries name unreachable actions, and success exit codes can mean
permanent halt. Debugging a harness issue requires reading implementation. Failed commands do not
consistently disclose durable effects.

## Root-cause analysis

Messages are authored in individual handlers. Recovery is prose rather than transition-plan output.
Controller write-ahead recovery semantics are correct locally but invisible operationally.

## Desired behavior

Every failure answers: what failed, which entity/input, observed versus expected, whether state
changed, who can act, exact legal recovery, and whether retry is safe. Debugging/repair has a legal
attempt model.

## Recommended design

Refusal envelope fields: code, category, entity/ref, observed, expected, inputs/digests, state_changed,
checkpoint/event id, retryable, actor_required, recovery_operations, detail. Central registry maps
codes to exit class and docs.

Distinguish:

- missing, failing, stale, malformed, incompatible evidence;
- waiting, blocked, failed, cancelled, completed controller outcomes;
- scope declaration error, baseline drift, harness-owned path, sibling work;
- config conflict, invalid syntax, unsupported combination;
- authority absent, expired, consumed, wrong scope, missing issuer.

Debugging workflow uses `kind: repair` or reopened attempt, permits bounded investigation context,
then requires real verify. Spike remains learning-only and cannot complete work.

Controller checkpoint is write-ahead: a failed command reports checkpoint id and next `resume` or
cleanup. Per-mission release removes TTL waiting. Halt before progress is non-zero.

## Workflow implications

Agents stop guessing and operators can recover without source inspection. Harness defects and product
defects have separate categories. Repair stays evidence-gated.

## Data-model implications

Persist failure/recovery events where state changed, attempt failure reason, checkpoint disposition,
and recovery actor. Ordinary validation failures may remain ephemeral but carry input digests.

## CLI implications

All JSON errors use envelope. Text renders shortest decisive line plus recovery. `doctor` validates
unreachable config/route combinations before run. `brain release` and reopen/repair commands remove
common dead ends.

## Coding-agent implications

Agent may retry only when envelope says retryable. It never alters config/state/evidence as a
workaround. It opens clarification or requests operator action when required.

## Compatibility implications

Preserve existing error substrings in detail during transition, but stable code becomes contract.
Exit-code corrections require command-reference and changelog notes.

## Failure scenarios

Recovery itself fails CAS; original refusal remains and new conflict is linked. No legal recovery
prints successor/escalation route, not an empty hint. Multiple blockers return ordered list, not only
first opaque error.

## Edge cases

Command writes checkpoint then ledger rejects; evidence import succeeds with fail verdict; no tests
run; old complete task receives verify; operator policy changed mid-attempt.

## Testing strategy

Golden refusal envelopes, recovery command executable tests, state_changed assertions, exit-class
matrix, checkpoint crash injection, and feedback reproductions.

## Implementation recommendations

Generate troubleshooting from refusal registry. Convert top recurring blockers first; do not rewrite
all messages before shared plan exists.

## Trade-offs

Structured errors add fields but reduce prose and round trips. Reporting input digests needs careful
redaction.

## Risks

Recovery command may become stale. Conformance test must execute or validate every registered
recovery against operation metadata.

## Acceptance criteria

- Target feedback cases need no source reading.
- Failing evidence is never called missing.
- Blocked zero-progress run exits non-zero.
- Durable failure effects are disclosed.
- Every recovery names legal actor/operation or says none.

## Open questions

- Stable exit-code taxonomy details.
- Retention policy for persisted failure events that do not mutate state.

