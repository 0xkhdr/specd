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
