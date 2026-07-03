# specd v0.2.0 — Implementation Review & Action Plan

**Reviewer pass date:** 2026-07-03
**Branch:** `v0.2.0`
**Scope:** Verify `specs/progress.md` claims against the real codebase, close
release-gate gaps, and lay out the path to tag.

---

## 1. Executive summary

The v0.2.0 program (Waves 1–6, specs V1–V12) is substantially complete and the
architecture holds the constitution: **zero external Go deps** (`go.mod` stays
stdlib-only), `go build ./...` and `go vet ./...` clean, and the full test suite
green (1915 tests before this pass, +8 added).

**One real release blocker was found and fixed:** `make ci` was **RED** on two
independent counts, both introduced by the Wave 5/6 landings:

1. **Coverage floors breached** — the new V11/V12 files (`dashboard`, `harness`,
   `migrate`, `pack_registry` across `internal/core`, `internal/cmd`,
   `internal/pack`) added code faster than tests, dropping four gates below
   floor.
2. **Banned test-file names committed** — six `wave4_*`/`wave5_*` coverage test
   files violate the repo's own `test-lint` rule (`wave[0-9]` suffix banned),
   hard-failing the `lint` stage of `make ci`.

Both are now resolved; **`make ci` is GREEN** (lint, test, test-order,
cover-check, perf-gate, and every stress target pass, race-clean).

The remaining work is exactly the documented V12 Wave 2/3 tail — a docs sweep,
success-metrics verification wiring, a benchmark refresh, and the release tag.
None of it blocks correctness; it blocks *ship*.

---

## 2. What was verified green (no action needed)

| Check | Result |
|-------|--------|
| `go build ./...` | ✅ success |
| `go vet ./...` | ✅ no issues |
| `go.mod` | ✅ stdlib-only, 3-line module (constitution #3 held) |
| Full `go test ./...` | ✅ 1915 → 1923 passing, 15 packages |
| V11 race gate (`-race -count=2`, Harness/Dashboard/Registry/Pack) | ✅ 116 pass |
| `make docs-lint` | ✅ cheat-sheet mirrors match (32 commands) |
| `make perf-gate` | ✅ deterministic-output + bench-contract stable |
| All 12 spec dirs present, 26 plan tasks → 1 spec each | ✅ |
| New commands registered (`dashboard`/`harness`/`migrate`) | ✅ in Registry + command-reference + parity tests |

The DV1–DV4 deviations recorded in `progress.md` (package path, schema v6, progress
placement, prototype-in-V5) all check out against the tree.

---

## 3. Gaps found & fixed in this pass

### 3.1 Coverage floors breached — FIXED

`make cover-check` failed on four gates. Root cause: V11/V12 code shipped with
happy-path tests only; error branches, transport branches, and the YAML renderer
were untested.

| Gate | Before | After | Floor |
|------|-------:|------:|------:|
| overall | 77.9% | **79.1%** | 79% |
| internal/core | 79.2% | **80.8%** | 80% |
| internal/cmd | 70.1% | **71.2%** | 71% |
| internal/pack | 81.3% | **87.6%** | 87% |

Tests added (all deterministic, no network beyond `httptest`/local git):

- `internal/pack/pack_registry_test.go` — direct `resolveRegistryPack` coverage:
  HTTP success, HTTP non-200, oversize (>1 MiB limit), missing `file://`,
  unsupported scheme. (31.8% → 90.9% on that function.)
- `internal/core/config_migrate_render_test.go` — `RenderConfigYAML` across all
  optional blocks (resilience, MCP essential-tools, compaction) + determinism.
  (0% → covered.)
- `internal/core/eval_trend_test.go` — `EvalTrend` (the reader feeding the V11
  dashboard eval panel): deltas, per-suite filter, failure clustering. (0% →
  covered.)
- `internal/core/dashboard_test.go` — escalation-row render branch.
- `internal/core/harness_test.go` — `SecureGitClone` (shallow/full/rejected
  transport) + `PushHarness` fail-closed branches (empty bundle, hostile URL).
- `internal/cmd/dashboard_test.go` — bad `--mode`, no-root, unbindable `--addr`.
- `internal/cmd/harness_test.go` — text-mode list, enable errors, `--json`
  push/pull, refused-without-`--force`, bad-remote branches.

### 3.2 Banned `wave*` test filenames committed — FIXED

`scripts/test-lint.sh` bans `wave[0-9]` (and `_more/_regression/_sweep/_scale`)
test-file suffixes; six committed files violated it and hard-failed `make ci`:

```
internal/core/wave4_config_coverage_test.go     → config_extra_coverage_test.go
internal/core/wave4_core_coverage_test.go       → core_extra_coverage_test.go
internal/core/wave4_specfiles_coverage_test.go  → specfiles_extra_coverage_test.go
internal/core/wave5_coverage_test.go            → lifecycle_extra_coverage_test.go
internal/cmd/wave4_helper_coverage_test.go      → helper_extra_coverage_test.go
internal/cmd/wave5_coverage_test.go             → cmd_extra_coverage_test.go
```

Renamed via `git mv` (content unchanged). `make test-lint` now passes and
`make ci` is green end-to-end.

---

## 4. Remaining work (documented V12 tail — not yet done)

These match `specs/v020-release-engineering/tasks.md` Waves 2–3 and are the true
gate to tagging. None is a correctness bug.

### P0 — required before tag

1. **Docs sweep (breadth).** New commands appear in `command-reference.md`, but
   `specd migrate` is absent from `user-guide.md`, `validation-gates.md`,
   `mcp-guide.md`, and both `AGENTS.md` templates; `dashboard`/`harness` coverage
   there is thin. `docs-lint` only checks cheat-sheet mirror parity, so it does
   **not** catch this. **Action:** add `dashboard`/`harness`/`migrate` sections to
   the four docs + AGENTS template, then keep `make docs-lint` green.

2. **Success-metrics verification wiring.** Plan Part III lists seven metrics
   (first-pass verify >85%, security catch >90%, mode-switch <30s, ingestion
   100%, cost attribution 100%, eval coverage ≥1/spec, observe→midreq). There is
   **no dedicated CI test** asserting each is measured (grep finds only incidental
   hits). **Action:** add one `metrics_verification_test.go` that exercises the
   measuring path for each metric, wire into `make ci`.

3. **`make bench` baseline refresh.** `docs/agent-harness-baselines.md` must be
   re-measured against v0.1.x with the ±10% floor held. **Action:** run
   `make bench`, update the baseline doc, confirm no regression.

### P1 — release mechanics

4. Confirm V8 threat-model refresh covers the now-shipped deploy/observe/submit
   exec surfaces (hard gate).
5. `bash scripts/install_test.sh` SHA256 re-verify + goreleaser dry-run.
6. PR `v0.2.0` → `main`; tag `v0.2.0` on `main` post-merge; release notes from
   CHANGELOG.

---

## 5. Recommended sequence

```
[done]  Fix make ci (coverage + test-lint)      ← this pass, GREEN
  1.    Docs sweep (4 docs + AGENTS)             ← P0, ~half day
  2.    Success-metrics verification test        ← P0, wire into make ci
  3.    make bench + baseline refresh             ← P0
  4.    Threat-model confirm + install_test.sh    ← P1
  5.    PR to main → merge → tag v0.2.0            ← P1
```

Update `specs/progress.md` V12 line to reflect: CI now green; only the docs
sweep, metrics wiring, bench refresh, and tag remain.

---

## 6. Constitution compliance (spot-check)

| # | Rule | Status |
|---|------|--------|
| 2 | Zero LLM calls in binary | ✅ judges stay external |
| 3 | Zero external Go deps | ✅ `go.mod` 3 lines, stdlib JSON |
| 5 | Evidence gates every state change | ✅ harness `enable`, deploy `approve --deploy` gated |
| 7 | Byte-stable round-trips; FakeClock | ✅ new `RenderConfigYAML`/dashboard render assert determinism |
| 8 | New exec surfaces hardened | ✅ single `SecureGitClone` path (scrubbed env, transport allowlist); adversarial `ext::` test added this pass |
| 10 | Registry discipline | ✅ new cmds have file+Registry+CommandMeta+parity |

No constitution violation found. Rule 8's shared git-exec path is now covered by
a core-level adversarial test in addition to the pack/cmd e2e.
