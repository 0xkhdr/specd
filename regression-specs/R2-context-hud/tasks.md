# Tasks R2 — Context HUD

> **Build wave:** R-B. See `regression-specs/progress.md`.
> **Depends on domains:** 08 (`Manifest`/estimator/budget), 02 (spec `mode`), 10 (flag plumbing).
> **Unblocks:** — (leaf surface).

## Wave 1 — pure render

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| TR2.1 | craftsman | `internal/context/hud.go` | — | `go test ./internal/context -run TestHUDRender` | `RenderHUD(Manifest)` prints per-file bytes + tokens, a total row, and mode/tier; pure function, no new estimation pass |

## Wave 2 — wire + guard

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| TR2.2 | craftsman | `internal/core/commands.go`, `internal/cmd/registry.go` | TR2.1 | `go run . new demo && go run . context <task> --hud \| grep -qi total` | `--hud` flag registered on `context`; handler branches to `RenderHUD`; `--json` and default paths unchanged; `--hud --json` rejected as usage error |
| TR2.3 | validator | `internal/context/hud_test.go` | TR2.2 | `go test ./internal/context -run TestHUDMatchesJSON` | byte/token totals from `--hud` equal those from `--json` for the same task (RH.3) |

## Traceability (task → requirement)
- TR2.1 → RH.1, RH.2 · TR2.2 → RH.1, RH.4 · TR2.3 → RH.3
