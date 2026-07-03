# Clean Deprecation — Minimal specd Workflow

## 1. Purpose and requirement coverage

Remove every deprecated/legacy artifact that is dead weight in the optimized
specd workflow, leaving only the minimal codebase the SDLC-paper workflow
needs. Three cuts: (A) collapse orphan helper command files whose functions
have no registry entry into their sole callers; (B) make config loading
YAML-only, dropping legacy JSON support; (C) confirm state migration is already
minimal and fix SchemaVersion 6 as the floor. No behavior of a *registered*
command changes — this is pure surface reduction. Covers CLEAN_DEPRECATION_
ACTION_PLAN §1.A, §1.B.

## 2. Verified current state

- **Registry (`internal/cmd/registry.go`)** dispatches only named top-level
  verbs (`status`, `report`, `deploy`, `observe`, `ingest`, `harness`, …).
  None of the target files below register a command — each is helper-only.
- **check.go** `RunCheck` calls `runValidate` (`validate.go`, 67 L) and
  `runSchema` (`schema.go`, 30 L). No other callers.
- **status.go:93** `RunStatus` calls `runModeSet`/`runModeRecommend`
  (`mode.go`, 114 L). Tests live in `mode_cmd_test.go` (5.8K).
- **report.go** `RunReport` (lines 91–110) branches to `runReplay`
  (`replay.go`), `runDiff` (`diff.go`), `runServe` (`serve.go`, 214 L),
  `runWatch` (`watch.go`, 136 L). `runWatch` depends on `runWatchSSE`
  (`watch_sse.go`) — SSE server helper, part of the report/watch cluster.
- **config_loader.go** (~746 L) accepts `.json` configs
  (`loadConfigFromPathLayer` `.json` case + "legacy JSON config is deprecated"
  warning), honors `SPECD_CONFIG_FORMAT=json`
  (`configFormatPreference`), and format-filters candidates
  (`configPathFormat`, `selectConfigCandidate`).
- **state.go** `migrate()` (lines 352–384) is **already minimal**: it stamps
  `schemaVersion` to the current `SchemaVersion = 6`, back-fills `revision`,
  rejects `sv > 6`. There is **no heavy v5→v6 conversion code** — the action
  plan's premise here is stale. Only an audit + floor decision remain.

## 3. Proposed design and end-to-end flow

- **A1 — validate/schema → check.go:** inline `runValidate` and `runSchema`
  (plus any file-local helpers they own) into `check.go`; delete
  `validate.go`, `schema.go`. `RunCheck` output byte-identical.
- **A2 — mode → status.go:** inline `runModeSet`, `runModeRecommend` into
  `status.go`; move `mode_cmd_test.go` cases into `status_test.go` (or a
  reporting test file already exercising `RunStatus`); delete `mode.go`,
  `mode_cmd_test.go`. Note: `internal/core/mode.go` /
  `internal/core/mode_recommend.go` are the state machine — **out of scope**,
  keep. Only the `cmd`-layer wrappers move.
- **A3 — report cluster:** consolidate `runReplay`, `runDiff`, `runServe`,
  `runWatch`, `runWatchSSE` into a single reporting submodule
  `internal/cmd/report_actions.go` (kept beside `report.go` for readability
  given serve/watch/SSE bulk), delete `replay.go`, `diff.go`, `serve.go`,
  `watch.go`, `watch_sse.go`. `RunReport` dispatch unchanged.
- **B — YAML-only config:** in `loadConfigFromPathLayer` drop the `.json`
  `json.Unmarshal` case so `.json` falls through to "unsupported config
  extension"; remove the legacy-JSON deprecation-warning branch; drop the
  `json` option from `configFormatPreference` (accept `yaml` only); simplify
  `configPathFormat`/`selectConfigCandidate` to the single YAML format. Remove
  now-dead `encoding/json` import if unused.
- **C — state floor:** keep `migrate()` as-is (already minimal); document
  `SchemaVersion = 6` as the absolute floor. Decision D1 below governs whether
  to also reject `sv < 6`.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Deleted files:** `validate.go`, `schema.go`, `mode.go`,
  `mode_cmd_test.go`, `diff.go`, `replay.go`, `serve.go`, `watch.go`,
  `watch_sse.go`.
- **New file:** `internal/cmd/report_actions.go` (report sub-actions).
- **Behavior contract:** `check`, `status`, `report` (all flags:
  `--history`/`--diff`/`--serve`/`--watch`) produce byte-identical output and
  exit codes. No registry entry added or removed.
- **Config contract change (breaking, intended for v0.2.0):** `config.json` /
  `SPECD_CONFIG_FORMAT=json` no longer supported; `config.yml`/`.yaml` only.
- **Dependencies:** none new. Internal `core.*` untouched except the `state.go`
  audit and the `config_loader.go` YAML-only edit.

## 5. Invariants, security, errors, observability, compatibility, rollback

- Integrity core untouched — no gate, lock, or evidence path modified.
- `state.json` schema unchanged; `SchemaVersion` stays 6 (no bump).
- JSON config removal is the only user-visible break; surfaced as a clear
  "unsupported config extension .json" error, not a silent skip.
- **Rollback:** revert the consolidation commit(s); deleted logic is preserved
  verbatim in git history and moved (not rewritten), so revert is mechanical.

## 6. Acceptance criteria and validation commands

- All nine target files deleted; `report_actions.go` present; no dangling
  references (`grep -rn 'runValidate\|runSchema\|runModeSet\|runModeRecommend\|runDiff\|runReplay\|runServe\|runWatch\|runWatchSSE'`
  resolves only inside the consolidated files).
- Registry unchanged (diff on `registry.go` empty).
- `.json` config path yields an "unsupported config extension" error; no
  `encoding/json` usage remains in `config_loader.go` for config parsing.
- `go test ./internal/cmd/... -run 'Check|Status|Report' -race -count=2`
- `go test ./internal/core/... -run 'Config|State|Schema' -race -count=2`
- `make ci` green (build + vet + full suite, no regressions).

## 7. Open decisions and deviations

- **D1 — state floor strictness:** `migrate()` currently *accepts* `sv < 6`
  and stamps up. Options: (a) keep lenient stamp-up (safest, zero data risk);
  (b) reject `sv < 6` to make 6 a hard floor per the plan. **Recommend (a)** —
  the migration is already trivial and rejecting old state breaks existing
  `.specd/` trees for no readability gain. Confirm before changing.
- **DV1 — report submodule path:** plan allowed "merge into report.go *or* a
  dedicated submodule". Chose `report_actions.go` (submodule) — serve+watch+SSE
  bulk (~450 L) would bloat `report.go`.
- **DV2 — state.go scope:** action plan assumed heavy v5→v6 migration code to
  strip; verification shows none exists. Task C reduced to audit + document.
