# Analysis and Action Plan: specd v0.1.0 Development Release — Deprecation Cleanup & Hardening

## 1. Intent and Success

**Requested outcome:** Prepare the `specd` repository for its **first development release (v0.1.0)** by eliminating all deprecated code, commands, documentation, and migration paths that exist only to support pre-release iterations. Harden the codebase and align documentation for the v0.1.0 release state. This release captures the current stable snapshot before proceeding with the project roadmap.

**Task class:** Hardening + Deprecation purge + Documentation alignment.

**Constraints:**
- Zero runtime dependencies (`go.mod` must remain stdlib-only).
- All existing tests must continue to pass (`make ci` clean).
- No behavioral regression for non-deprecated paths.
- Windows build must remain intact (known `specd update` limitation documented, not silently broken).

**Measurable success:**
- `make ci` passes on Linux, macOS, and Windows (build-only on Windows).
- No deprecated commands, flags, or old-version handling code remains in the source tree.
- `go.mod` has zero `require` entries.
- All documentation references the v0.1.0 command surface only.
- Release artifacts are produced by GoReleaser with `SHA256SUMS`.
- Install examples in docs reference `--version 0.1.0`.

---

## 2. Scope and Decisions

### In scope
- Remove deprecated CLI commands and their handlers (`doctor`, `dispatch`, `program`, `validate`, `schema`, `replay`, `diff`, `serve`, `watch`, `mode`, `migrate`, `update`, `uninstall`).
- Remove legacy alias infrastructure from `internal/cmd/registry.go`.
- Remove deprecated command metadata from `internal/core/commands.go`.
- Remove `internal/cmd/update.go` (self-update binary) and its tests.
- Remove legacy JSON config migration path (`internal/core/config_migrate.go` and `internal/cmd/` migration handler).
- Remove `scripts/uninstall.sh` (superseded by install script `--force` / package managers).
- Remove `AGENTS.md` references to retired commands and the `RTK` section (appears to be an artifact from another project).
- Update `README.md` to reflect v0.1.0 surface — no deprecated commands, no old-version handling.
- Update all `docs/*.md` files to remove deprecated command references and old-version handling.
- Update `SECURITY.md` to reflect v0.1.0 as current supported version.
- Update `TESTING.md` to remove coverage-gap deferrals that are now resolved or no longer relevant.
- Update all install examples to reference `--version 0.1.0`.
- Harden: add `govulncheck` to CI (already present), ensure `staticcheck` passes, verify no dead code.

### Out of scope
- New features or commands.
- Changes to the core spec lifecycle, validation gates, or state machine.
- Changes to the Brain/Pinky orchestration model.
- Changes to the MCP server contract.
- External dependency additions.
- Post-v0.1.0 roadmap items.

### Assumptions
- A1: The project has never been released; no user depends on deprecated aliases.
- A2: The `install.sh` script is the canonical installation path; `uninstall.sh` and `update` are safe to remove.
- A3: `AGENTS.md` RTK section is copy-paste artifact from another repo and should be removed.
- A4: `docs/` files contain scattered references to deprecated commands and old versions that need scrubbing.
- A5: v0.1.0 is the correct first release version (development stage, per user decision).

### Decision gates
- D1: Confirm `uninstall.sh` is unused in any CI or docs before deleting.
- D2: Confirm `update.go` has no callers outside its own command handler before deleting.
- D3: Verify `AGENTS.md` RTK section is not referenced by any template in `internal/core/embed_templates/`.
- D4: **Version numbering confirmed — v0.1.0 as first development release.** All docs, examples, and version strings must reference v0.1.0. No v1.0.0 references should exist.

---

## 3. Repository Context

**Stack:** Go (stdlib only), zero external dependencies. Single binary CLI.

**Architecture:**
- `main.go` — entry point, arg router, `--json` bridging, `help`/`version`/`mcp` bypass.
- `internal/cli/args.go` — custom ~40-line flag parser (no Cobra).
- `internal/cmd/` — one file per command + `registry.go` dispatch table + `helpers.go`.
- `internal/core/` — domain logic: paths, IO, lock, state, phases, DAG, EARS, tasks parser, spec files, config, embed templates.
- `internal/testharness/` — deterministic test infrastructure.
- `scripts/` — `install.sh`, `coverage-check.sh`, `stress.sh`, `test-lint.sh`, etc.
- `docs/` — 11 markdown docs.
- `.github/workflows/ci.yml` — CI matrix (lint, test, coverage, stress, build).
- `.goreleaser.yml` — release automation.

**Key invariants (must be preserved):**
- INV1: `go.mod` has zero `require` entries. `E1: go.mod` (observed as stdlib-only in contributor guide).
- INV2: Exit code contract: `0=ok`, `1=gate/validation`, `2=usage`, `3=not-found`. `E2: internal/core/exit.go`.
- INV3: Atomic writes via `core.AtomicWrite` (temp + fsync + rename). `E3: internal/core/io.go`.
- INV4: CAS on `state.json` revision for concurrent mutation safety. `E4: internal/core/state.go`.
- INV5: `ParseTasksMd` round-trip byte stability. `E5: internal/core/tasksparser.go`.
- INV6: `Registry` and `Commands` parity enforced by `TestRegistryMatchesHelp`. `E6: internal/cmd/registry.go`.
- INV7: `SHA256SUMS` filename is load-bearing across `.goreleaser.yml`, `update.go`, and `install.sh`. `E7: .goreleaser.yml`, `internal/cmd/update.go`, `scripts/install.sh`.

**Analogous implementations:**
- The `legacyAliases` map in `registry.go` is the single source of deprecated command handling. Its removal pattern is straightforward: delete entries, delete handlers, delete tests.
- The `commands.go` metadata mirrors `registry.go`; both must be kept in sync per INV6.

---

## 4. Requirements and Evidence

| ID | Requirement or Fact | Evidence/Source | Priority | Acceptance Signal |
|----|---------------------|-----------------|----------|-------------------|
| R1 | Remove all deprecated CLI commands and aliases | `internal/cmd/registry.go` `legacyAliases` map; `commands.go` Hidden/DeprecatedIn fields | P0 | `grep -r 'deprecated\|DeprecatedIn\|legacyAlias' internal/` returns nothing |
| R2 | Remove `update` command handler and tests | `internal/cmd/update.go` | P0 | File deleted; `make build` passes |
| R3 | Remove `uninstall.sh` script | `scripts/uninstall.sh` | P0 | File deleted; no CI/docs reference |
| R4 | Remove legacy config migration (`migrate` command + `config_migrate.go`) | `internal/core/config_migrate.go`; `commands.go` "migrate" entry | P0 | Files/handlers deleted; `make test` passes |
| R5 | Scrub all docs of deprecated command references | `docs/*.md`, `README.md`, `AGENTS.md`, `TESTING.md`, `SECURITY.md` | P0 | No deprecated command names found in docs |
| R6 | Remove `AGENTS.md` RTK artifact section | `AGENTS.md` tail section | P0 | Section removed; no template references it |
| R7 | Update `README.md` for v0.1.0 (remove old-version handling, update install examples) | `README.md` | P0 | Install examples use `--version 0.1.0`; no old-version caveats |
| R8 | Update `SECURITY.md` to reflect v0.1.0 as supported version | `SECURITY.md` | P1 | Security policy references v0.1.0; no pre-1.0 language |
| R9 | Ensure `make ci` passes after all removals | `Makefile`, `.github/workflows/ci.yml` | P0 | `make ci` clean on Linux/macOS; `go build` on Windows |
| R10 | Preserve zero-runtime-dependency invariant | `go.mod` | P0 | `go.mod` still has zero `require` entries |
| R11 | Preserve exit-code contract | `internal/core/exit.go` | P0 | All tests pass; no exit code changes |
| R12 | Preserve SHA256SUMS filename contract | `.goreleaser.yml`, `scripts/install.sh` | P0 | Filename unchanged; install script still verifies |
| R13 | Remove `doctor` command handler and tests | `internal/cmd/` (handler referenced in `legacyAliases`) | P0 | Handler deleted; tests updated |
| R14 | Update all version references to v0.1.0 | `README.md`, `SECURITY.md`, `docs/*.md`, `AGENTS.md`, `TESTING.md` | P0 | `grep -r 'v1\.0\.0\|v0\.2\.0\|v0\.3\.0' docs/ README.md AGENTS.md SECURITY.md TESTING.md` returns nothing (except historical changelog if any) |
| R15 | Update install script examples to use `--version 0.1.0` | `README.md`, `docs/user-guide.md`, `docs/agent-integration.md` | P0 | All install examples reference `0.1.0` |
| R16 | Remove old-version handling code (any code that branches on version) | `internal/core/`, `internal/cmd/` | P0 | No version-gated behavior for non-existent old versions |

---

## 5. Findings and Impact

| ID | Finding/Constraint | Evidence | Impact | Recommendation |
|----|--------------------|----------|--------|----------------|
| F1 | `legacyAliases` in `registry.go` contains 13 deprecated commands with removal target `v0.2.0` or `v0.3.0` | `internal/cmd/registry.go` lines 80–130 | These aliases bloat the dispatch table and confuse documentation | Remove all entries; delete associated handler functions and tests |
| F2 | `commands.go` still carries metadata for `doctor`, `migrate`, `dispatch`, `validate`, `schema`, `replay`, `diff`, `serve`, `watch`, `mode`, `program`, `update`, `uninstall` | `internal/core/commands.go` | Metadata drift risk per INV6; docs cite these as "hidden" | Delete all `CommandMeta` entries with `DeprecatedIn` or `Hidden` that correspond to removed aliases |
| F3 | `update.go` implements self-update binary download with SHA256 verification | `internal/cmd/update.go` | Self-update is deprecated; install script is the canonical path | Delete `update.go` and its tests; update `README.md` to reference install script only |
| F4 | `config_migrate.go` implements JSON→YAML migration | `internal/core/config_migrate.go` | Pre-release artifact; no released version to migrate from | Delete `config_migrate.go` and `RunMigrate` handler; remove from `init.go` `--migrate` branch |
| F5 | `AGENTS.md` contains a large `RTK (Rust Token Killer)` section at the bottom | `AGENTS.md` tail | Appears to be a copy-paste artifact from another project; unrelated to specd | Remove entire RTK section |
| F6 | `README.md` describes `update`/`uninstall` as CLI commands and uses `--version 0.1.0` in install examples | `README.md` "Update" section; install examples | Confuses users; version example is correct but needs verification | Rewrite to reference `scripts/install.sh` only; keep `--version 0.1.0` example; remove `specd update`/`uninstall` mentions |
| F7 | `docs/command-reference.md` has a "Migration appendix" listing old→new command mappings | `docs/command-reference.md` | Useful for pre-release users, irrelevant for v0.1.0 | Remove migration appendix; add a note that v0.1.0+ has no deprecated commands |
| F8 | `docs/validation-gates.md` references `doctor` in security model | `docs/validation-gates.md` | Stale reference | Replace with `init --repair` or remove |
| F9 | `TESTING.md` references `COVERAGE_GAPS.md` and dark-path inventory | `TESTING.md` | May contain resolved gaps; needs audit | Review and update or remove if all gaps are now covered |
| F10 | `install.sh` references `uninstall.sh` indirectly? | `scripts/install.sh` | Need to verify | Search for `uninstall` references in `install.sh` |
| F11 | `Makefile` has `make install` target that uses `go install` | `Makefile` | Valid for dev builds; keep | No action |
| F12 | `.goreleaser.yml` generates `SHA256SUMS` — filename is load-bearing | `.goreleaser.yml` | Must not change | No action (preserve) |
| F13 | `internal/cmd/init.go` has `--migrate` branch calling `RunMigrate` | `internal/cmd/init.go` | Will break when `RunMigrate` is removed | Remove `--migrate` flag handling from `init.go` |
| F14 | `README.md` and docs may contain `v0.2.0` or `v0.3.0` references from deprecation targets | `README.md`, `docs/*.md` | Confuses users about what version they're installing | Scrub all old version references; ensure only v0.1.0 is referenced |
| F15 | `SECURITY.md` says "specd is pre-1.0" | `SECURITY.md` | Outdated for v0.1.0 release | Update to reflect v0.1.0 as current supported version |

---

## 6. Implementation Vision

**Design direction:** Surgical removal of deprecated surface + documentation alignment for v0.1.0. No architectural changes.

**Affected modules:**
- `internal/cmd/registry.go` — delete `legacyAliases`, `legacyAlias`, `deprecationMessage`, `terminalDeprecation`, `nextMinorVersion`.
- `internal/cmd/` — delete `update.go`, `update_test.go` (if exists); delete `doctor` handler references; delete `runMode`, `runDispatch`, `runProgram`, `runValidate`, `runSchema`, `runReplay`, `runDiff`, `runServe`, `runWatch` handler stubs.
- `internal/core/commands.go` — delete `CommandMeta` entries for all deprecated commands.
- `internal/core/config_migrate.go` — delete entire file.
- `internal/cmd/init.go` — remove `--migrate` branch.
- `scripts/uninstall.sh` — delete.
- `AGENTS.md` — remove RTK section; remove retired command references.
- `README.md` — rewrite install/update/uninstall sections; ensure `--version 0.1.0` in examples; remove old-version handling language.
- `docs/command-reference.md` — remove migration appendix; update command list.
- `docs/user-guide.md` — remove deprecated command mentions (e.g., `doctor` → `init --repair` already done, but verify no stale references).
- `docs/validation-gates.md` — remove `doctor` references.
- `docs/agent-integration.md` — remove deprecated command references; update install examples to `0.1.0`.
- `SECURITY.md` — update to v0.1.0 as supported version; remove `update`/`uninstall` CLI references.
- `TESTING.md` — audit and update coverage gaps reference.

**Interfaces/contracts:**
- `cmd.Dispatch` contract unchanged for surviving commands.
- `core.Commands` slice shrinks; `TestRegistryMatchesHelp` must still pass.
- Exit codes unchanged.

**Data/configuration:**
- No `state.json` schema changes.
- No config format changes (YAML v2 remains; JSON read support stays for backward compatibility of config files, but migration tool is removed).

**Security/failures:**
- Removing `update.go` eliminates a network-download path from the binary surface — a security win.
- Removing `uninstall.sh` eliminates a script that mutates user shell configs.

**Observability:**
- No changes to reporting, metrics, or logging.

**Compatibility:**
- This is a **breaking change** by design: v0.1.0 drops all pre-release compatibility shims.
- Release notes must document the removed commands and their replacements.

**Rollout:**
- Tag `v0.1.0` after merge.
- GoReleaser produces artifacts.
- `install.sh` already points to latest release; update examples to reference `0.1.0`.

**Rollback:**
- If critical issue found post-release, tag `v0.1.1` with fix; no rollback of deprecation removals intended.

---

## 7. Specification Map

| Spec | Responsibility | Requirements | Dependencies | Validation |
|------|----------------|--------------|--------------|------------|
| S1: Deprecation Cleanup — Commands | Remove deprecated CLI aliases, handlers, metadata | R1, R2, R13 | None | `make test`, `TestRegistryMatchesHelp` |
| S2: Deprecation Cleanup — Config Migration | Remove JSON→YAML migration tool and `--migrate` flag | R4 | S1 (shares `init.go` touch point) | `make test`, `go build` |
| S3: Deprecation Cleanup — Scripts | Remove `uninstall.sh`; verify `install.sh` has no references | R3 | None | `shellcheck scripts/*.sh`, grep for `uninstall` |
| S4: Documentation Alignment — Root Docs | Update `README.md`, `AGENTS.md`, `SECURITY.md`, `TESTING.md` | R5, R6, R7, R8, R14, R15 | S1, S2, S3 | Manual review + grep for deprecated names and old versions |
| S5: Documentation Alignment — Guide Docs | Update `docs/*.md` to remove deprecated references and old versions | R5, R14, R15 | S1 | Manual review + grep |
| S6: Hardening — CI & Static Analysis | Verify `make ci` passes, `govulncheck` clean, no dead code | R9, R10, R11, R12, R16 | S1–S5 | `make ci` on all platforms |

---

## 8. Execution and Validation Plan

### Phase 1: Code Removal (parallel where possible)
1.1 Delete `internal/cmd/update.go` and any `update_test.go`.  
1.2 Delete `internal/core/config_migrate.go`.  
1.3 Delete `scripts/uninstall.sh`.  
1.4 In `internal/cmd/registry.go`: remove `legacyAliases`, `legacyAlias`, `deprecationMessage`, `terminalDeprecation`, `nextMinorVersion`. Remove `runDoctorCmd`, `runDispatch`, `runProgram`, `runValidate`, `runSchema`, `runReplay`, `runDiff`, `runServe`, `runWatch`, `runMode` stubs (verify they exist as unexported functions or are inline in `legacyAliases`).  
1.5 In `internal/core/commands.go`: delete all `CommandMeta` entries with `DeprecatedIn` set or `Hidden=true` for commands being removed.  
1.6 In `internal/cmd/init.go`: remove `--migrate` branch and `RunMigrate` call.  

**Validation:** `go build ./...`, `go test ./... -race -count=1`.

### Phase 2: Test Updates
2.1 Update `internal/cmd/registry_test.go` (or equivalent) to remove legacy alias tests.  
2.2 Update `internal/cmd/commands_test.go` to remove tests for deleted commands.  
2.3 Update any test that asserts on `Commands` slice length or contents.  
2.4 Run `make test-order` (`-count=2`) to catch iteration dependence.  

**Validation:** `make test`, `make test-order`, `make cover-check`.

### Phase 3: Documentation Alignment for v0.1.0
3.1 `README.md`: 
   - Rewrite Installation/Update/Uninstall to reference `scripts/install.sh` only.
   - Ensure install examples use `--version 0.1.0`.
   - Remove all references to `specd update`, `specd uninstall`, `specd migrate` as CLI commands.
   - Remove any "pre-1.0" or "not released" language; replace with v0.1.0 release language.
   - Remove any `v0.2.0`/`v0.3.0` deprecation target references.

3.2 `AGENTS.md`: 
   - Remove RTK section.
   - Remove any references to retired commands.
   - Update version references to v0.1.0.

3.3 `SECURITY.md`: 
   - Update "Supported versions" to reflect v0.1.0 as current supported version.
   - Remove `update`/`uninstall` CLI references.
   - Remove "pre-1.0" language.

3.4 `TESTING.md`: 
   - Audit `COVERAGE_GAPS.md` reference.
   - Update or remove if stale.
   - Update version references to v0.1.0.

3.5 `docs/command-reference.md`: 
   - Remove migration appendix.
   - Ensure command list matches surviving `Commands`.
   - Update version references to v0.1.0.

3.6 `docs/user-guide.md`: 
   - Remove deprecated command mentions.
   - Update install examples to `--version 0.1.0`.

3.7 `docs/validation-gates.md`: 
   - Remove `doctor` references.
   - Update version references to v0.1.0.

3.8 `docs/agent-integration.md`: 
   - Remove deprecated command references.
   - Update install examples to `--version 0.1.0`.

3.9 `docs/contributor-guide.md`: 
   - Update "Extending the CLI" to not mention removed commands.
   - Update version references to v0.1.0.

3.10 `docs/mcp-guide.md`, `docs/github-action.md`, `docs/open-spec-format.md`, `docs/spec-packs.md`, `docs/custom-gates.md`, `docs/concepts.md`, `docs/troubleshooting.md`:
   - Audit for deprecated command references.
   - Update version references to v0.1.0.
   - Update install examples to `--version 0.1.0`.

**Validation:** 
- `grep -r 'doctor\|dispatch\|program\|validate\|schema\|replay\|diff\|serve\|watch\|mode\|migrate\|update\|uninstall' docs/ README.md AGENTS.md TESTING.md SECURITY.md` returns nothing (except legitimate references like "install script" or "update your shell").
- `grep -r 'v1\.0\.0\|v0\.2\.0\|v0\.3\.0' docs/ README.md AGENTS.md TESTING.md SECURITY.md` returns nothing (except historical changelog if any).
- All install examples reference `0.1.0`.

### Phase 4: Hardening & Final Validation
4.1 Run `make lint` (`gofmt`, `go vet`, `shellcheck`, `test-lint`).  
4.2 Run `make ci` (full gate).  
4.3 Run `make stress` and all stress variants.  
4.4 Verify `go.mod` has zero `require` entries.  
4.5 Verify `.goreleaser.yml` still produces `SHA256SUMS`.  
4.6 Verify `scripts/install.sh` still references `SHA256SUMS` correctly.  
4.7 Build on Windows (`GOOS=windows go build`).  
4.8 Run `govulncheck ./...` if available.  
4.9 Verify no dead code with `staticcheck ./...` if available.  

**Gate:** All checks green before tagging.

---

## 9. Risks and Unknowns

| ID | Risk/Unknown | Consequence | Resolution/Mitigation | Gate |
|----|--------------|-------------|-----------------------|------|
| U1 | `runDoctorCmd` and other handler stubs may be defined in separate files not yet inspected | Build failure after alias removal | Search for all referenced handler names before deleting aliases | Phase 1 validation |
| U2 | `AGENTS.md` RTK section may be referenced by an embed template | Broken template or confusing agent guidance | Search `internal/core/embed_templates/` for "RTK" or "rust" | D3 |
| U3 | `install.sh` may reference `uninstall.sh` | Broken install script | Grep `install.sh` for "uninstall" | D1 |
| U4 | Some tests may assert on the full `Commands` slice contents | Test failures after metadata deletion | Update tests to assert only on surviving commands | Phase 2 |
| U5 | `config_migrate.go` may be imported by `init.go` or tests beyond `--migrate` | Build failure | Audit all imports of `config_migrate.go` symbols | Phase 1 |
| U6 | `update.go` may define symbols used by other commands (e.g., `fetchChecksums`) | Build failure or lost shared utility | Check if `fetchChecksums`/`releaseURL` are used elsewhere; move to shared package if needed | Phase 1 |
| U7 | `docs/` may contain cross-references to deprecated commands in examples | User confusion | Comprehensive grep in Phase 3 | Phase 3 validation |
| U8 | `go.mod` fetch failed during inspection; actual content unknown | Risk of hidden dependencies | Re-inspect `go.mod` in live repo before starting | Phase 4 |
| U9 | Version strings may be hardcoded in unexpected places (templates, error messages) | Inconsistent version references | Comprehensive grep for `v0.2.0`, `v0.3.0`, `v1.0.0` across entire repo | Phase 3 validation |
| U10 | `install.sh` may have version-specific logic | Breaking install for v0.1.0 | Audit `install.sh` for version assumptions | Phase 4 |

---

## 10. Definition of Done

- [ ] `make ci` passes on Linux and macOS.
- [ ] `GOOS=windows go build` succeeds.
- [ ] `go.mod` has zero `require` entries.
- [ ] No `legacyAliases`, `DeprecatedIn`, or `Hidden` deprecated commands remain in `registry.go` or `commands.go`.
- [ ] `internal/cmd/update.go` deleted.
- [ ] `internal/core/config_migrate.go` deleted.
- [ ] `scripts/uninstall.sh` deleted.
- [ ] `AGENTS.md` RTK section removed.
- [ ] `README.md` has no deprecated command references; install examples use `--version 0.1.0`.
- [ ] All `docs/*.md` files contain no references to removed commands.
- [ ] `SECURITY.md` references v0.1.0 as supported version; no pre-1.0 language.
- [ ] `TestRegistryMatchesHelp` passes.
- [ ] Coverage floors are still met (`scripts/coverage-check.sh`).
- [ ] No `v0.2.0`, `v0.3.0`, or `v1.0.0` references exist in docs (except historical changelog).
- [ ] All install examples across docs reference `--version 0.1.0`.
- [ ] Release tag `v0.1.0` is ready to be applied.

---

## 11. Traceability

| Requirement | Evidence | Spec | Validation | Done Criterion |
|-------------|----------|------|------------|----------------|
| R1 | `registry.go` `legacyAliases` | S1 | `make test`, grep | No deprecated aliases in source |
| R2 | `internal/cmd/update.go` | S1 | `go build` | File deleted, build passes |
| R3 | `scripts/uninstall.sh` | S3 | `shellcheck`, grep | File deleted, no references |
| R4 | `config_migrate.go`, `init.go --migrate` | S2 | `make test` | Migration code removed |
| R5 | `docs/*.md`, `README.md`, etc. | S4, S5 | grep for deprecated names | No matches |
| R6 | `AGENTS.md` tail | S4 | grep for "RTK" | Section removed |
| R7 | `README.md` | S4 | Manual review | No deprecated commands; `0.1.0` examples |
| R8 | `SECURITY.md` | S4 | grep for "pre-1.0" | Updated to v0.1.0 |
| R9 | `Makefile`, `ci.yml` | S6 | `make ci` | All green |
| R10 | `go.mod` | S6 | `cat go.mod` | Zero `require` |
| R11 | `internal/core/exit.go` | S6 | `make test` | Exit codes unchanged |
| R12 | `.goreleaser.yml`, `install.sh` | S6 | grep `SHA256SUMS` | Filename preserved |
| R13 | `registry.go` `doctor` alias | S1 | `make test` | Handler deleted |
| R14 | Version refs across docs | S4, S5 | grep for old versions | Only v0.1.0 remains |
| R15 | Install examples | S4, S5 | grep for `--version` | All use `0.1.0` |
| R16 | Old-version handling code | S1, S2, S6 | `make test`, code review | No version-gated branches |
