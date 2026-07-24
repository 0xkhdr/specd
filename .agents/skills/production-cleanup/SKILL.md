---
name: production-cleanup
description: Audit and safely remove repository redundancy before production. Use when Codex must consolidate duplicate or historical feedback records, distinguish canonical product documentation from implementation artifacts, retire completed planning/spec state, remove obsolete files, or prove a repository contains only production-relevant material without relying on fixed filenames.
---

# Production Cleanup

Reduce repository clutter only when evidence proves an artifact is duplicated, completed,
generated, obsolete, or excluded from the production source of truth.

## Establish authority

1. Read repository instructions and deployment/build configuration.
2. Inspect the worktree before changes. Preserve unrelated user edits.
3. Determine whether the request is audit-only or authorizes cleanup. If ambiguous, stop
   after the plan.
4. Treat repository workflow state as tool-owned. Use its supported lifecycle commands;
   never hand-edit state, evidence ledgers, task markers, or equivalent control files.

## Inventory and classify

Inventory tracked, ignored, and relevant untracked artifacts. Use references, history,
generators, tests, lint rules, packaging, deployment inputs, and lifecycle state as evidence.
Do not classify by filename, age, directory, or apparent spec origin alone.

Assign each candidate exactly one disposition:

- `keep`: canonical production, operator, contributor, compliance, or build input.
- `consolidate`: unique content belongs in another canonical artifact.
- `archive`: history remains valuable but is not active production input.
- `delete`: fully duplicated, generated-recreatable, expired, or provably obsolete.
- `review`: evidence is insufficient or contradictory.

For every non-keep item record: path, disposition, evidence, content destination if any,
references requiring updates, validation command, and recovery method.

## Consolidate record ledgers

Parse records as complete units. Determine completion from implementation evidence:
current code/tests, linked changes, release records, or terminal workflow state. Wording such
as “resolved,” “done,” or “implemented” is not sufficient by itself.

- Move completed records into one durable implemented-history artifact.
- Keep unresolved, partially fixed, regressed, and unverifiable records in one active
  backlog artifact.
- Preserve record text, dates, identifiers, provenance, and resolution evidence.
- Deduplicate only semantically identical records; retain links between superseding records.
- Remove redundant source ledgers only after record counts and identifiers reconcile.

## Reduce documentation

Build a reference map before deleting documentation. Keep documents that contain unique
user, operator, API, architecture, security, compliance, deployment, troubleshooting, or
contributor knowledge. Keep generated documents when release packaging or repository checks
expect them.

Consolidate unique useful sections before removing an obsolete document. Update inbound
links, indexes, generators, lint allowlists, tests, and packaging in the same cleanup.
Never delete a document merely because a spec created it.

## Retire completed workflow artifacts

Confirm terminal lifecycle state, completed work, retained implementation evidence, and no
active references. Prefer the repository’s supported archive/prune command. If none exists,
include explicit deletion in the plan and remove only after authorization. Never manufacture
completion or modify control ledgers directly.

## Apply safely

1. Capture the clean baseline and exact candidate manifest.
2. Consolidate content before deleting sources.
3. Delete only explicit reviewed targets; avoid broad globs and recursive repository-root
   operations.
4. Search again for stale references and duplicate records.
5. Run the smallest relevant checks, then the full production build/test/lint gates required
   by repository instructions.
6. Compare final tracked files and deployment/package contents against the manifest.
7. Report kept, consolidated, archived, deleted, unresolved, validation results, and recovery
   path. Do not claim “production-ready” while review items or failing gates remain.

Prefer deletion over new structure. Create an archive, index, or automation script only when
retention rules or repeated measurable work require it.
