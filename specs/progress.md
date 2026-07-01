# Progress: specd v0.1.0 Development Release — Deprecation Cleanup & Hardening

## Overall Status

**Status: specs authored, not yet executed.** All six specs and their task lists are
written and internally validated (cross-referenced against a live re-inspection of the
repository, not assumed from the analysis plan). No code, docs, or scripts have been
changed yet — per the action prompt's stop condition, execution requires separate,
explicit user instruction.

**Current wave:** none (pre-execution).

## Critical Deviation Flagged Before Spec Authoring

The analysis plan assumes (§1, §2 Assumption A1, §2 Decision Gate D4) that v0.1.0 has
**never been released** — "first development release." Live repo inspection contradicts
this:

- Git tag `v0.1.0` already exists.
- `CHANGELOG.md` has a `## [0.1.0] - 2026-06-14` section: *"First public release of
  `specd`..."*
- `CHANGELOG.md` already has an `[Unreleased]` section documenting the `boot`/`enrich`
  command removal as a prior breaking change — meaning deprecation-cleanup work of this
  exact kind is already underway, mid-stream, not a one-time pre-release scrub.

**User decision (asked directly, 2026-07-01):** proceed under the v0.1.0 framing anyway,
as the analysis/action prompt specify. The existing `v0.1.0` tag will need to be re-cut or
amended at actual release time — **this is explicitly out of scope for spec authoring and
is not one of this spec tree's tasks.** Flagging it here so it isn't lost before the
eventual release step.

## Requirement → Spec Coverage Matrix

| Req | Description | Spec | Status |
|---|---|---|---|
| R1 | Remove deprecated CLI commands/aliases | S1 | Spec written |
| R2 | Remove `update` handler | S1 | Spec written |
| R3 | Remove `uninstall.sh` | S3 | Spec written |
| R4 | Remove legacy config migration | S2 | Spec written |
| R5 | Scrub docs of deprecated command refs | S4, S5 | Spec written |
| R6 | Remove `AGENTS.md` RTK section | S4 | Spec written |
| R7 | Update `README.md` for v0.1.0 | S4 | Spec written |
| R8 | Update `SECURITY.md` for v0.1.0 | S4 | Spec written |
| R9 | `make ci` passes after removals | S6 | Spec written |
| R10 | Preserve zero-dependency invariant | S6 | Spec written |
| R11 | Preserve exit-code contract | S1, S2, S6 | Spec written |
| R12 | Preserve `SHA256SUMS` contract | S3, S6 | Spec written |
| R13 | Remove `doctor` handler | S1 | Spec written |
| R14 | Update version references to v0.1.0 | S4, S5, S6 | Spec written |
| R15 | Install examples use `--version 0.1.0` | S4, S5, S6 | Spec written |
| R16 | Remove old-version-gated code | S1, S2, S6 | Spec written |

## Spec Status, Dependencies, Blockers

| Spec | Wave | Depends on | Status | Notes |
|---|---|---|---|---|
| S1: Deprecation Cleanup — Commands | 1 | none | Spec + tasks written | See "Major Design Correction" below |
| S2: Deprecation Cleanup — Config Migration | 1 | none (logically after/with S1) | Spec + tasks written | `config_migrate.go` is NOT deleted wholesale — see correction below |
| S3: Deprecation Cleanup — Scripts | 1 | none | Spec + tasks written | Straightforward; matches analysis plan closely |
| S4: Docs Alignment — Root | 2 | S1, S2, S3 | Spec + tasks written | SECURITY.md needs substantive (not cosmetic) rewrite for the `doctor` advisory loss |
| S5: Docs Alignment — Guides | 2 | S1, S2, S3 | Spec + tasks written | Only `command-reference.md` needs edits; 13 other files audited clean |
| S6: Hardening — CI & Validation | 3 | S1-S5 | Spec + tasks written | New findings: `docs-lint.sh` is broken/unwired; `CHANGELOG.md` needs breaking-change entries |

No spec is blocked. S1/S2/S3 have no interdependencies and can execute in parallel. S4/S5
must follow S1-S3. S6 is terminal.

## Version Alignment Status

Verified via direct repo inspection (not assumed) at spec-authoring time:

| File | `--version` install example | Old version strings (v0.2.0/v0.3.0/v1.0.0/pre-1.0) |
|---|---|---|
| `README.md` | Already `0.1.0` ✅ | None found |
| `docs/user-guide.md` | Already `0.1.0` ✅ | None found |
| `scripts/install.sh` | Usage comment already `v0.1.0` ✅ | None found |
| `SECURITY.md` | n/a | **"pre-1.0" found, line 16 — S4 fixes** |
| `AGENTS.md` | n/a | None found (outside RTK section, unrelated to versioning) |
| `TESTING.md` | n/a | None found |
| `docs/command-reference.md` | n/a | **`v0.2.0` found ×11, all inside the migration appendix — S5 deletes the appendix** |
| All other `docs/*.md` | n/a | None found |

**Net finding: version alignment is already ~95% done in the live repo.** The bulk of
remaining work is removing the deprecated *command* surface and its doc traces, not
fixing version strings — most version strings were already correctly v0.1.0 before this
cleanup started.

## Major Design Corrections vs. the Analysis Plan (read before executing S1/S2)

These were discovered via live cross-reference grep, not assumed, and materially change
the implementation vision from analysis-plan §6:

1. **Not all 10 "functional" legacy alias handlers are dead code.** `runDispatch`,
   `runProgram`, `runValidate`, `runSchema`, `runReplay`, `runDiff`, `runServe`, `runWatch`
   are called from **both** the deprecated alias table **and** the canonical v0.1.0
   surface (`next --dispatch`, `status --program`, `check --schema*`,
   `report --history/--diff/--serve/--watch`). **Their handler files must NOT be deleted**
   — only the top-level alias registration goes away. Deleting `dispatch.go` etc. would be
   a functional regression to the *current, non-deprecated* command surface, not a cleanup.
   Only `doctor.go` (whole file, genuinely orphaned) and `mode.go`'s `runMode` function
   specifically (not the whole file — `runModeSet`/`runModeRecommend`/`printMode` are
   shared with `status.go`) are actually dead code. See S1 spec for full detail.

2. **`internal/core/config_migrate.go` is not migration-only.** Only 2 of its 8 functions
   (`MigrateConfigPreview`, `MigrateConfigFile`) are migration-specific. `RenderConfigYAML`
   is used by `init.go`'s normal (non-migration) scaffolding; `ValidateConfigDoc` is used
   by `config_validate.go`. Deleting the whole file (as analysis plan F4 assumed) would
   break `init` and config validation entirely. See S2 spec for full detail.

3. **Removing `doctor` is a genuine, accepted capability loss, not a pure rename.** The
   codebase's own comments (`registry.go:78-85`) state the survivor (`init --repair`) does
   **not** preserve `doctor`'s diagnostics. In particular, `SECURITY.md`'s documented
   `bwrap`/container-dependency advisory (`inspectSandboxAvailability`, only called from
   `runDoctor`) disappears entirely — this is a real threat-model change requiring a
   substantive `SECURITY.md` rewrite (S4), not a word-swap.

4. **This cleanup is anticipated by the codebase's own comments**, not a novel departure:
   `registry.go:60-64` explicitly says *"At this release the corresponding entries in
   legacyAliases are deleted..."* — the code authors already planned this exact removal.

## New Findings Not in the Original Analysis Plan

- `scripts/docs-lint.sh` currently hard-fails (missing
  `.specd/specs/cmd-audit/audit.csv`) and is wired into neither `Makefile` nor CI — it is
  dead, broken tooling today, unrelated to this cleanup but surfaced by it. S6 proposes two
  remediation paths and flags the choice as an open decision for the user/executor.
- `TESTING.md`'s reference to `COVERAGE_GAPS.md` points at a file that does not exist
  anywhere in the repo. S4 recommends removing the dangling claim rather than fabricating
  the file.
- `TESTING.md`'s "Windows limitation (known, documented)" section and its `SHA256SUMS`
  three-consumer list both need substantive rewrites (not just deletions) once `update.go`
  is gone — their entire premise depends on that file existing.
- `.goreleaser.yml:35`'s comment names `update.go` — needs a one-line edit once that file
  is deleted (S6).
- `docs/agent-harness-baselines.md`, `docs/agent-harness-compat.md`,
  `docs/agent-harness-gap-analysis.md`, and `docs/dashboard.md` exist in `docs/` but were
  not in the analysis plan's file inventory. Audited (S5) and found clean — no action
  needed, noted so their absence from the plan isn't mistaken for an oversight.
- `mode`'s `removedIn: "v0.3.0"` (registry.go) vs. `DeprecatedIn: "v0.2.0"` (commands.go) is
  **not a bug** — two different fields with two different meanings (S1 spec explains).
  `docs/command-reference.md`'s migration-appendix table did conflate them under one
  "Removed in" column showing v0.2.0 for `mode`, but that entire table is deleted in S5, so
  the conflation is moot.
- `internal/cmd/init.go`'s `legacy-config-deprecated` warning message references the
  disappearing `specd migrate config` command and needs its own edit (S2), which the
  original analysis plan did not call out at this level of detail.

## Corrections vs. Specific Analysis-Plan Findings

- **F8 is stale.** `docs/validation-gates.md` does **not** reference `doctor` (verified,
  zero grep matches). No action needed there.
- **F10/U3 resolved, not a risk.** `scripts/install.sh` has zero references to
  `uninstall.sh` — confirmed directly, not just "should verify."

## Completed / Remaining Waves

- [x] Pre-flight repository re-inspection (this document's basis).
- [x] All six specs (`spec.md` + `tasks.md`) authored and cross-checked against live repo
      state.
- [ ] S1 execution (Wave 1: orphaned-file deletion; Wave 2: shared-code surgery + alias
      removal; Wave 3: metadata removal; Wave 4: regression).
- [x] S2 execution (Wave 1: delete migration command/functions; Wave 2: remove `--migrate`
      flag; Wave 3: regression). Also fixed a compile-blocking caller in
      `internal/cmd/config_e2e_test.go` (`TestConfigInitMigrateE2E`'s migrate subtest) that
      called `RunMigrate` directly — not in S2's original file inventory, discovered during
      `go build`. Regenerated `internal/mcp/testdata/tool_schemas.golden.json` (the `init`
      tool's schema dropped the `migrate` flag). Confirmed `init --migrate` is silently
      tolerated (exit 0, normal init runs), not a usage error — the analysis plan's exit-2
      assumption was wrong; the CLI parser's unknown-flag tolerance is pre-existing,
      intentional, and covered by `TestUnknownFlagIsTolerated` — no fix needed, not an S2
      regression.
- [ ] S3 execution (Wave 1: delete script; Wave 2: CI confirmation).
- [ ] S4 execution (Waves 1-5: README, AGENTS.md, SECURITY.md, TESTING.md, cross-file gate).
- [ ] S5 execution (Wave 1: command-reference.md edits; Wave 2: audit pass; Wave 3: handoff
      to S6).
- [ ] S6 execution (Wave 1: regression; Wave 2: release-artifact checks; Wave 3:
      `docs-lint.sh` fix; Wave 4: `CHANGELOG.md`; Wave 5: final gate / Definition of Done).

## Stop Condition Reminder

Per the action prompt: do not open a PR, tag a release, or push to `main` beyond writing
and validating these specs, unless the user explicitly requests execution.
