# S2 Tasks: Deprecation Cleanup — Config Migration

Dependencies: none required, but logically follows S1 (S1 removes the `migrate` alias
metadata; S2 removes the underlying handler and flag). Can run in parallel with S1 in
practice since they touch different files, but validate S1's `migrate` alias removal
together with S2's handler removal before declaring either fully done.

Blocks: S4, S6 (docs/CI need the full command surface finalized first).

---

## Wave 1 — Delete the migration command and its migration-only core functions

- [ ] **T1.1** Delete `internal/cmd/migrate.go` entirely.
  - Verify first: `grep -rn "RunMigrate(" --include="*.go" .` shows only `migrate.go`
    (definition) and `internal/cmd/init.go:98` (the one caller being removed in T1.3).
  - Dependencies: none.
  - Completion evidence: file absent.

- [ ] **T1.2** In `internal/core/config_migrate.go`, delete `MigrateConfigPreview` (currently
  ~line 123) and `MigrateConfigFile` (currently ~line 161) — re-locate by function
  signature, not line number. After deleting, check the file's import block for any import
  that was only used by these two functions (e.g. `os` for file rename) and remove unused
  imports. Do NOT delete `RenderConfigYAML`, `wKV`, `quoteYAML`, `ValidateConfigDoc`,
  `checkEnumDoc`, or `valueAtPath` — these are used by `init.go` and
  `config_validate.go` respectively.
  - Verify before: `grep -n "RenderConfigYAML(\|ValidateConfigDoc(" internal/cmd/init.go internal/core/config_validate.go` shows live callers outside this file.
  - Dependencies: T1.1 (do together so `go build` doesn't fail on a dangling caller mid-step).
  - Completion evidence: `go build ./internal/core/...` succeeds; `grep -n "^func MigrateConfigPreview\|^func MigrateConfigFile" internal/core/config_migrate.go` returns nothing; `grep -n "^func RenderConfigYAML\|^func ValidateConfigDoc" internal/core/config_migrate.go` still returns both.

- [ ] **T1.3** In `internal/core/config_migrate_test.go`, delete `TestMigrateConfigPreviewAndFile`
  (currently ~line 11). Keep `TestConfigYAMLJSONCompatibility` and
  `TestEnsureGlobalConfigScaffold`.
  - Dependencies: T1.2.
  - Completion evidence: `go test ./internal/core/... -run TestConfigYAMLJSONCompatibility|TestEnsureGlobalConfigScaffold` passes; `go build ./internal/core/...` succeeds (no dangling references to deleted functions in the test file).

**Wave 1 validation:** `go build ./...` (expected to fail until Wave 2 removes the
`init.go` caller — if it fails only on `internal/cmd/init.go:98`'s `RunMigrate` reference,
that is expected; proceed to Wave 2 immediately).

---

## Wave 2 — Remove the `--migrate` flag from `init`

- [ ] **T2.1** In `internal/cmd/init.go`, remove the `--migrate` branch from `RunInit`
  (currently lines ~96-99):
  ```go
  if args.Bool("migrate") {
      migrateArgs := args
      migrateArgs.Pos = []string{"config"}
      return RunMigrate(migrateArgs)
  }
  ```
  Also edit the doc comment directly above `RunInit` (currently: *"RunInit implements
  `specd init`. With --migrate it delegates to RunMigrate to move the legacy JSON config to
  YAML; otherwise it runs the workspace onboarding flow..."*) to drop the `--migrate` clause.
  - Dependencies: T1.1 (RunMigrate must not be referenced once deleted).
  - Completion evidence: `go build ./...` succeeds.

- [ ] **T2.2** In `internal/cmd/init.go` (currently ~line 514), update the
  `legacy-config-deprecated` warning message from `"config.json is deprecated; run specd
  migrate config to convert to config.yml."` to remove the now-nonexistent command
  suggestion — e.g. `"config.json is deprecated; convert it to config.yml manually (see
  docs/user-guide.md) or continue using JSON."` (exact wording is a documentation-quality
  call, not a contract — pick wording consistent with the rest of `init.go`'s warning
  style, and make sure any doc referenced by the new message actually contains conversion
  guidance — see S5 for the doc-side follow-up).
  - Dependencies: none (independent of T2.1, but same file — do in the same commit).
  - Completion evidence: `grep -n "migrate config" internal/cmd/init.go` returns nothing.

- [ ] **T2.3** In `internal/core/commands.go`, edit the `init` `CommandMeta` entry:
  - Remove `[--migrate]` from the `Usage`/`Synopsis` strings.
  - Remove the `{Name: "migrate", Type: "boolean", Description: "Migrate legacy config.json to config.yml"}` entry from its `Flags` slice.
  - Dependencies: T2.1 (flag removed from source of truth after behavior is gone).
  - Completion evidence: `grep -n '"migrate"' internal/core/commands.go` returns nothing
    (note: run this after S1's Wave 3 also removes the standalone `migrate` `CommandMeta`
    entry, so the only pre-S1 hit left here should be `init`'s flag — confirm S1 landed
    first if this grep still shows the standalone entry).

**Wave 2 validation:** `go build ./... && go vet ./... && go test ./... -race -count=1`.

---

## Wave 3 — Regression and manual verification

- [ ] **T3.1** Run full test suite: `go test ./... -race -count=1`.
  - Completion evidence: all green.

- [ ] **T3.2** Manual check, in a scratch temp directory (not this repo): run
  `go run . init --migrate` (built from the modified source) and confirm it returns exit 2
  (usage error for unknown flag), not a panic or silent no-op.
  - Completion evidence: transcript pasted into commit message.

- [ ] **T3.3** Manual check: with a scratch project containing a legacy `config.json` and no
  `config.yml`, run `go run . init` (no flags) and confirm the `legacy-config-deprecated`
  warning now prints the updated message from T2.2 (no reference to a nonexistent
  `migrate config` command).
  - Completion evidence: transcript pasted into commit message.

- [ ] **T3.4** `grep -rn "RunMigrate\|MigrateConfigPreview\|MigrateConfigFile" internal/`
  returns nothing.
  - Completion evidence: empty grep output.

**Wave 3 validation (gate for moving to S4/S5/S6):** all of T3.1-T3.4 green.
