# S2: Deprecation Cleanup — Config Migration

## Purpose and Requirement Coverage

Remove the legacy JSON→YAML config migration tool (`specd migrate config`) — the
`RunMigrate` handler, the `internal/cmd/init.go` `--migrate` flag wiring, and the two
migration-specific functions in `internal/core/config_migrate.go` — while preserving the
config-rendering and config-validation logic that happens to live in the same file but is
**not** migration-specific.

Covers: **R4** (remove legacy config migration).

## Verified Current State

### The migration command chain

- `internal/cmd/init.go:96-99` — `RunInit`'s `--migrate` branch:
  ```go
  if args.Bool("migrate") {
      migrateArgs := args
      migrateArgs.Pos = []string{"config"}
      return RunMigrate(migrateArgs)
  }
  ```
- `internal/cmd/migrate.go` (72 lines) — `RunMigrate` (lines 16-72) implements
  `specd migrate config [--dry-run] [--global]`. It calls `core.MigrateConfigPreview`
  (`migrate.go:52`, for `--dry-run`) and `core.MigrateConfigFile` (`migrate.go:59`,
  otherwise), and resolves the legacy path via `core.LegacyConfigPath` (`migrate.go:27,48`).
  **This entire file's reason to exist is the migration command — delete it wholesale.**
- `internal/core/config_migrate.go` (251 lines, 8 functions) — **only 2 of its 8 functions
  are migration-specific**:
  - `MigrateConfigPreview` (line 123) — DELETE. Called only from `migrate.go:52`.
  - `MigrateConfigFile` (line 161) — DELETE. Called only from `migrate.go:59`.
  - `RenderConfigYAML` (line 16) — **KEEP.** Called from `internal/cmd/init.go:497` as part
    of normal (non-migration) config scaffolding. This is NOT migration-specific despite
    living in this file.
  - `wKV` (line 95), `quoteYAML` (line 118) — **KEEP.** Private helpers for
    `RenderConfigYAML`.
  - `ValidateConfigDoc` (line 197) — **KEEP.** Called from `internal/core/config_validate.go:40`,
    unrelated to migration — it's the general config-document validator.
  - `checkEnumDoc` (line 220), `valueAtPath` (line 238) — **KEEP.** Private helpers for
    `ValidateConfigDoc`.
  - **The analysis plan's F4/§6 assumption ("delete `config_migrate.go` and `RunMigrate`
    handler") is corrected here: `config_migrate.go` as a whole file must NOT be deleted —
    only 2 of its 8 functions are migration-specific. `RunMigrate` itself lives in the
    separate `internal/cmd/migrate.go`, which IS deleted wholesale.**
- `internal/core/config_migrate_test.go` (3 tests):
  - `TestMigrateConfigPreviewAndFile` (line 11) — DELETE, tests the deleted functions.
  - `TestConfigYAMLJSONCompatibility` (line 40) — **KEEP**, tests `RenderConfigYAML`.
  - `TestEnsureGlobalConfigScaffold` (line 104) — **KEEP**, tests `EnsureGlobalConfigScaffold`
    (defined in `internal/core/config_scaffold.go`, unrelated to migration; it is simply
    co-located in this test file and has no dedicated test file of its own).

### A stale warning message

- `internal/cmd/init.go:514` (approximate — verify live) emits an init warning coded
  `legacy-config-deprecated` whose message currently directs the user to run
  `specd migrate config` to convert. Once this spec lands, that command no longer exists —
  **this message text must be updated** (e.g. point at manual conversion instructions in
  docs, or drop the actionable suggestion and just state the JSON config is deprecated but
  still read).

### What stays untouched

- `core.LegacyConfigPath` / `core.ConfigPath` (`internal/core/paths.go:69-83`) — used for
  reading legacy JSON config at runtime (backward-compat read support). Per the analysis
  plan's own explicit constraint, JSON *read* support is preserved; only the migration
  *tool* is removed. Do not touch these.
- `internal/core/config_loader.go`, `internal/core/config_scaffold.go` — unrelated to
  migration, no changes.

## Proposed Design and End-to-End Flow

1. Delete `internal/cmd/migrate.go` entirely.
2. In `internal/core/config_migrate.go`, delete only `MigrateConfigPreview` and
   `MigrateConfigFile` (and any now-unused imports those two functions alone required —
   check imports after deletion, since `RenderConfigYAML`/`ValidateConfigDoc` likely still
   need most of them).
3. In `internal/core/config_migrate_test.go`, delete only `TestMigrateConfigPreviewAndFile`.
4. In `internal/cmd/init.go`, remove the `--migrate` branch (lines ~96-99) from `RunInit`,
   and update the doc comment above it (currently states "With --migrate it delegates to
   RunMigrate...") to remove that clause.
5. In `internal/cmd/init.go`, update the `legacy-config-deprecated` warning message (~line
   514) to no longer reference `specd migrate config`.
6. Remove the `--migrate` flag from `init`'s `CommandMeta` in `internal/core/commands.go`
   (its `Flags` list currently includes a `migrate` entry for the `init` command — verify
   and remove; this is separate from the standalone `migrate` `CommandMeta` entry already
   deleted in S1).

## Interfaces, Contracts, Data, Configuration, Dependencies

- `specd init` loses its `--migrate` flag. `specd init --migrate` becomes a plain usage
  error (exit 2) once the flag is removed from `init`'s `CommandMeta`/flag parsing — verify
  this is the actual resulting behavior (vs. silently ignoring the flag) during validation.
- No `state.json` changes. No YAML config schema changes — `RenderConfigYAML`'s output
  format is unaffected; it is preserved verbatim.
- Legacy JSON config files can still be *read* at runtime (unchanged); they simply can no
  longer be *migrated* to YAML via a built-in command. Users must convert manually or keep
  using JSON.

## Invariants, Security, Errors, Observability, Compatibility, Rollback

- No changes to atomic writes, CAS, or exit codes.
- **Compatibility (breaking, intentional)**: `specd migrate config` and `specd init --migrate`
  both go away. Anyone relying on the built-in converter must hand-edit their config to
  YAML or continue running on legacy JSON. Document in release notes (S6 scope).
- **Security**: removing `MigrateConfigFile` removes a file-rename/write path
  (JSON→`.bak` rename plus new YAML write) from the binary's surface — a minor hardening
  win, consistent with the analysis plan's F3-adjacent reasoning for `update.go`.
- **Rollback**: pure deletion + one flag removal + one message edit; `git revert` is
  sufficient.

## Acceptance Criteria and Validation Commands

- `go build ./...` succeeds.
- `go test ./... -race -count=1` passes.
- `grep -rn "RunMigrate\|MigrateConfigPreview\|MigrateConfigFile" internal/` returns nothing.
- `grep -n "RenderConfigYAML\|ValidateConfigDoc" internal/core/config_migrate.go` still
  returns matches (proves these were NOT accidentally deleted).
- `grep -n "migrate" internal/cmd/init.go` returns nothing (branch and doc comment both
  gone) except possibly inside unrelated identifiers — inspect any remaining hit manually.
- Manual check: `go run . init --migrate` in a scratch directory returns a usage error
  (exit 2), not a crash and not a silent no-op.
- Manual check: `go run . migrate config` returns the generic unknown-command exit (2) —
  this is actually validated by S1 (the `migrate` alias removal), but re-confirm here since
  S2 removes the underlying handler S1's alias would have called.

## Open Decisions and Deviations

- **Deviation from analysis plan F4**: `config_migrate.go` is NOT deleted wholesale — only
  2 of its 8 functions are migration-specific. Deleting the whole file would break
  `init.go`'s normal (non-migration) config scaffolding via `RenderConfigYAML` and
  `config_validate.go`'s use of `ValidateConfigDoc`. This is a correction based on live
  cross-reference grep, not a stylistic choice.
- **Deferred, not required**: the file is arguably misnamed once its migration functions
  are gone (it's now a config-rendering/validation file that happens to be called
  `config_migrate.go`). A rename to e.g. `config_render.go` would be more accurate but adds
  unrelated churn (import path etc. are unaffected since it's all package-internal, but the
  filename rename itself is a bigger diff for no functional gain). Left as an optional
  follow-up, not part of this spec's required tasks — the executor may do it but it does
  not block acceptance.
- **New finding not in analysis plan**: `internal/cmd/init.go`'s `legacy-config-deprecated`
  warning message references the disappearing `specd migrate config` command and must be
  updated as part of this spec (task added; not present in the original plan's task list).

## Version Alignment Checklist

- No version-string literals live in the files this spec touches (`migrate.go`,
  `config_migrate.go`, `config_migrate_test.go`, `init.go`'s migrate branch). No action
  needed here; version alignment for user-facing docs mentioning `migrate` is covered by
  S4/S5.
