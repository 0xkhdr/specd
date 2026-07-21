# Design — workflow-07-compatibility-cleanup

- references: R1, R2, R3, R4, R5
- boundaries: Read-only compatibility inventory/metrics, removal eligibility proof, explicit legacy-path removal, archive safety, and synchronized release documentation.
- interfaces: `agents doctor --compat`, workflow metrics report, compatibility registry, removal-exit test, and precise upgrade refusals.
- invariants: No network telemetry, no second metrics store, no premature removal, future schemas fail closed, and archived history is never silently rewritten.
- failure: Unmet window, usage, fixture, owner, docs, or archive gate blocks removal while retaining supported compatibility route.
- integration: Reuses diagnostics, command metadata, history/ledgers, migration tools, generated docs, version policy, and existing test/lint pipeline.
- alternatives: Automatic time-based removal, outbound usage collection, and mixing cleanup with feature work are rejected.
- disposition: accepted
- owner: project maintainers

## Boundaries

Owned code:

- A small compatibility registry in core listing diagnostic code, surface, introduced version,
  minimum removal version/date, replacement, owner, and detector.
- `specd agents doctor --compat` projection using existing doctor surface.
- `report --workflow-metrics` derived from events and ledgers.
- Removal-exit tests, legacy branch deletion, upgrade errors, docs, changelog, and archive fixtures.

Excluded: daemon, network reporting, new metrics database, new workflow features, and automatic removal.

## Compatibility inventory

Each detector receives explicit loaded metadata and returns stable diagnostics for legacy config source,
state schema, compatibility status writes, output schema, unknown actor provenance, and task grammar.
Detectors do not load source prose or secret values. Results sort by code and entity.

`agents doctor --compat` reports active use, replacement command, window, owner, and removal eligibility.
Migrated surfaces stop appearing as active but remain in migration history.

Workflow metrics derive at read time from workflow events, approval/grant/evidence/controller ledgers, and
current state. Counts cover transition attempts/refusals, waits, retries, reopen cycles, stale descendants,
delegated approvals, zero-progress halts, and deprecated use. No aggregate file is written.

## Removal eligibility

One table-driven release test reads the compatibility registry and requires for each proposed removal:

- published two-minor-release minimum and date/version reached;
- zero unsupported active-use findings in release fixtures;
- passing upgrade, downgrade-preflight, archive, default, and production journeys;
- explicit release-owner decision record;
- generated command reference, upgrade guide, archival guide, examples, and changelog synchronized.

Any failed item blocks removal and names the retained code path. Time alone never deletes support.

## Removal implementation

Cleanup is a dedicated breaking-release change. It removes only registry entries whose exit proof passes:

- legacy config discovery and writes;
- legacy state/status mutation paths;
- deprecated machine-output routes;
- legacy task grammar aliases and stale examples.

Old input then fails before mutation with stable code, detected source/schema, minimum supporting binary,
exact migration/upgrade command, and backup requirements. Future schemas continue to fail preflight.
Remaining compatibility branches must have registry ownership or structural lint fails.

## Archive and downgrade safety

Archived v1 specs remain content-addressed and inspectable through an archive reader that does not rewrite
their manifest. Restore/active mutation requires explicit upgrade. Binary downgrade preflight reads schema
header before any write and refuses upgraded state. Config/state backups preserve permissions and are never
deleted by cleanup.

## Failure and recovery

- Read-only filesystem: inventory still reports; migration names write requirement.
- Existing backup: migration refuses overwrite.
- Nested legacy root: detector reports selected root/source deterministically.
- Old JSON client: supported window retains explicit old route; post-window refusal names output upgrade.
- Incomplete docs or release decision: build/release test blocks deletion.

## Verification

- Registry detector and stable ordering tests.
- No-write inventory/metrics tests and secret/source redaction.
- Removal exit matrix for every prerequisite.
- Golden v1/current/future config, state, output, grammar, and archive fixtures.
- Upgrade/downgrade/idempotency/crash tests.
- Default and production journeys, race, repeated-count, gofmt, vet, structural lint, docs lint, and domain regressions.

## Deployment and rollback

Ship inventory at least one release before deletion. Publish removal decision and release notes before
cutting paths. Rollback uses previous compatibility binary plus schema preflight; it never overwrites newer
state. If usage remains, extend window instead of weakening migration safety.
