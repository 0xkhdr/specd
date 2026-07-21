# Configuration and project discovery

## Domain definition

Owns project-root resolution, config source selection, parsing, validation, precedence, migration,
and source identity exposed to gates and agents.

## Current behavior

`configPaths` points to root `project.yml`. `LoadConfig` accepts only `.yml`, overlays environment
values, and parses a strict YAML subset. `main.go` dispatches with root `.` even though `FindRoot`
exists. `specd init` writes root `project.yml`; config is operator-owned. Runtime metadata already
lives under `.specd`.

## Evidence from feedback

- [`project.yml` separators and inline comments disagreed](../WORKFLOW-FEEDBACK.md#2026-07-21--friction--projectyml-list-separators-disagree-and-inline--comments-are-parsed-as-value).
- [`init --repair` could not repair advertised layout](../WORKFLOW-FEEDBACK.md#2026-07-21--friction--specd-init---repair-cannot-repair-the-layout-finding-it-is-told-to-repair).
- [Configuration changes collided with task scope](../AIDO-WORKFLOW-FEEDBACK.md#2026-07-21--friction--the-two-files-agentsmd-requires-an-agent-to-touch-are-outside-every-tasks-scope).

## Main problems

- Requested filename and real filename differ (`project.yaml` versus `project.yml`).
- Root-level config is visually detached from `.specd` and can pollute ordinary repos.
- Source/precedence are not surfaced in most results.
- Parser behavior is narrower than users infer from `.yml` and template comments.
- Environment overlays can change governed behavior without a source digest explanation.

## Root-cause analysis

Config began as one explicit path and later accumulated many policy keys. Discovery, migration,
syntax, and provenance never became their own contract.

## Desired behavior

Canonical `.specd/config.yaml`, discovered from nearest `.specd`, validated before governed work,
and identified by normalized digest. Legacy files work temporarily but never create silent
ambiguity. Config contains policy only.

## Recommended design

Resolution order:

1. Find nearest repository root containing `.specd` for non-init commands.
2. Inspect `.specd/config.yaml`, root `project.yml`, and root `project.yaml`.
3. Canonical only: load.
4. One legacy only: load with structured deprecation warning.
5. Canonical plus semantically equal legacy: load canonical, warn ignored legacy.
6. Any semantic conflict or two conflicting legacy files: fail `CONFIG_CONFLICT`, listing keys.
7. Apply only documented environment overrides, include their names and redacted value digests in
   effective-config digest.

Do not support global governing config. Future global display defaults may exist, but cannot alter
gates, security, authority, evidence, routing enforcement, or approval.

`specd config migrate --dry-run` previews normalized output. Real migration atomically writes and
validates `.specd/config.yaml`, then renames source to `project.yml.specd-v1.bak` or
`project.yaml.specd-v1.bak`. Reads never mutate.

Keep workflow metadata in state/event files. Keep bearer delegation secrets outside tracked config.

## Workflow implications

Initialization and diagnosis become predictable from nested directories. Every readiness plan pins
config source/digest, so unexplained gate flips become attributable.

## Data-model implications

Add `ConfigSource{Path, Kind, Digest, Legacy, Overrides}` and `schema_version` in canonical file.
Config parser accepts `.yaml` and `.yml`; unsupported anchors, sequences, or multi-doc input fail
with exact line and supported subset.

## CLI implications

Add `specd config show|validate|migrate`; `show --json` returns source and effective values with
secrets omitted. `doctor` checks duplicates, parser compatibility, root discovery, and migration.

## Coding-agent implications

Agent may read effective config and migration diagnostics. It never edits operator policy to unblock
itself; it reports exact migration or conflict action.

## Compatibility implications

Legacy reads remain for two minor releases. Current code's `.yml` extension restriction must be
removed before canonical `.yaml` scaffolding. Docs/tests/templates change together. Env behavior is
preserved but made visible.

## Failure scenarios

Conflicting files fail before gates; malformed canonical does not fall back to weaker legacy;
interrupted migration leaves either valid canonical or untouched legacy plus temp cleanup;
unreadable parent `.specd` reports root and permission error.

## Edge cases

Nested Specd repos select nearest root; symlinked cwd resolves canonical real path; equal values with
different comments count equal; unknown keys fail; existing backup blocks migration until user
chooses a path.

## Testing strategy

Source-combination table, nested root tests, symlink cases, parser grammar, normalized equality,
fault-injected migration, env provenance, scaffold parse, and upgrade/downgrade matrix.

## Implementation recommendations

Build resolver and diagnostics before moving scaffold. Preserve current simple parser; fix comment
and delimiter consistency rather than adding a YAML dependency.

## Trade-offs

Strict conflict refusal is noisier than precedence but prevents silent policy weakening. No global
governing config sacrifices convenience for reproducibility.

## Risks

Automation may parse warnings from stderr. Provide structured diagnostics and a published window.

## Acceptance criteria

- Fresh init writes `.specd/config.yaml` only.
- Both real legacy `project.yml` and requested `project.yaml` migrate.
- Conflicts fail with key list.
- Nested invocation resolves same source/digest as root invocation.
- Migration preview writes nothing and real migration preserves effective values.

## Open questions

- Exact two-release deprecation dates.
- Whether CLI flags may override governed config or only operation-local behavior.

