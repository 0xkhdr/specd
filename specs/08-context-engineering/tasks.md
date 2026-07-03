# Tasks 08 — Context Engineering

> **Build waves:** E (T8.1–T8.4), F (T8.5–T8.7). See `specs/progress.md`.
> **Depends on domains:** 04, 02. **Unblocks:** 03 (budget gate), 07, 09.

## Wave 1 — engine

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T8.1 | craftsman | `internal/context/manifest.go` | — | `go test ./internal/context -run TestBuildManifest` | four item modes; deterministic order |
| T8.2 | craftsman | `internal/context/estimate.go` | — | `go test ./internal/context -run TestEstimateNoLLM` | pure heuristic; stable output |
| T8.3 | craftsman | `internal/core/pinky_context.go` | T8.1 | `go build ./... && go vet ./...` | adapter compiles; no import cycle |

## Wave 2 — surfaces & budget

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T8.4 | craftsman | `internal/cmd/context.go`, `internal/cmd/dispatch.go` | T8.1 | `diff <(go run . context demo T1 --json) <(go run . next demo --dispatch --json \| jq .items)` | surfaces share the engine |
| T8.5 | craftsman | `internal/context/budget.go`, `internal/core/gates/contextbudget.go` | T8.2, T8.1 | `SPECD_MAX_CONTEXT_TOKENS=10 go run . check demo` | over-budget manifest fails gate |
| T8.6 | craftsman | `docs/context.md` | T8.1 | `grep -q read-targeted docs/context.md` | first-class doc maps modes↔paper context types |
| T8.7 | validator | `internal/context/manifest_test.go` | T8.1 | `go test ./internal/context -run TestManifestValidate` | malformed manifest rejected |

## Traceability (task → requirement)
- T8.1 → R8.1, R8.2 · T8.2 → R8.3 · T8.3 → R8.6 · T8.4 → R8.1 · T8.5 → R8.4 · T8.6 → R8.2 · T8.7 → R8.5
