# Specification authoring

## Domain definition

Owns intake, clarification, requirements, design, task-plan creation, review, traceability, and
approval readiness before execution.

## Current behavior

`new` scaffolds requirements/design/tasks. Gates parse EARS, design contracts, task schema, DAG,
coverage, evidence planning, and optional production profiles. Templates and consumers have drifted;
midreq records changes after approval but does not provide a full revision workflow.

## Evidence from feedback

- [Requirements scaffold/parser mismatch](../WORKFLOW-FEEDBACK.md#2026-07-20--friction--requirements-scaffold-cannot-pass-parserequirements-and-the-error-blames-tasksmd).
- [Design scaffold failed its contract](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--the-scaffolded-designmd-fails-the-design-contract-gate-it-is-scaffolding-for).
- [Line-wrapped EARS criteria failed unexpectedly](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--ears-gate-is-line-scoped-but-the-requirements-template-is-not).
- [Design mentions counted as coverage](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--the-design-gate-counts-requirement-mentions-not-requirement-coverage-auto-approval-finding).
- [Task scopes could not satisfy their own verifies](../WORKFLOW-FEEDBACK.md#2026-07-20--friction--per-task-verify-names-a-test-in-a-package-whose-declared-files-cannot-contain-it).

## Main problems

Scaffolds are not executable contracts. Clarification is informal. Design coverage checks presence,
not behavioral disposition. Task authoring guesses files, tests, capability and evidence grammars.
Approval errors arrive after costly authoring.

## Root-cause analysis

Each artifact has separate templates, parsers, prose instructions, and gate interpretations. No
generated schema or conformance journey proves fill-in instructions produce valid artifacts.

## Desired behavior

Authoring is iterative, previewable, and revisioned. Clarifications are records. Canonical parsers
drive templates, docs, errors, and gates. Approval proves semantic trace structure without pretending
deterministic gates can judge product fit.

## Recommended design

- Intake records provenance, owner, risk, source, desired routing, and whether Specd is appropriate.
- Clarification requests link to exact artifact/requirement and block only affected readiness.
- Requirements parser joins Markdown continuation lines or scaffold explicitly forbids them; joining
  is preferable user behavior.
- Design requires each requirement to map to interface, invariant/failure, and verification or an
  explicit disposition. Mere mention is insufficient.
- Task plan distinguishes `files` (authorized outputs) from `context` (required inputs), test file,
  work kind (`implement|verify|review|defer|repair`), evidence producer, capability ids, and risk.
- Shared typed parsers generate scaffold examples and command docs.
- Readiness preview shows warnings/errors while authoring, with artifact/key/fix.
- Approved edits create artifact revision and impact plan; no silent byte changes.

## Workflow implications

Fewer approve/reject loops, earlier detection of unsatisfiable task scope, and explicit human review
for fit. Greenfield, debugging, maintenance, and read-only verification use distinct task kinds.

## Data-model implications

Version artifacts with source digest, parent revision, author, submission/review state, requirement
coverage edges, clarification ids, and dispositions. Task plan revision owns file scopes.

## CLI implications

Add `check --readiness`, artifact submit/reject details, clarification commands, and plan/coverage
views. Keep Markdown authoring; no new DSL is needed.

## Coding-agent implications

Agent authors only current draft revision, does not approve it, and resolves parser findings before
handoff. It records unknowns as clarification rather than inventing decisions.

## Compatibility implications

Existing six-column tasks remain valid in default profile. Production/new profiles add typed fields
through optional columns, then migrate with warnings. Legacy design headings may parse into canonical
fields during window.

## Failure scenarios

Unknown requirement id points to requirements artifact; incompatible evidence producer fails plan;
missing test path names likely file; unresolved clarification blocks only affected transition;
artifact drift invalidates submitted review.

## Edge cases

Requirement already implemented uses `kind: verify`; read-only task uses explicit trivial evidence;
cross-cutting repair declares approved multi-task scope; deferred requirement requires owner/reason
and parent coverage disposition.

## Testing strategy

Golden scaffold fill-ins under default/production, parser round-trip, template-consumer parity,
wrapped Markdown, coverage edge matrices, task verify reachability, and migration fixtures.

## Implementation recommendations

First consolidate parsers and conformance. Avoid heuristic source-code semantic analysis as a gate;
use explicit coverage edges and human review.

## Trade-offs

More structured metadata adds authoring fields, but replaces repeated guessing and late deadlocks.
Markdown remains primary interface.

## Risks

Overly rigid design coverage may encourage checkbox prose. Keep gates structural and show human fit
review as separate approval responsibility.

## Acceptance criteria

- Filled scaffolds pass consumers.
- Every requirement has explicit design and task disposition.
- Task plan catches missing test/output/evidence producer/routing capability before execution.
- Clarification state is visible and scoped.
- Approved artifact changes create revision and staleness.

## Open questions

- Minimum design coverage edge set for low-risk default profile.
- Whether Markdown continuation joining changes byte-stable requirement identity.
