# Tasks 06 — Agent-Agnostic Integration

> **Build waves:** E (T6.1–T6.7). See `specs/progress.md`.
> **Depends on domains:** 02. **Unblocks:** 07, 09.

## Wave 1 — roles & steering scaffold

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T6.1 | craftsman | `internal/core/embed_templates/roles/*`, `internal/core/scaffold.go` | — | `go run . init && test $(ls .specd/roles \| wc -l) -eq 4` | four roles scaffolded |
| T6.2 | craftsman | `internal/core/embed_templates/steering/*` | T6.1 | `test -f .specd/steering/workflow.md` | steering constitution scaffolded |
| T6.3 | craftsman | `internal/core/agents.go` | — | `go test ./internal/core -run TestAgentsMergePreservesUser` | marker-merge preserves user content |

## Wave 2 — adapters & injection

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T6.4 | craftsman | `internal/integration/registry.go` | — | `go test ./internal/integration -run TestSnippetFallback` | snippet emitted with no adapter |
| T6.5 | craftsman | `internal/integration/<host>.go` | T6.4 | `go test ./internal/integration -run TestAdapterConformance` | reference adapter passes the kit |
| T6.6 | craftsman | role-injection wiring | T6.1 | `go test ./internal/core -run TestRolePromptDedup` | prompt emitted once per response |
| T6.7 | validator | `internal/integration/conformance_test.go` | T6.5 | `go test ./internal/integration -run TestAdapterConformance` | idempotent install; ownership recorded |

## Traceability (task → requirement)
- T6.1 → R6.1, R6.8 · T6.2 → R6.3 · T6.3 → R6.4 · T6.4 → R6.5 · T6.5 → R6.6, R6.7 · T6.6 → R6.1, R6.2 · T6.7 → R6.7
