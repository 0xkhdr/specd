# Design — workflow-02-explicit-activation

- references: R1, R2, R3, R4, R5, R6
- boundaries: Canonical config discovery/migration and deterministic request routing; no workflow-state v2 or automatic language classifier.
- interfaces: `ConfigResolution`, `ResolveConfigSource`, `RequestModeResolution`, config show/validate/migrate operations, and mode-bearing host envelopes.
- invariants: One effective config source, explicit provenance, no silent policy weakening, general mode by default, and no mutable Specd operation outside managed mode.
- failure: Conflicts and malformed canonical policy fail before gates or mutation; migration remains previewable, atomic, and recoverable.
- integration: Extends existing strict YAML parser, scaffold, command metadata, handshake, MCP, and generated agent guide with additive versioned fields.
- alternatives: Generic YAML dependency and natural-language auto-routing are rejected; implicit canonical precedence over conflicting policy is rejected.
- disposition: accepted
- owner: project maintainers

## Boundaries

Owned code:

- `internal/core/config_loader.go`, `paths.go`, and `doctor.go`: root discovery, source selection, normalization, digests, and diagnostics.
- `internal/core/scaffold.go` and embedded templates: canonical `.specd/config.yaml` creation.
- New config command handler registered through existing command metadata: show, validate, migrate, and dry-run.
- Pure request-mode resolver in core; handshake, drive, MCP, and generated `AGENTS.md` integrations.
- Upgrade, configuration, host, and agent-routing documentation.

Excluded: full YAML, remote/global policy, automatic content classification, state v2, and delegation.

## Configuration source interface

`ResolveConfigSource(cwd)` finds the nearest project root using existing repository/project markers,
resolves symlinks, and inspects only:

1. `<root>/.specd/config.yaml`;
2. `<root>/project.yml`;
3. `<root>/project.yaml`.

It returns `ConfigResolution` with root, selected path and kind, source digest, normalized effective
digest, deprecation diagnostics, duplicate paths, and conflict keys. Raw secret values never enter diagnostics.

All present sources are parsed with the existing strict scalar/subsection parser. `.yaml` becomes an
accepted extension; unsupported anchors, aliases, sequences, multi-document input, unknown keys, or
bad indentation keep exact line errors. Equal normalized sources may select canonical and warn.
Different normalized values fail closed instead of using precedence.

Environment overrides remain last in effective-value assembly but every overridden key reports its
environment provenance. Governed values are redacted in text and JSON.

## Scaffold and migration

Fresh init embeds `.specd/config.yaml`; it never writes a legacy filename. Config stays outside
managed clobber regions. Existing config is operator-owned and init does not replace it.

`specd config migrate --dry-run` returns ordered file operations, digests, permission plan, backup
path, and effective equivalence. Real migration under a project config lock:

1. Refuses malformed, conflicting, unreadable, or ambiguous sources and existing backup collisions.
2. Writes canonical content to a target-directory temporary file with source permissions.
3. Parses the temporary canonical file and compares its normalized effective value to source.
4. Atomically renames canonical into place and fsyncs the directory.
5. Renames legacy source to a non-overwriting `.specd-v1.bak` and fsyncs.
6. Writes a digest-only migration result.

Replay detects completed steps and returns the same outcome. It never deletes backups.

## Request-mode interface

`ResolveRequestMode(RequestModeInput)` is pure. Input includes explicit directive, active host session,
enforceable repository rule, configured default, selected spec, and host capability proof. Output:

- mode: `general`, `consult`, or `managed`;
- resolution source and enforcement level;
- selected slug where required;
- assurance and missing host capabilities;
- permitted operation classes and blocker/recovery.

Precedence is explicit directive, active session, enforceable rule, configured default, then compiled
`general`. A classifier may return a recommendation separately and cannot alter this result.

General exposes no mutable Specd operations and requires no bootstrap. Consult filters command metadata
to read-only operations. Managed requires a slug or intake route, then retains the existing bootstrap
and authority loop. Switching mode or slug invalidates prior authority/session binding.

Execution mode in `state.json` remains distinct and is rendered as `execution_mode` in new envelopes.

## Host and guide integration

Generated `AGENTS.md` begins with activation rules, mode disclosure, and host enforcement ceiling. Only
the managed branch contains the lifecycle recipe. Handshake, drive, and MCP return mode, source,
enforcement, slug, and assurance. Hosts that cannot hide tools or intercept writes report advisory;
they never claim required enforcement.

## Failure and recovery

- Conflicting sources: list paths and conflicting keys, then config migration/show recovery.
- Malformed canonical: fail without legacy fallback.
- Nested or symlink root ambiguity: report resolved candidates and refuse.
- General directive against enforced path: refuse before write and name rule/path.
- Stale session: invalidate authority, disclose fallback mode, require explicit reattachment.
- Interrupted migration: dry-run reports completed and remaining idempotent steps.

## Compatibility

Both legacy filenames remain readable with stable warnings for at least two minor releases. New
projects scaffold canonical only. Existing explicitly attached managed sessions remain managed; new
unbound sessions resolve general. Machine fields are additive and versioned.

## Verification

- Config source-combination, normalized-equality, conflict, nested-root, symlink, permission, parser,
  environment provenance, and crash-injection tables.
- Migration dry-run no-write and idempotent replay tests.
- Request-mode precedence and enforcement matrices.
- General-mode no-Specd-command, consult read-only, managed-bootstrap, and authority-invalidation tests.
- Template/runtime parser conformance, docs lint, race, and black-box upgrade journey.

## Deployment and rollback

Land resolver before scaffold and migration. Land pure router before guide changes. Legacy reads make
rollback safe during the window; canonical projects retain an explicit downgrade preflight and backup.
