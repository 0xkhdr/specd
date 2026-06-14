# specd — Regression Findings

> Branch: `regression/ship-readiness`
> Started: 2026-06-14

Severity: **blocker** (must fix before ship) | **warning** (should fix) | **info** (record only).

---

## Stage 1 — Spec & Refactor-Stage Traceability Audit

**Verdict: PASS.** All 7 refactor stages (`01-security` … `07-testing-ci`) have
their claimed work present in the codebase. Acceptance criteria satisfied; every
`tasks.md` verify command re-run and green.

### Baseline re-confirmed
- `go build ./...` → exit 0
- `go vet ./...` → clean
- `gofmt -l .` → empty
- `go test ./... -count=1` → all 5 packages PASS (root, cli, cmd, core, testharness)
- Targeted verify runs (`Lock|CAS|State|Concurren|DAG|Ordinal|Tasks|Ears|Boot|Enrich`,
  `Check|Task|Verify|Update|Boot`, cli, `RegistryMatchesHelp`) → PASS

### Per-stage artifact confirmation
- **01-security:** `core.EnvInt` (`env.go`); `AtomicWrite` `f.Chmod(0o644)` (`io.go:53`);
  `fetchChecksums`/`sha256`/"checksum mismatch" (`update.go`); `scrubbedEnv`,
  `SPECD_VERIFY_SHELL`, NUL reject (`verify.go`); `--no-verify`+`SHA256SUMS` (`install.sh`);
  "Security model" sections in `AGENTS.md` + `docs/validation-gates.md`.
- **02-concurrency-state:** `goIDFallback` + `1<<40` sentinel, `owner == gid && gid != -1`
  (`lock.go`); `lockHeldBy`+`assertLocked` and "disappeared mid-session" conflict
  (`state.go`); "corrupt schemaVersion" (`state.go:144`); `concurrency_test.go` with panic case.
- **03-command-decomposition:** `gates.go` with `GateEars/Design/TaskSchema/DAG/Sync/Traceability/Evidence`;
  `RemoveBlocker`/`AddBlocker` (`blockers.go`); `IsValidRole`/`IsReadonlyRole`;
  `validateComplete` (`task.go`), `runVerifyCommand` (`verify.go`); `min3`/`min` removed.
- **04-cli-output-consistency:** `PrintJSON` (`output.go`); zero bare `return 0/1` in `internal/cmd`;
  `--key=value` parsing (`args.go:22`); `Registry` + parity test (`registry.go`/`registry_test.go`);
  `errLine` (`helpers.go`).
- **05-dag-domain-logic:** `CriticalPath` cycle guard (`dag.go:282`); `ordinal` documented;
  duplicate-id `SpecdError` (`tasksparser.go:229`); `ParseDepends`; boot detectors deterministic.
- **06-performance:** `sectionRECache` (`report.go`), `wordRECache`/`wordRE` hoist (`boot_detectors.go`);
  benchmarks `BenchmarkDetectCycle/NextRunnable/BootDetect`; slice prealloc in `dag.go`/`render.go`.
- **07-testing-ci:** `ci.yml` with gofmt/shellcheck/race/`-count=2`/coverage-floor/cross-process stress/OS matrix;
  `.goreleaser.yml` `checksum.name_template: SHA256SUMS`; `scripts/coverage-check.sh`.

### Findings logged (do NOT fix in Stage 1)
| ID | Severity | Stage | Finding |
|----|----------|-------|---------|
| F-S3-1 | warning | 03 | `RunCheck` body is ~73 lines; spec AC1 targets "≤ ~40 lines". Behavior correct and gates extracted into `core/gates.go` as required; only the soft line-count target is exceeded. Revisit in Stage 8. |
| F-S1-1 | info | 01/07 | `shellcheck` not installed in local env — `install.sh`/`stress.sh` lint not verifiable locally; covered by CI `shellcheck` job. |

No **blocker** findings in Stage 1.

---

## Stage 2 — Build Integrity & Test Suite Validation

**Verdict: PASS** (after fixing one gate-breaking lint finding).

### Gate results
- `make clean` + `make build` → binary 7.3 MB, version `-ldflags`-injected (`specd v0.1.0-24-...`)
- `make test` (`-race -count=1`) → all 5 packages PASS
- `make test-order` (`-count=2`) → PASS (no order dependence)
- `gofmt -l .` empty; `go vet ./...` clean; `shellcheck scripts/*.sh` clean (after fix)
- `make cover-check` → overall 64.0% ≥ 60% floor; `internal/core` 59.9% ≥ 58% floor
- `make stress` → 16×20 cross-process, 320 committed writes, turn==successes, `state.json` intact
- `go.mod` → zero `require` entries
- Embed: lone built binary copied to empty temp dir → `specd init` creates `.specd/` (config.json, roles, steering) — no disk templates needed
- **`make ci` (full gate) → GREEN**

### Findings fixed (gate-breaking)
| ID | Severity | Finding | Fix |
|----|----------|---------|-----|
| F-S2-1 | warning | `shellcheck scripts/*.sh` exited 1 → `make lint`/`make ci` failed. SC2034 unused `BOLD`/`BLUE` in `uninstall.sh`; SC2059 variables-in-printf-format in `install.sh` (×2) and `uninstall.sh` (×4). Pre-existing, not introduced by Stage 1. | Removed unused color vars; converted color printfs to `printf '...%s...' "$VAR"` form. Behavior identical (same output), shellcheck now clean. |

### Notes
- `shellcheck` not in local env; installed static binary `v0.10.0` to `/tmp/shellcheck` for local gate parity. CI uses `ludeeus/action-shellcheck`. (Supersedes Stage 1 F-S1-1 for local verifiability.)

No **blocker** findings in Stage 2.

---

## Stage 5 — CLI Surface & Command Registry Consistency

**Verdict: PASS** (after fixing one gate-breaking parity bug).

### Gate results
- `specd help --json` → complete registry dump (all 19 dispatchable commands, with flags/exit-codes/examples)
- `TestRegistryMatchesHelp` + `TestRegistryHandlersNonNil` → PASS (`cmd.Registry` ⇔ `core.Commands` parity, no nil handlers)
- Every command exercised by co-located tests in `internal/cmd/` (`commands_test.go` covers new/check/next/dispatch/task/status/approve/midreq/decision/context/waves/report/memory/init/program; dedicated `boot_test`/`enrich_test`/`task_test`/`update_test`/`verify_test`/`lifecycle_test`)
- Boolean flags: all registered in `cli.booleanFlags`; `TestBooleanFlagsRegistered` derives usage from source so the list can't drift
- Exit codes verified live: `0` ok, `2` usage (`new` no-args, unknown command), `3` not-found (`check ghost`), `1` gate (`check` bad EARS)
- `SPECD_JSON=1` → valid JSON on success paths for status/check/next/context/waves/program/boot/enrich plan/help (validated through `json.load`); error paths emit human text to stderr + correct exit code (consistent across all commands)
- `--json` flag == `SPECD_JSON=1` after fix (see F-S5-1)

### Findings fixed (gate-breaking)
| ID | Severity | Finding | Fix |
|----|----------|---------|-----|
| F-S5-1 | blocker | `SPECD_JSON=1` did **not** produce JSON for flag-reading commands, breaking the documented `--json == SPECD_JSON=1` parity. Commands resolve JSON via `args.Bool("json")`; `main.run` bridged the `--json` flag *into* the env (`os.Setenv`) but never the reverse, so env-only invocations (`SPECD_JSON=1 specd status` → human text, while `specd status --json` → JSON) diverged. | Seed `jsonMode := core.IsJSONMode()` at the top of `main.run` so `SPECD_JSON` is re-threaded into the per-command `--json` flag at the dispatch boundary — one fix covers all 13 flag-reading commands. Added regression test `TestRunDispatch/SPECD_JSON_env_matches_--json_flag` asserting byte-identical output + exit code. |

No **open** findings in Stage 5.

---

## Stage 7 — Documentation & Release Artifact Review

**Verdict: PASS** (after fixing one blocker + one warning).

### Review results
- `README.md` — features/install/quick-start accurate; repo & docs map matches actual tree.
- `TESTING.md` — accurate: Go commands, `make ci` gate, coverage floors (overall 60% / core 58%),
  Windows `specd update` limitation, `SHA256SUMS` three-consumer note.
- `docs/` complete — all six present (`concepts user-guide command-reference validation-gates
  agent-integration contributor-guide`) + `docs/README.md` index. `contributor-guide.md` is
  Go-accurate (codebase map, contracts, extension recipes, custom-parser ADR).
- `LICENSE` — MIT, © 2026 0xkhdr.
- `.github/workflows/ci.yml` — lint (gofmt/vet/shellcheck), test (ubuntu+macOS, `-race -count=1`
  then `-count=2`), coverage-floor, cross-process stress, build (ubuntu+macOS+Windows). ✓
- `.github/workflows/release.yml` — on `v*` tags → re-run race suite, then GoReleaser. ✓
- `.goreleaser.yml` — `checksum.name_template: SHA256SUMS` matches the filename `update.go`
  (`fetchChecksums`) and `install.sh` (`verify_checksum`) expect; builds linux/darwin/windows ×
  amd64/arm64; `-ldflags "-s -w -X main.version={{.Version}}"`. ✓
- Version injection via `-ldflags` confirmed in `Makefile` + both workflows.

### Findings

| ID | Severity | Finding | Fix |
|----|----------|---------|-----|
| F-S7-1 | blocker | Root `AGENTS.md` described a **TypeScript/npm** project (`npm run build`, `tsc → dist/`, `node --test`, "65 tests", `src/**/*.ts` layout, `dist/templates/`) — wholly wrong for this Go/stdlib repo. An agent following it would chase non-existent files/commands. | Rewrote "What this repo is", "Build & test", "Repo layout", "Key contracts", "Templates are shipped", "Design references", "Working on this repo" to the actual Go layout (`main.go` + `internal/{cli,cmd,core,testharness}`, `make build`/`make test`/`make ci`, `go:embed embed_templates/`, `cmd.Registry`/`CommandMeta`/`TestRegistryMatchesHelp`). Security-model section was already correct, left intact. |
| F-S7-2 | warning | `scripts/install.sh` writes its PATH-export line with the marker `# specd`, but `scripts/uninstall.sh` detected/stripped lines matching `# specd PATH` — so uninstall never removed the PATH line install added (orphaned config). | Aligned `uninstall.sh` to match on `# specd` (both the detection guard and the `grep -v` strip). |
| F-S7-3 | info | No static `CHANGELOG.md` in repo. Not a blocker: `.goreleaser.yml` has a `changelog:` block that auto-generates release notes from commits (excluding `docs:`/`chore:`/`ci:`), so each GitHub Release carries its changelog. | None — documented as intentional. |

### Post-fix validation
- `gofmt -l .` empty · `go vet ./...` clean · `shellcheck scripts/*.sh` clean · `go build` ok.

No **open** findings in Stage 7.

---

## Stage 8 — Final Ship Gate & Sign-Off

**Verdict: PASS — ship-ready.** All blocker findings resolved; the sole open
warning (F-S3-1) was closed by a low-risk refactor. Full local gate
(`make ci`) green.

### Finding resolved this stage

| ID | Severity | Finding | Resolution |
|----|----------|---------|------------|
| F-S3-1 | warning | `RunCheck` body was ~73 lines vs the spec AC1 soft target of "≤ ~40 lines". Gate logic was already extracted to `core/gates.go`; the excess was inline JSON + human output rendering. | Extracted the two output paths into `renderCheckJSON` and `renderCheckHuman` helpers (`internal/cmd/check.go`). `RunCheck` body is now ~30 lines. Behavior byte-identical (same JSON shape, same human lines, same exit codes); existing check/lifecycle tests still pass. |

### Final findings ledger

| ID | Severity | Stage | Status |
|----|----------|-------|--------|
| F-S1-1 | info | 01/07 | Closed — `shellcheck` run locally via pinned `v0.10.0` static binary; covered by CI. |
| F-S2-1 | warning | 02 | Fixed — shellcheck SC2034/SC2059 cleaned in `install.sh`/`uninstall.sh`. |
| F-S3-1 | warning | 03 | Fixed — `RunCheck` decomposed (this stage). |
| F-S5-1 | blocker | 05 | Fixed — `SPECD_JSON` ⇔ `--json` parity restored; regression test added. |
| F-S7-1 | blocker | 07 | Fixed — `AGENTS.md` rewritten from TypeScript to actual Go layout. |
| F-S7-2 | warning | 07 | Fixed — `uninstall.sh` PATH marker aligned to `# specd`. |
| F-S7-3 | info | 07 | Closed — no static CHANGELOG by design; GoReleaser auto-generates per-release notes. |

**Blockers: 0 open (2 fixed). Warnings: 0 open (3 fixed). Info: 0 open (2 closed).**

### Final gate results
- `make test` (`-race -count=1`) → all 5 packages PASS
- `make ci` (lint · test · test-order · cover-check · stress) → **GREEN**
  - lint: `gofmt -l .` empty · `go vet` clean · `shellcheck scripts/*.sh` clean
  - test-order (`-count=2`) → PASS (no order dependence)
  - coverage: overall **64.1% ≥ 60%**, `internal/core` **59.9% ≥ 58%**
  - stress: 16×20 cross-process, 320 committed writes, turn==successes, `state.json` intact
- `git status` clean after commit.

No **open** findings. See `SHIP_REPORT.md` for sign-off.
