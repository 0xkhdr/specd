# Clean Deprecation — Tasks

Each task moves logic verbatim (no rewrite) and gates on tests + build. Verify
step 0 before deleting anything.

## T0 — Trace & confirm (gate: grep clean)
- [ ] Confirm no callers outside claimed sites for every target function:
      `grep -rn 'runValidate\|runSchema\|runModeSet\|runModeRecommend\|runDiff\|runReplay\|runServe\|runWatch\|runWatchSSE' internal/ --include='*.go'`
- [ ] Confirm none of the target files appear in `internal/cmd/registry.go`.
- **Verify:** caller map matches spec §2; else stop and re-scope.

## T1 — validate.go + schema.go → check.go (gate: Check tests)
- [ ] Inline `runValidate` (validate.go) and `runSchema` (schema.go) + their
      file-local helpers into `check.go`.
- [ ] Delete `internal/cmd/validate.go`, `internal/cmd/schema.go`.
- **Verify:** `go test ./internal/cmd/... -run 'Check' -race -count=2`; build.

## T2 — mode.go + mode_cmd_test.go → status.go (gate: Status tests)
- [ ] Inline `runModeSet`, `runModeRecommend` into `status.go`.
- [ ] Move `mode_cmd_test.go` cases into `status_test.go` (or existing
      RunStatus test file).
- [ ] Delete `internal/cmd/mode.go`, `internal/cmd/mode_cmd_test.go`.
- [ ] Do NOT touch `internal/core/mode.go` / `mode_recommend.go` (out of scope).
- **Verify:** `go test ./internal/cmd/... -run 'Status|Mode' -race -count=2`.

## T3 — report cluster → report_actions.go (gate: Report tests)
- [ ] Create `internal/cmd/report_actions.go`; move `runReplay`, `runDiff`,
      `runServe`, `runWatch`, `runWatchSSE` + local helpers into it.
- [ ] Delete `replay.go`, `diff.go`, `serve.go`, `watch.go`, `watch_sse.go`.
- [ ] `RunReport` dispatch in `report.go` unchanged.
- **Verify:** `go test ./internal/cmd/... -run 'Report' -race -count=2`;
      manual `report --serve`/`--watch` smoke if server tests thin.

## T4 — YAML-only config (gate: Config tests)
- [ ] `config_loader.go`: drop `.json` parse case (fall through to
      "unsupported config extension"); remove legacy-JSON warning branch.
- [ ] `configFormatPreference`: accept `yaml` only; drop `json`.
- [ ] Simplify `configPathFormat`/`selectConfigCandidate` to single format.
- [ ] Remove dead `encoding/json` import if now unused.
- **Verify:** `go test ./internal/core/... -run 'Config' -race -count=2`;
      `.json` config produces clear unsupported-extension error.

## T5 — state floor audit (gate: State tests, decision D1)
- [ ] Confirm `migrate()` already minimal (no v5→v6 body); document
      `SchemaVersion = 6` as floor in the doc comment.
- [ ] Apply D1: default keep lenient stamp-up; reject `sv < 6` only if user
      confirms strict floor.
- **Verify:** `go test ./internal/core/... -run 'State|Schema' -race -count=2`.

## T6 — Full gate (gate: make ci)
- [ ] `grep` for any dangling reference to deleted symbols/files — none remain.
- [ ] `go test ./internal/cmd/... -run 'Check|Status|Report' -race -count=2`
- [ ] `make ci` green.
- **Verify:** clean build, full suite pass, `registry.go` diff empty.
