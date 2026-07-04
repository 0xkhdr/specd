# Tasks W6 — Production Hardening & Release

> Dogfooded — and the dogfood evidence *is* the release gate (P6.3).

## Wave 1 — CI & identity

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P6.1 | craftsman | `.github/workflows/ci.yml`, `.github/workflows/release.yml` | — | `gh run watch --exit-status $(gh run list -b fresh-start -L1 --json databaseId -q '.[0].databaseId')` | green pipeline: build+vet+`test -race`+fuzz smoke+e2e; release job builds static `CGO_ENABLED=0` binaries, version stamped |
| P6.2 | craftsman | `main.go`, `internal/cmd/registry.go` | — | `go run -ldflags "-X main.version=v0-test" . --version \| grep v0-test` | `--version` prints stamp; unstamped prints `dev`; same value in `handshake` + MCP serverInfo |

## Wave 2 — dogfood & docs

| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| P6.3 | auditor | `.specd/specs/` | P6.1, P6.2 | `./specd report --pr \| grep -q '100%'` | repo `.specd/` carries W1–W6 specs, every task closed via `specd task complete` with real evidence at real HEAD; `report --pr` pasted into release PR |
| P6.4 | craftsman | `README.md`, `docs/` | — | `sh scripts/check-links.sh` | charter + conductor quickstart + orchestrated quickstart + MCP setup per host adapter; links resolve |
| P6.5 | validator | — (release check) | P6.3, P6.4 | `./specd status --json \| grep -vq '"open"'` (W0/W1/P2.1 specs all complete) | R6.5 ship gate: nothing releases while W0+W1+P2.1 open |

## Traceability (task → requirement)
- P6.1 → R6.1 · P6.2 → R6.2 · P6.3 → R6.3 · P6.4 → R6.4 · P6.5 → R6.5
