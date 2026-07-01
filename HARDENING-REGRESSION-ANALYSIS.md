# Hardening Regression Analysis — Post S1–S6 Deprecation Cleanup

**Date:** 2026-07-01
**Branch:** `optimization`
**Basis:** `specs/progress.md` (S1–S6, all executed) cross-referenced against live repo state.

## Executive Summary

The six specs (S1 commands, S2 config-migration, S3 scripts, S4 root docs, S5 guide
docs, S6 CI/hardening) are **functionally sound**. Core regression is clean:

- `go build ./...` and `go vet ./...` — clean.
- All 13 removed aliases + `boot`/`enrich` return exit 2 (unknown command). Verified
  directly against the built binary (`doctor update uninstall migrate boot enrich dispatch mode` → all exit 2).
- `TestRegistryMatchesHelp`, `TestFlagSingleOwner`, `TestPaletteCeiling` pass.
- `bash scripts/docs-lint.sh` exits 0.
- Build artifacts (`specd`, `coverage-*.out`) are gitignored — no artifact-tracking leak.

**But the docs waves (S4/S5) under-scrubbed.** Multiple user-facing docs still reference
**deleted** scripts and **removed** commands. One is a broken install instruction shipped
to users. The cleanup also left an **inert config surface** in MCP and a **CI enforcement
gap** for the sync check S6 built. None of these break the build or tests — which is
exactly why they survived the specs' own acceptance gates.

Findings are ranked most-severe first.

---

## A. Regression Gaps — Docs Reference Deleted Artifacts

These broke *during* the cleanup: the referenced file/command was deleted by S1/S3 but the
doc pointing at it was never updated. S3's progress note (`progress.md:186`) explicitly
flagged `docs/mcp-guide.md` and `docs/concepts.md` "for S4/S5" — but S4/S5 never resolved
them. `grep` confirms they are all still live.

### G1 — HIGH: `docs/user-guide.md:39` ships a broken uninstall instruction

```
### Uninstall / update
    curl -fsSL https://raw.githubusercontent.com/0xkhdr/specd/main/scripts/uninstall.sh | bash
```

`scripts/uninstall.sh` was deleted in S3 (commit `0ee3050`). This command 404s. A user
following the published guide to uninstall gets an error. S5's "all other docs clean"
audit missed it because its grep matched `specd uninstall` (the command), not the
`uninstall.sh` **path** in a curl URL.

**Impact:** user-facing broken instruction. **Fix:** replace with the real mechanism
(delete the installed binary — the same wording S4 already put in
`docs/command-reference.md:70`).

### G2 — MEDIUM: `docs/mcp-guide.md` documents a removed tool surface

The MCP guide describes tools using the **old alias names** removed in S1, and
install-maintenance tools removed in S3:

- `:238` `readOnlyHint` table lists `dispatch, serve, watch, validate, replay, diff` as
  tool names — all now flags, not commands/tools.
- `:238` `destructiveHint` lists `uninstall`, `update` — both removed.
- `:297` "uninstall script | `scripts/uninstall.sh`" — deleted file.
- `:315`, `:325` `includeMeta` "expose update/uninstall/schema" — `update`/`uninstall`
  tools no longer exist (see G4 — the gate is now inert anyway).
- `:582` "repair/uninstall" reference.

**Impact:** MCP integrators reading this configure against tools that aren't advertised.
**Fix:** rewrite the hint tables and `includeMeta` description against the actual
`internal/mcp/tools.go` tool list; drop `uninstall`/`update`/removed-alias rows.

### G3 — LOW: `docs/concepts.md:128` directory tree lists a deleted file

```
├── scripts/                      # install.sh / uninstall.sh / stress.sh
```

Cosmetic, but it's a factual claim about repo layout that is now wrong. **Fix:** drop
`uninstall.sh` from the comment.

---

## B. Dead / Inert Surface Left by the Cleanup

### G4 — MEDIUM: `includeMeta` is now an inert config field

`internal/mcp/tools.go` — `metaRiskCommands` (line ~39) is now an **empty map** (its
members `update`/`uninstall` were the removed commands). `includeMeta` is still parsed and
plumbed (`mc.IncludeMeta` → `plan.includeMeta`) and still gates line ~149, but its
*documented* purpose ("hide the install-maintenance meta tools") is now vacuous — there
are no meta-risk commands left to hide.

This is a decision point, not an obvious bug:
- **Option A (remove):** drop `includeMeta` + the empty `metaRiskCommands` map + the
  line-149 branch, and delete the config field. Smallest surface.
- **Option B (keep as extension point):** retain the empty map deliberately (the
  `tools.go:39` comment already frames it that way), add a unit test that asserts an
  entry *would* be filtered, and fix the mcp-guide (G2) to describe it as reserved.

**Recommendation:** Option A unless a near-term meta command is planned — an untested,
inert flag is a maintenance liability and a source of doc drift (it caused G2).

---

## C. CI / Tooling Enforcement Gaps

### G5 — MEDIUM: `docs-lint.sh` is not enforced by PR CI

S6 fixed `docs-lint.sh` and wired it into the `Makefile` `lint` target. But
`.github/workflows/ci.yml`'s Lint job runs `gofmt`, `go vet`, `scripts/test-lint.sh`, and
`shellcheck` **as individual steps** — it does **not** call `make lint` and does **not**
invoke `docs-lint.sh`. Only `release.yml` (via `make ci`) runs it.

**Net:** the cheat-sheet ↔ `command-reference.md` sync check exists but does not gate
PRs — the same "not wired into CI" state the original analysis flagged, just moved one
layer. Drift can land on `main` unblocked. **Fix:** add a `docs-lint` step (or a
`make docs-lint` call) to the ci.yml Lint job.

### G6 — LOW–MEDIUM: the cheat-sheet check is weak and triple-maintained

`scripts/docs-lint.sh`:
- Validates only the **row count** of `.specd/specs/CHEATSHEET.md` (`count != 20`), not
  content equality with `command-reference.md`'s cheat sheet. `CHEATSHEET.md` can silently
  drift (wrong command, reordered) as long as it keeps 20 rows — defeating the file's
  stated purpose ("verbatim mirror").
- The 20-name `survivors` list is hardcoded in **three** places that must be hand-synced:
  the script array, `CHEATSHEET.md`, and `command-reference.md`. Any command
  addition/removal silently breaks the invariant unless all three are edited.

**Fix:** compare `CHEATSHEET.md` content against the extracted `command-reference.md`
cheat sheet directly (assert equality, not count); derive `survivors` from one source
instead of hardcoding.

### G7 — LOW: `coverage-check.sh` usage-header comment drift

The usage block (lines 34–36) documents `OVERALL_MIN` default **77** and `CORE_MIN`
**86**; the code (lines 53–54) sets **78** and **80**. Misleading to anyone reading the
header to understand the floors. **Fix:** sync the comment to the actual defaults.

---

## D. Improvements (Not Regressions)

- **I1 — Ratchet coverage floors up.** `coverage-check.sh` floors sit below the
  `TESTING.md` long-term targets (85% overall / 90%→95% core). With the suite now stable
  (worker back to 88.5%, flake fixed), raise the floors toward target in small steps.
- **I2 — Version tag re-cut.** `progress.md:30-34` flags that the `v0.1.0` git tag +
  CHANGELOG `[0.1.0]` section predate this cleanup and must be amended at real release
  time. Out of scope for the specs, but tracked here so it isn't lost.
- **I3 — CHANGELOG `[Unreleased]` → `[0.1.0]`.** The breaking-change bullets S6 wrote sit
  under `[Unreleased]`; they must move under a re-cut `0.1.0` (or a new version) at release.

---

## Action Plan

Do the docs-regression batch first (user-facing, mechanical, no code risk), then the CI
gap (prevents recurrence), then the code decision.

| # | Item | Sev | Effort | Files |
|---|------|-----|--------|-------|
| 1 | G1 — fix broken uninstall curl | HIGH | XS | `docs/user-guide.md` |
| 2 | G2 — rewrite MCP hint tables / includeMeta doc | MED | S | `docs/mcp-guide.md` |
| 3 | G3 — drop `uninstall.sh` from dir tree | LOW | XS | `docs/concepts.md` |
| 4 | G5 — add docs-lint step to PR CI | MED | XS | `.github/workflows/ci.yml` |
| 5 | G6 — strengthen cheat-sheet check to content-equality | LOW-MED | S | `scripts/docs-lint.sh` |
| 6 | G7 — sync coverage-check header comment | LOW | XS | `scripts/coverage-check.sh` |
| 7 | G4 — remove (or test+document) inert `includeMeta` | MED | S | `internal/mcp/tools.go` (+ config) |
| 8 | I1 — ratchet coverage floors toward target | — | S | `scripts/coverage-check.sh` |

### Recommended sequencing

**Wave 1 — docs regression (batch, doc-only):** items 1, 2, 3. After: re-grep for deleted
artifacts to prove closure:
```
grep -rn "uninstall\.sh\|specd update\|specd uninstall" docs/ README.md TESTING.md
# expect: zero (allow only historical "removed"/threat-model mentions)
```

**Wave 2 — CI enforcement:** items 4, 5. After: `make lint` and the ci.yml Lint job both
run docs-lint; introduce a deliberate cheat-sheet mismatch and confirm CI would fail.

**Wave 3 — hygiene:** items 6, 7.

**Wave 4 — MCP decision:** item 7 (G4). Requires the Option A/B call above; touches code,
so `go build`/`go test ./internal/mcp/...` must pass and the golden tool schema
(`internal/mcp/testdata/tool_schemas.golden.json`) may need regeneration if the config
field is removed.

### Definition of Done

- All deleted-artifact grep sweeps return zero live references.
- `docs-lint.sh` runs in `ci.yml` and fails on a seeded cheat-sheet mismatch.
- `coverage-check.sh` header matches its own defaults.
- G4 resolved (removed, or kept with a covering test + accurate mcp-guide).
- `make ci` green; `go vet ./...` clean; no new build/test regressions.

### Out of scope (per stop condition)

Tagging a release, re-cutting `v0.1.0`, opening a PR, or pushing to `main` — all require
separate explicit user instruction (`progress.md:268-271`). This document is analysis +
plan only.
