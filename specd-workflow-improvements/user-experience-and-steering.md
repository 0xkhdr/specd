# User experience and steering

## Domain definition

Owns generated instructions, steering authoring/selection, status/check output, help, next actions,
review ergonomics, and human-agent handoffs.

## Current behavior

Init writes managed steering templates and an AGENTS guide. Steering machine selection depends on
metadata. Status, guide, check, help, and command handlers have partially independent wording. Review
scaffold stores HEAD in prose. Successful check is silent.

## Evidence from feedback

- [Managed steering region put placeholders before real law](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--steering-managed-region-forces-placeholder-text-ahead-of-real-project-law).
- [No pre-spec steering inspection command](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--improvement--no-way-to-ask-whether-steering-is-actually-filled-in).
- [Passing check produced no approval record](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--improvement--a-passing-specd-check-prints-nothing-to-approve-on).
- [Review command destroyed an in-progress audit](../WORKFLOW-FEEDBACK.md#2026-07-21--friction--specd-review-slug-destroyed-a-1011-line-in-progress-audit-the-anti-clobber-guard-keys-on-the-one-field-an-auditor-is-known-to-leave-stale).
- [Criterion route was absent from reached guidance surfaces](../WORKFLOW-FEEDBACK.md#2026-07-20--improvement--every-task-complete-and-check-clean-but-criterion-coverage-reads-012-with-no-next-action-named).

## Main problems

Machine-owned metadata and human prose share clobber regions. Guidance is broad rather than one
actor-specific action. Silent success cannot be audited. Write verbs look like read verbs. Strict
field grammar is not shown where authored.

## Root-cause analysis

Outputs were built per command and templates optimized for scaffolding, not repeated operation.
Machine metadata ownership was not separated from human project law.

## Desired behavior

Users see current mode, state, readiness, blocker, actor, and one next operation. Steering is safely
editable and inspectable before first spec. Destructive/write-shaped commands disclose effects and
preserve user content.

## Recommended design

- Managed steering region contains selection metadata only; human content lives outside and is never
  refreshed over.
- `specd steering inspect [--json]` lists metadata, digest, selected/omitted reason, placeholder
  state, and content size without slug.
- Generated AGENTS begins with request routing, then managed loop.
- Successful check prints slug, transition, revision, config/source digest, gates passed.
- Guide ranks one exact next action, then alternatives; human handoffs are separate.
- Operation metadata declares read/write/external effect and unknown flags fail before mutation.
- Review scaffold refuses any existing file without force; restamp preserves body; verdict token and
  note have separate fields.
- Help and usage errors use command metadata only.

## Workflow implications

Bootstrap becomes verifiable, agents spend fewer turns rediscovering commands, and users can trust
whether a command reads or writes. Steering remains durable law rather than template debris.

## Data-model implications

Steering selection record carries metadata/content digests and placeholder flag. Guidance envelope
adds primary action, alternatives, mode, actor, effect, and reason.

## CLI implications

Add steering inspect and review restamp. Improve check/status/help outputs without adding redundant
verbs where flags suffice.

## Coding-agent implications

Agent reads selected real steering content, sees one legal action, and never calls a write verb to
inspect state. Mode disclosure prevents accidental Specd activation.

## Compatibility implications

Managed-region repair can split metadata while preserving below-marker content. Existing review
reports parse; restamp is additive. Output additions may affect silence-dependent scripts, so offer
`--quiet` explicitly.

## Failure scenarios

Placeholder steering in production is warning/error per config; missing metadata explains omission;
existing review report refuses overwrite; stale guidance digest forces refresh.

## Edge cases

Intentional short steering file should not be called placeholder based on size alone; use canonical
placeholder digests. Multiple possible next actions retain stable ranking.

## Testing strategy

Managed-region preservation, steering selection, placeholder digests, guide/dispatch parity, command
effect/flag lint, review body preservation, output golden tests.

## Implementation recommendations

Prefer improving existing status/check/help over adding commands, except steering inspection has no
pre-spec home and merits one read-only surface.

## Trade-offs

Non-empty success output changes scripts; explicit `--quiet` makes intent clear. One primary action
may hide optional power, retained under alternatives/help.

## Risks

Generated AGENTS repair could move user content incorrectly. Migration must compare managed digests
and preview.

## Acceptance criteria

- Steering real content survives refresh.
- Steering validity inspectable before spec creation.
- Check success is auditable.
- Guide primary action is actor/route legal.
- Review content cannot be accidentally overwritten.

## Open questions

- Placeholder severity in default profile.
- Whether primary next action should be stable command string or operation id plus arguments.

