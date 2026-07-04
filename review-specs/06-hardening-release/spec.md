# Wave 6 — Production Hardening & Release

> **Order:** 7 / 7 · **Depends:** W0–W5 all closed
> **Findings:** none new — this wave proves closure of all of them
> **Sources:** PROJECT.md §8 Wave P6, BUILD_REVIEW.md §5 Wave 6
> **Files:** `.github/workflows/`, `main.go`, `internal/cmd/registry.go`, `README.md`, `docs/`, `.specd/`

The release gate is itself evidence-shaped: CI proves the invariants on every push,
the binary carries its identity, and — the dogfood gate — this repo's own `.specd/`
carries waves W1–W6 closed via `specd task complete` with real evidence. A
spec-discipline tool ships only when it has survived its own discipline.

## 1. Purpose & principles

- **Principles owned:** P3 (dogfood evidence), P7 (deterministic release reporting).
- **Harness components:** observability (CI, version), instructions (docs).

## 2. Requirements (EARS)

- **R6.1** When any push lands, CI shall run build + vet + `test -race` + fuzz smoke
  (parser round-trip) + the e2e lifecycle test; the release job shall build static
  binaries (`CGO_ENABLED=0`) with the version stamped via `-ldflags`.
- **R6.2** When `specd --version` runs (and in `handshake` output), the system shall
  print the build-stamped version; an unstamped dev build prints `dev`.
- **R6.3** When the release PR is prepared, this repo's `.specd/` shall carry specs for
  waves W1–W6, each task closed via `specd task complete` with a passing evidence
  record at a real HEAD (no `--unverified` for craftsman tasks), and
  `specd report --pr` output shall be pasted into the release PR showing 100%
  completion with evidence for the hardening waves.
- **R6.4** When the docs pass completes, README + docs shall cover: the charter, a
  conductor-flow quickstart, an orchestrated-flow quickstart, and MCP setup per host
  adapter; all internal links shall resolve.
- **R6.5** While W0 + W1 + P2.1 remain unclosed, the system shall not be released
  (the "what NOT to do" ship gate, restated as a requirement).

## 3. Design

- **CI (R6.1):** extend the scaffolded `.github/` workflows — no new tooling. Fuzz
  smoke = bounded `go test -fuzz=FuzzRoundTrip -fuzztime=30s`. Release matrix:
  linux/darwin × amd64/arm64.
- **Version (R6.2):** one `var version = "dev"` in `main.go`, stamped by
  `-ldflags "-X main.version=..."`; surfaced through the registry so `--version`,
  `handshake`, and MCP `serverInfo` all read the same value.
- **Dogfood gate (R6.3):** this is the acceptance test for the whole review-specs
  program — the evidence ledger under `.specd/specs/` is the proof the concept↔
  functionality gap is closed. The PR body embeds the deterministic `report --pr`
  output verbatim (computed, never generated — P7).

## 4. Invariants preserved

- Zero runtime deps; single static binary; version stamping adds no dependency.
- Deterministic reporting: the release report is a projection of `state.json` + ledger.
