# Tasks W5 — Surface & Config Reconciliation

> Dogfooded. Depends on W4 (memory functional before folding it in) and W2 (surface stable).

## Wave 1 — verb reconciliation

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P5.1 | craftsman | `internal/cmd/registry.go`, `docs/charter.md`, `specs/01-product-philosophy-core/spec.md` | — | `go test ./internal/cmd -run TestRegistryMatchesCharter` | `triage` cut; `memory` folded by superseding ADR recorded via `specd decision --text`; bare `specd` verb count == Spec 01 R1.5 == charter; test CI-blocking |

## Wave 2 — config

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P5.2 | craftsman | `internal/core/config.go`, `internal/cmd/registry.go`, `internal/core/embed_templates/config.yml` | P5.1 | `go test ./internal/core -run TestConfigFailLoud && go test ./internal/cmd -run TestInitSeedsConfig` | loads `config.yml` (not `project.yml`); corrupt file → non-zero exit with message, no silent defaults; `init` seeds template; `--agent` wired or removed per ADR |

## Wave 3 — CLI consistency

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P5.3 | craftsman | `internal/cmd/registry.go`, `internal/context/manifest.go` | — | `go test ./internal/cmd ./internal/context -run 'TestTaskSlugShape|TestManifestPaths|TestCheckSummary'` | `task <slug> <id>` shape; manifest paths repo-relative incl. `.specd/`, golden-tested; `check` one-line green summary, `--json` unchanged |
| P5.4 | validator | `internal/cmd/e2e_test.go` | P5.1, P5.2, P5.3 | `go test ./internal/cmd -run TestE2E` | e2e regression spec updated and green across reconciled surface |

## Traceability (task → requirement → finding)
- P5.1 → R5.1 → F7 · P5.2 → R5.2 → F10 · P5.3 → R5.3 → F14 · P5.4 → R5.1–R5.3
