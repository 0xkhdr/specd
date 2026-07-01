# S1: Deprecation Cleanup — Commands

## Purpose and Requirement Coverage

Remove the deprecated top-level command surface (13 legacy aliases) and its supporting
infrastructure from `internal/cmd/registry.go` and `internal/core/commands.go`, so that
v0.1.0 hardening ships with zero deprecated commands, zero legacy-alias plumbing, and zero
orphaned handler code.

Covers: **R1** (remove deprecated commands/aliases), **R2** (remove `update` handler),
**R13** (remove `doctor` handler), and contributes evidence to **R16** (no version-gated
old-command handling).

## Verified Current State

This section reflects the *live* repository, not the analysis plan's assumptions. Two
material corrections vs. the plan are called out explicitly (see "Deviations" below).

### `internal/cmd/registry.go`

- `const nextMinorVersion = "v0.2.0"` — `registry.go:65`.
- `type legacyAliasMeta struct { home, removedIn, functional, run }` — `registry.go:86-91`.
- `var legacyAliases map[string]legacyAliasMeta` — `registry.go:97-118`, 13 entries:

  | Alias | Home | removedIn | functional | run |
  |---|---|---|---|---|
  | `doctor` | `specd init --repair` | v0.2.0 | true | `runDoctorCmd` |
  | `dispatch` | `specd next --dispatch` | v0.2.0 | true | `runDispatch` |
  | `program` | `specd status --program` | v0.2.0 | true | `runProgram` |
  | `validate` | `specd check --schema-only` | v0.2.0 | true | `runValidate` |
  | `schema` | `specd check --schema` | v0.2.0 | true | `runSchema` |
  | `replay` | `specd report --history` | v0.2.0 | true | `runReplay` |
  | `diff` | `specd report --diff` | v0.2.0 | true | `runDiff` |
  | `serve` | `specd report --serve` | v0.2.0 | true | `runServe` |
  | `watch` | `specd report --watch` | v0.2.0 | true | `runWatch` |
  | `mode` | `specd status <slug> --set-mode\|--recommend, specd new --orchestrated` | **v0.3.0** | true | `runMode` |
  | `migrate` | `specd init --migrate` | v0.2.0 | **false** | — |
  | `update` | `scripts/install.sh or your package manager` | v0.2.0 | **false** | — |
  | `uninstall` | `scripts/uninstall.sh or your package manager` | v0.2.0 | **false** | — |

- Helper functions, all to be deleted: `legacyAlias` (`registry.go:120-134`),
  `deprecationMessage` (`registry.go:138-140`), `terminalDeprecation`
  (`registry.go:145-161`).
- The block carries an existing code comment (`registry.go:60-64`) stating: *"At this
  release the corresponding entries in legacyAliases are deleted so the old top-level
  names fall through to the unknown-command help path"* — this cleanup is anticipated by
  the code's own authors, not a novel departure.
- A second comment (`registry.go:78-85`) states explicitly that for `doctor`, "the
  survivor does NOT preserve full capability (doctor's diagnostics ≠ `init --repair`)" —
  i.e. removing `doctor` is a **known, accepted capability loss**, not an oversight.

### `internal/core/commands.go`

- `CommandMeta` struct has `Hidden bool` and `DeprecatedIn string` fields (`commands.go:61-62`).
- 13 entries carry `DeprecatedIn: "v0.2.0"` (all of them, including `mode` — this is the
  deprecation-warning-start version, a different field from `registry.go`'s `removedIn`,
  which for `mode` is `v0.3.0`; this is **not a contradiction**, see Deviations).
- Exact line numbers of the 13 deprecated `CommandMeta` entries (block start lines):
  `doctor` (81), `migrate` (91), `dispatch` (173), `mode` (215), `validate` (245),
  `schema` (256), `serve` (277), `replay` (287), `diff` (297), `watch` (308),
  `program` (395), `update` (406), `uninstall` (416).
- Three other `Hidden: true` entries are **not deprecated** and must NOT be touched:
  `fusion` (102), `version` (431), `mcp` (440), `help` (455).

### Handler ownership — critical distinction

Investigation (not assumed from the analysis plan) established that the ten
`functional: true` aliases split into two very different categories:

1. **Shared implementation — 8 commands.** `runDispatch`, `runProgram`, `runValidate`,
   `runSchema`, `runReplay`, `runDiff`, `runServe`, `runWatch` are called from **both**
   `legacyAliases` **and** the canonical v0.1.0 surface:
   - `internal/cmd/next.go` calls `runDispatch` for `next --dispatch`.
   - `internal/cmd/status.go` calls `runProgram` for `status --program`.
   - `internal/cmd/check.go` calls `runValidate`/`runSchema` for `check --schema-only`/`check --schema`.
   - `internal/cmd/report.go` calls `runReplay`/`runDiff`/`runServe`/`runWatch` for
     `report --history`/`--diff`/`--serve`/`--watch`.
   - **These 8 handler files (`dispatch.go`, `program.go`, `validate.go`, `schema.go`,
     `replay.go`, `diff.go`, `serve.go`, `watch.go`) MUST NOT be deleted or have their
     `run*` functions removed.** Only the alias registration (the ability to invoke them
     via the bare old command name) is being removed.

2. **Genuinely orphaned — `doctor` and `mode`.**
   - `runDoctorCmd` (`internal/cmd/doctor.go:65`) has **no caller anywhere** except its own
     `legacyAliases` entry. No function in `doctor.go` is called from `init.go` or any
     other file (`init.go`'s `--repair` path is an independent implementation). **The
     entire `doctor.go` file is dead once the alias is removed** and must be deleted.
   - `runMode` (`internal/cmd/mode.go:20-44`) is called **only** from its own
     `legacyAliases` entry. However, `mode.go` also defines `runModeSet` and
     `runModeRecommend`, which **are** called externally from `internal/cmd/status.go`
     (`status --set-mode`, `status --recommend`), and `printMode`, called from within
     `runMode`, `runModeSet`, and `runModeRecommend`. **Only the `runMode` function itself
     is deleted; `runModeSet`, `runModeRecommend`, `printMode`, and `modePayload` are kept.**

3. **No handler at all — `migrate`, `update`, `uninstall`.** These three are
   `functional: false` in `legacyAliases` — invoking the bare command name only ever
   prints a deprecation warning and exits non-zero (`terminalDeprecation`); no handler
   function is wired into the alias path for them.
   - `internal/cmd/update.go` (232 lines) defines `RunUpdate`, `fetchChecksums`,
     `releaseURL`, `downloadBinary`, `extractBinary`. Grep confirms **zero callers**
     outside `update.go` and `update_test.go` — this file is 100% dead code today and can
     be deleted outright (this is R2).
   - `migrate`'s handler (`RunMigrate`, in `internal/cmd/migrate.go`) is real and IS called
     — but from `internal/cmd/init.go`'s `--migrate` flag, not from the alias table. Its
     removal is **S2's** responsibility, not S1's (S1 only removes the `migrate` alias
     entry/metadata; the `RunMigrate` function and its `init.go` wiring survive S1 and are
     deleted in S2).
   - `uninstall` has no Go handler at all; only the alias metadata and the
     `scripts/uninstall.sh` file exist. The script's removal is **S3's** responsibility.

### Tests referencing this surface

- `internal/cmd/registry_sunset_test.go` (104 lines) — `TestLegacyAliasSunset` iterates
  `legacyAliases` and asserts sunset invariants. **Delete this entire file** — its subject
  (`legacyAliases`) no longer exists.
- `internal/cmd/registry_test.go` — `TestRegistryMatchesHelp` (`registry_test.go:19-46`)
  already `continue`s past `c.DeprecatedIn != ""` entries (line 30) when comparing
  `Registry` against `core.Commands`; once no entries carry `DeprecatedIn`, this skip
  clause becomes a no-op (harmless to leave, but delete for clarity per "no dead code").
  `TestRegistryHandlersNonNil`, `TestDispatchUnknownCommand`,
  `TestEveryRegisteredCommandHasHelp` are unaffected.
- `internal/core/commands_palette_test.go` — `TestFlagSingleOwner` (15-48) skips
  `c.DeprecatedIn != ""` entries (line 25); same no-op cleanup applies.
  `TestPaletteCeiling` (50-67) asserts ≤16 non-deprecated, non-hidden commands and ≤20
  non-deprecated commands total — after removal, all remaining entries are non-deprecated
  by construction, so these ceilings need re-verification (the palette should be smaller
  post-removal, not larger, so the ceiling check is expected to keep passing, but must be
  run, not assumed).
- `internal/cmd/doctor_test.go` (7.1K) — delete (tests the deleted file).
- `internal/cmd/mode_cmd_test.go` (5.8K) — **audit, do not blanket-delete.** Tests may cover
  `runModeSet`/`runModeRecommend`/`printMode` (kept) as well as `runMode` (deleted). Only
  remove test cases that exclusively exercise the deleted `runMode` entrypoint (i.e. tests
  that invoke the bare `mode` command through `Dispatch`/`Registry`, not `status --set-mode`
  style tests).
- `internal/cmd/update_test.go` (3.4K) — delete (tests the deleted file).

## Proposed Design and End-to-End Flow

1. Delete `internal/cmd/doctor.go` and `internal/cmd/doctor_test.go`.
2. Delete `internal/cmd/update.go` and `internal/cmd/update_test.go`.
3. In `internal/cmd/mode.go`, delete only the `runMode` function (`mode.go:20-44`); keep
   everything else in the file.
4. In `internal/cmd/registry.go`:
   - Delete `nextMinorVersion`, `legacyAliasMeta`, `legacyAliases`, `legacyAlias`,
     `deprecationMessage`, `terminalDeprecation`.
   - Delete whatever call site in `Dispatch` (or equivalent) currently checks
     `legacyAlias(command)` before falling through to unknown-command handling — verify the
     exact call site by reading the file fresh at execution time (this spec does not
     invent unread line numbers for it).
5. In `internal/core/commands.go`, delete the 13 `CommandMeta` entries listed above by
   `Command` field (`doctor`, `migrate`, `dispatch`, `mode`, `validate`, `schema`, `serve`,
   `replay`, `diff`, `watch`, `program`, `update`, `uninstall`). Leave `fusion`, `version`,
   `mcp`, `help` untouched.
6. Delete `internal/cmd/registry_sunset_test.go`.
7. Update `internal/cmd/registry_test.go` and `internal/core/commands_palette_test.go` to
   remove the now-dead `DeprecatedIn != ""` skip clauses.
8. In `internal/cmd/mode_cmd_test.go`, remove only the test cases exercising bare `mode`
   dispatch; keep `--set-mode`/`--recommend` coverage.
9. Build and test after every step, not just at the end (`go build ./...` is cheap; run it
   after each deletion to localize break points).

## Interfaces, Contracts, Data, Configuration, Dependencies

- `cmd.Dispatch`'s contract for all **surviving** commands (including `next --dispatch`,
  `status --program`, `check --schema`/`--schema-only`, `report --history/--diff/--serve/--watch`,
  `status --set-mode`/`--recommend`) is unchanged — same flags, same exit codes, same output.
- `core.Commands` shrinks from its current size by exactly 13 entries.
- No `state.json` or config schema changes.
- No new external dependencies; this is pure deletion.

## Invariants, Security, Errors, Observability, Compatibility, Rollback

- **INV6** (`Registry`/`Commands` parity via `TestRegistryMatchesHelp`) must still pass —
  since both the alias entries and the metadata entries are removed together, parity is
  preserved by construction.
- **Security**: none of these deletions touch atomic writes, CAS, or exit-code contracts
  (INV2-INV4 preserved).
- **Compatibility (breaking, intentional)**: after this change, running `specd doctor`,
  `specd dispatch`, `specd program`, `specd validate`, `specd schema`, `specd replay`,
  `specd diff`, `specd serve`, `specd watch`, `specd mode`, `specd migrate`, `specd update`,
  or `specd uninstall` as a bare top-level command returns the generic unknown-command exit
  (2), with no deprecation warning — because the fallback path is gone entirely. This must
  be called out in release notes (S6/CHANGELOG scope).
- **Known, accepted capability loss**: `specd doctor`'s diagnostics (scaffold/MCP/host
  registration health checks with `--fix`) have no surviving equivalent — `init --repair`
  does not replicate them (confirmed by the codebase's own comment at `registry.go:78-85`
  and by the absence of any `doctor.go` function call from `init.go`). This must be stated
  plainly in release notes, not silently dropped.
- **Rollback**: this spec's changes are pure deletions plus metadata edits; rollback is a
  `git revert` of the commit(s) implementing this spec. No data migration is involved.

## Acceptance Criteria and Validation Commands

- `go build ./...` succeeds.
- `go vet ./...` succeeds.
- `go test ./... -race -count=1` passes.
- `grep -rn "legacyAlias\|nextMinorVersion\|DeprecatedIn" internal/` returns nothing.
- `grep -rn "runDoctorCmd\|runMode\b" internal/cmd/*.go` returns nothing (note: `runModeSet`,
  `runModeRecommend` must still be found — verify they were NOT accidentally deleted).
- `grep -c "Command:" internal/core/commands.go` decreases by exactly 13 vs. the pre-change
  count.
- `TestRegistryMatchesHelp`, `TestPaletteCeiling`, `TestFlagSingleOwner` all pass.
- Manual check: `go run . dispatch`, `go run . doctor`, `go run . mode x` (and the other 10
  removed names) all return exit code 2 with a generic "unknown command" message (not a
  deprecation warning).
- Manual check: `go run . next --dispatch`, `go run . status --program`,
  `go run . check --schema`, `go run . report --history` still work exactly as before
  (these prove the shared-handler files were not broken).

## Open Decisions and Deviations

- **Deviation from analysis plan F1/F2/Implementation Vision §6**: the plan implies all ten
  functional alias handlers are deprecated implementation to be deleted. Verified repo
  evidence shows only `doctor.go` (whole file) and `mode.go`'s `runMode` (one function) are
  actually orphaned; the other 8 handler files are load-bearing for the canonical v0.1.0
  command surface and must be preserved. Deleting them would be a functional regression,
  not a cleanup — flagged here rather than silently deviating.
- **Deviation from analysis plan §1 assumption A1 / D4**: the plan assumes v0.1.0 has never
  been released. The repo has a `v0.1.0` git tag (2026-06-14) and a `CHANGELOG.md` entry for
  it already. Per explicit user instruction, this spec proceeds under the v0.1.0 framing
  anyway; the tag will need to be re-cut or amended at actual release time (out of scope for
  spec authoring — tracked in `specs/progress.md`).
- **Not a bug**: `mode`'s `DeprecatedIn: "v0.2.0"` (commands.go) vs. `removedIn: "v0.3.0"`
  (registry.go) are two different fields with two different meanings (when the warning
  started vs. when the alias is deleted), not a data inconsistency. Both are being removed
  now regardless, ahead of their originally recorded schedule, because this cleanup targets
  "zero deprecated commands," not "commands past their `removedIn` date."

## Version Alignment Checklist

- This spec's scope is Go source and tests, not documentation — no `--version` install
  examples live here.
- `nextMinorVersion = "v0.2.0"` and the `"v0.3.0"` literal in `registry.go` are deleted
  entirely as part of this spec, so no v0.2.0/v0.3.0 string survives in `internal/cmd/` or
  `internal/core/` after S1. Verify: `grep -rn "v0\.2\.0\|v0\.3\.0" internal/` returns
  nothing after this spec completes.
