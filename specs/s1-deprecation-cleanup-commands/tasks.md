# S1 Tasks: Deprecation Cleanup — Commands

Dependencies: none (S1 is a Wave 1 spec — no other spec must land first).
Blocks: S4, S5 (docs reference this command surface), S6 (final CI gate needs S1 done).

---

## Wave 1 — Orphaned file deletion (no shared-code risk)

- [ ] **T1.1** Delete `internal/cmd/doctor.go`.
  - Evidence required before deleting: re-run `grep -rn "runDoctorCmd\|doctorCheck\|doctorHost\|doctorResult\|inspectDoctor\|repairDoctor" --include="*.go" .` and confirm every hit is inside `doctor.go`/`doctor_test.go` (this was true at investigation time; re-verify live, since this task executes after some time may have passed).
  - Dependencies: none.
  - Completion evidence: file absent; `go build ./...` still succeeds (will fail until T1.2 runs, since `doctor_test.go` still references it — do T1.1 and T1.2 together, verify build after both).
  - Rollback: `git checkout -- internal/cmd/doctor.go` (only if not yet committed).

- [ ] **T1.2** Delete `internal/cmd/doctor_test.go`.
  - Dependencies: T1.1 (do together, single commit).
  - Completion evidence: `go build ./... && go test ./internal/cmd/...` passes.

- [ ] **T1.3** Delete `internal/cmd/update.go`.
  - Evidence required: re-confirm `grep -rn "RunUpdate\|fetchChecksums\|releaseURL\|downloadBinary\|extractBinary" --include="*.go" .` shows only `update.go`/`update_test.go` hits.
  - Dependencies: none.
  - Completion evidence: file absent.

- [ ] **T1.4** Delete `internal/cmd/update_test.go`.
  - Dependencies: T1.3 (do together, single commit).
  - Completion evidence: `go build ./... && go test ./internal/cmd/...` passes.

- [ ] **T1.5** Delete `internal/cmd/registry_sunset_test.go`.
  - Dependencies: none (can run in parallel with T1.1-T1.4), but this file references
    `legacyAliases`, so it must be deleted no later than Wave 2 T2.1, or Wave 1's build
    will fail once `legacyAliases` is gone. Safe to delete now regardless.
  - Completion evidence: file absent.

**Wave 1 validation:** `go build ./...` (will still reference `legacyAliases`/`runMode` from
registry.go/mode.go — expected to still pass since those aren't touched yet in this wave;
if it fails, stop and diagnose before Wave 2).

---

## Wave 2 — Shared-code surgery (mode.go) and alias-table removal

- [ ] **T2.1** In `internal/cmd/mode.go`, delete only the `runMode` function (currently
  lines ~20-44 — re-locate by function signature `func runMode(args cli.Args) int`, do not
  assume line numbers are still accurate).
  - Verify before deleting: `grep -n "runModeSet(\|runModeRecommend(\|printMode(" internal/cmd/*.go` shows callers in `status.go` for the first two — keep those functions and their bodies untouched.
  - Dependencies: none directly, but logically pairs with T2.2 (registry.go alias removal) since `runMode` is only referenced from `legacyAliases`.
  - Completion evidence: `grep -n "^func runMode(" internal/cmd/mode.go` returns nothing; `grep -n "^func runModeSet\|^func runModeRecommend\|^func printMode" internal/cmd/mode.go` still returns 3 matches.

- [ ] **T2.2** In `internal/cmd/mode_cmd_test.go`, remove test cases that dispatch the bare
  `mode` command (i.e. call `Dispatch`/`Registry` with `"mode"` as the command name, not
  `status --set-mode`/`--recommend`). Keep all tests covering `runModeSet`/`runModeRecommend`
  behavior via their real surviving entrypoints.
  - Dependencies: T2.1.
  - Completion evidence: `go test ./internal/cmd/... -run TestMode` (or equivalent) passes;
    no test references a deleted `runMode` symbol.

- [ ] **T2.3** In `internal/cmd/registry.go`, delete (in order, so intermediate states still
  parse): `legacyAliases` map, `legacyAliasMeta` struct, `nextMinorVersion` const,
  `legacyAlias` function, `deprecationMessage` function, `terminalDeprecation` function.
  - Also delete the call site inside `Dispatch` (or wherever `Registry` lookup fails) that
    currently calls `legacyAlias(command)` before returning the unknown-command exit — locate
    it via `grep -n "legacyAlias(" internal/cmd/registry.go` (this call site, not the
    function definition) before deleting the function so you know exactly what to detach.
  - Dependencies: T1.1-T1.5, T2.1 (all symbols `legacyAliases.run` pointed to must be gone
    or unaffected first, though Go's compiler will simply catch any miss here).
  - Completion evidence: `go build ./...` succeeds.

- [ ] **T2.4** In `internal/cmd/registry_test.go`, remove the `c.DeprecatedIn != ""` skip
  clause in `TestRegistryMatchesHelp` (~line 30) since it is now always false.
  - Dependencies: T3.1 (commands.go must have DeprecatedIn entries removed first, or this
    test still needs the skip to pass in an intermediate state — run T3 before this, or
    verify test still passes either way before committing).
  - Completion evidence: `go test ./internal/cmd/... -run TestRegistryMatchesHelp` passes.

**Wave 2 validation:** `go build ./... && go vet ./...`.

---

## Wave 3 — Command metadata removal

- [ ] **T3.1** In `internal/core/commands.go`, delete the 13 `CommandMeta` entries for:
  `doctor`, `migrate`, `dispatch`, `mode`, `validate`, `schema`, `serve`, `replay`, `diff`,
  `watch`, `program`, `update`, `uninstall`. Locate each by `Command: "<name>"` field, not by
  the line numbers in the spec (re-verify live, since earlier deletions in this same file
  shift line numbers). Do NOT touch `fusion`, `version`, `mcp`, `help` entries.
  - Dependencies: none technically, but do after Wave 2 so `TestRegistryMatchesHelp` doesn't
    need its skip clause at all by the time you run it.
  - Completion evidence: `grep -c "DeprecatedIn:" internal/core/commands.go` returns `0`.

- [ ] **T3.2** In `internal/core/commands_palette_test.go`, remove the `c.DeprecatedIn != ""`
  skip clause in `TestFlagSingleOwner` (~line 25) since it is now always false.
  - Dependencies: T3.1.
  - Completion evidence: `go test ./internal/core/... -run TestFlagSingleOwner` passes.

- [ ] **T3.3** Run `TestPaletteCeiling` and confirm it still passes with the new (smaller)
  command count — do not adjust the ceiling constants unless the test fails; if it fails,
  investigate why the count changed unexpectedly before touching the assertion.
  - Dependencies: T3.1.
  - Completion evidence: `go test ./internal/core/... -run TestPaletteCeiling` passes.

**Wave 3 validation:** `go test ./... -race -count=1`.

---

## Wave 4 — Full regression and manual verification

- [ ] **T4.1** Run `go build ./...`, `go vet ./...`, `go test ./... -race -count=1`.
  - Completion evidence: all green.

- [ ] **T4.2** Manual dispatch check: run each of `go run . doctor`, `go run . dispatch`,
  `go run . program`, `go run . validate`, `go run . schema`, `go run . replay`,
  `go run . diff`, `go run . serve`, `go run . watch`, `go run . mode x`,
  `go run . migrate`, `go run . update`, `go run . uninstall` and confirm each exits 2 with
  a generic unknown-command message (not a deprecation warning).
  - Completion evidence: transcript of all 13 invocations pasted into the PR description or
    commit message.

- [ ] **T4.3** Manual survivor check: run `go run . next --dispatch --json` (or without
  `--json` if that requires a live spec — use `--help` if a real spec isn't set up),
  `go run . status --program`, `go run . check --schema`, `go run . report --history`,
  `go run . status <slug> --set-mode simple`, `go run . status <slug> --recommend` and
  confirm all still function (no panic, no "unknown flag", sensible output or expected
  gate/usage error given no spec context).
  - Completion evidence: transcript pasted into commit message; no regressions vs.
    pre-change behavior.

- [ ] **T4.4** `grep -rn "legacyAlias\|nextMinorVersion\|DeprecatedIn" internal/` returns
  nothing.
  - Completion evidence: empty grep output.

**Wave 4 validation (gate for moving to S4/S5/S6):** all of T4.1-T4.4 green.
