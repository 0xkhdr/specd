# Migration and backward compatibility

## Domain definition

Owns rollout from current config/state/API/templates to canonical config, versioned transitions,
request routing, and new machine envelopes without corrupting existing projects.

## Current behavior

State schema is v1 and rejects unknown fields on load. Config has no schema file contract beyond
defaults and supports root `project.yml`. JSON surfaces vary between arrays and envelopes. Archived
specs contain hashed manifests and must remain verifiable.

## Evidence from feedback

Configuration and workflow behavior changed across binaries while guidance/config remained stale.
The unexplained evidence-policy pass/fail flip shows why input/source identity must accompany
migration and runtime output.

## Main problems

Several proposed improvements are breaking if landed together: config path/extension, state schema,
status semantics, JSON shapes, actor enforcement, and completed-spec reopen policy.

## Root-cause analysis

Compatibility has focused on individual decoders. Workflow migration needs cross-file discovery,
preview, backup, effective-value equivalence, and host/client capability negotiation.

## Desired behavior

No silent reinterpretation. Old projects keep working through a defined window, receive exact
migration commands, and preserve evidence/history. New binaries fail safely on future schemas;
older binaries cannot overwrite upgraded state.

## Recommended design

Rollout stages:

1. Add source/version fields and read-only new projections.
2. Read canonical plus legacy config; scaffold canonical for new repos.
3. Offer explicit config/state migration with dry-run and backups.
4. Dual-project old status/JSON fields while clients negotiate schema.
5. Warn on legacy use with stable diagnostic codes.
6. Measure locally through doctor/status.
7. Remove after at least two minor releases and published threshold/date.

Config migration handles real `project.yml` plus requested `project.yaml`. State v1 upgrader maps
statuses/tasks/records to cycle/attempt 1, preserves old bytes as `state.v1.json.bak`, validates
replay, then atomically switches. Archived state stays readable without in-place rewrite; upgrade on
explicit restore/inspect if needed.

JSON uses `schema_version` envelopes. For `check --json`, support legacy array through a flag or
content negotiation during window; never silently return an object to old consumers.

Actor unknown remains unknown, not human. Existing complete work does not auto-reopen; new command
performs eligibility check.

## Workflow implications

Operators can upgrade deliberately, automation detects shape, and mixed repositories fail with
clear conflict rather than selecting weaker policy.

## Data-model implications

Add migration record with from/to schema, source/target digests, tool version, actor, timestamp, and
backup path. Migration events do not imply approval.

## CLI implications

`config migrate`, `state migrate`, `doctor --compat`, and `--output-schema`/legacy flag as needed.
All dry-runs are read-only and list exact file operations.

## Coding-agent implications

Agent may inspect and recommend migration but treats operator-owned config/state migration as
authorization-sensitive. It never deletes backups.

## Compatibility implications

This domain is compatibility policy: additive reads first, explicit writes, backups, downgrade
preflight, archived verification, and removal only by version policy.

## Failure scenarios

Unknown future schema refuses; partial migration rolls back or resumes idempotently; canonical and
legacy conflict halts; backup exists prevents overwrite; old binary preflight refuses upgraded state.

## Edge cases

Uncommitted config, read-only filesystem, multiple nested roots, archived v1 specs, empty task map,
legacy blocked status, malformed record unknown to old decoder.

## Testing strategy

Golden v1/current/future fixtures, config source combinations, fault injection, upgrade-downgrade
matrix, idempotent retry, archived verification, old/new JSON clients.

## Implementation recommendations

Ship migration tooling before changing scaffold defaults. Never combine compatibility removal with
reopen feature landing.

## Trade-offs

Dual-read code adds temporary branches. Strict window and local compatibility report prevent them
becoming permanent.

## Risks

Backup files may contain sensitive config. Preserve permissions, warn, and never print values.

## Acceptance criteria

- Every legacy project has deterministic dry-run.
- No migration loses evidence/history/effective config.
- Future schema fails before mutation.
- Old JSON clients retain supported route during window.
- Compatibility removal follows published policy.

## Open questions

- Exact output negotiation mechanism.
- Whether archived records are upgraded lazily or never rewritten.

