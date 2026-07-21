# Context management and enforcement

## Domain definition

Owns bounded task knowledge, input/output path semantics, digest selection, role/authority packets,
context budget, and host enforcement disclosure.

## Current behavior

Machine context selects task, requirements, design, role, steering, and declared files. Current
`SelectRequiredLanes` correctly skips missing declared outputs while requiring requirements/design/
role. Authority carries declared write paths. Host contract admits Specd cannot stop tool, file, or
network actions and labels unsupported hosts advisory.

## Evidence from feedback

- [Greenfield outputs were once treated as required inputs](../specd-context-greenfield-debug-analysis.md#executive-summary); current code contains the minimal fix.
- [Directories failed as unreadable files](../WORKFLOW-FEEDBACK.md#2026-07-20--friction--specd-context-refuses-any-task-that-declares-a-directory-blaming-the-wrong-cause).
- [Steering machine selection silently omitted templates](../WORKFLOW-FEEDBACK.md#2026-07-20--friction--steering-templates-never-load-into-the-machine-manifest).
- [Read-only agent violated tool boundaries](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--a-read-only-subagent-committed-and-pushed-to-the-remote-and-briefing-was-the-only-control).

## Main problems

Input, optional existing output, prospective output, directory query, and steering lanes are not
fully typed end to end. Budget can evaluate irrelevant completed tasks. Authority statements may be
mistaken for host containment.

## Root-cause analysis

Context and authority share paths but different requirements. Earlier code treated all as readable
sources; current fix is local. Host capabilities and context selection are reported separately.

## Desired behavior

Manifest makes each lane's purpose, existence requirement, digest, sensitivity, trust, budget cost,
and authority effect explicit. Missing output is legal; missing input is not. Host enforcement
ceiling travels with authority.

## Recommended design

Lane kinds:

- `required_input`: must resolve to readable bounded file.
- `optional_existing_output`: load if present; absence is normal.
- `prospective_output`: authorized path, no content/digest.
- `directory_query`: explicit include/exclude/max-files/max-bytes selector; bare directory rejected.
- `managed_policy`: role/steering/config selected by pinned metadata.

Only required and loaded lanes count tokens. Completed tasks are not budget-gated unless reopened or
selected for revalidation. Manifest includes config, palette, policy, artifact, plan, scope, and host
capability digests.

Authority is derived from task attempt and host capabilities. It cannot be widened by context prose.
When host cannot enforce tools/paths/network, assurance is advisory and managed production may refuse
if policy requires containment.

## Workflow implications

Greenfield and debugging tasks load reliably, read-only surveys use bounded directory queries, and
agents know whether instructions are enforceable or advisory before edits.

## Data-model implications

Extend manifest lane schema and authority attempt/scope revision. Preserve content trust labels.
Store selection omissions with reason and severity.

## CLI implications

`context --json` remains authoritative; HUD shows lane type, required/absent, tokens, authority
effect, and assurance. Task gate validates directory query syntax.

## Coding-agent implications

Agent may create prospective outputs only after authority. It cannot treat a referenced file as
writable. It stops on digest mismatch or missing required input.

## Compatibility implications

Existing `files` map optional/prospective depending on existence; `context` maps required input.
Bare directory declarations warn then fail in production after window.

## Failure scenarios

Symlink escape always refuses; missing required input names column/path; omitted steering total emits
error or warning per policy; budget overflow names largest lanes and reachable author/operator fix;
host overclaims capability lowers assurance or fails conformance.

## Edge cases

Mixed existing/new outputs, reopened deleted output, generated test file, binary/large source, nested
repo, sensitive config, directory selector matching zero files.

## Testing strategy

All greenfield analysis cases, directories, symlinks, budgets, completed tasks, steering selection,
attempt authority, host capability ceiling, deterministic lane order/digests.

## Implementation recommendations

Promote current missing-output behavior into typed lane model. Do not auto-create placeholders or
expand unbounded directories.

## Trade-offs

Directory queries add schema, but avoid either unusable scout tasks or unbounded context. Advisory
labels may disappoint users but prevent false containment.

## Risks

Manifest shape changes affect drivers; version it and provide additive fields first.

## Acceptance criteria

- Missing output succeeds and stays authorized.
- Missing required input fails precisely.
- No bare directory causes opaque EISDIR.
- Budget ignores terminal tasks not selected.
- Authority and assurance reflect host enforcement honestly.

## Open questions

- Default max files/bytes for directory queries.
- Whether production should require sandboxed or merely governed host capability.
