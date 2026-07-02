# V11 — Team Harness Sharing & Unified Dashboard

## 1. Purpose and requirement coverage

Make the configured harness a shareable, versioned team asset with
supply-chain quarantine, plus the unified dashboard and git-based pack
registry. Covers plan tasks **P6.1** (P0), **P6.2** (P1), **P6.3** (P2) — the
SDLC "team harness sharing" ecosystem concept, without hosted services.

## 2. Verified current state

- Serve/dashboard surfaces: `serve.go`, `docs/dashboard.md`, SSE watch stream.
- Pack machinery: `internal/pack/` + `specd init --pack`.
- stdlib-exec `git` precedent (V2 changed-file scoping, existing backends).
- Artifacts to bundle exist from V2 (guardrails), V5 (eval suites), V4
  (routing), V9 (deploy templates), V8 (roles).

## 3. Proposed design and end-to-end flow

- **Harness bundle (P6.1):** `.specd/harness/` (guardrails, eval suites,
  routing, deploy templates, roles) with `harness.json` manifest
  (name, version, provenance). `specd harness push/pull <git-url>` via
  stdlib-exec `git`. Pull verifies the manifest, refuses to overwrite local
  modifications without `--force`, and **quarantines** imported `command`
  checks: listed, disabled until explicitly enabled — supply-chain guard.
- **Unified dashboard (P6.2):** extend `specd serve` — server-rendered HTML +
  existing SSE, rendering conductor sessions, orchestrator waves, eval trends,
  cost, escalations from state/ledgers. No JS framework, no new deps.
  `specd dashboard` alias with `--mode` filter.
- **Pack registry (P6.3):** `specd init --pack <git-url|name>` — named packs
  resolve via a registry index that is itself a git repo (no hosted service).
  Same quarantine rules as P6.1; checksum pinning in a lockfile.

## 4. Interfaces, contracts, data, configuration, dependencies

- **New artifacts:** `.specd/harness/harness.json`, pack lockfile.
- **New commands:** `harness push|pull`, `dashboard` (registry discipline).
- **Dependencies:** V2, V4, V5, V8, V9 (bundle contents); V6/V7 (dashboard
  data). **Dependents:** V12.

## 5. Invariants, security, errors, observability, compatibility, rollback

- Quarantine is the load-bearing security property: any imported executable
  check (eval `command`, deploy step, custom gate) arrives disabled with a
  visible list; enabling is an explicit per-item command recorded in the
  decision log.
- Pull never overwrites local modifications silently (`--force` +
  diff summary).
- Checksum pinning: pulled content hash recorded; mismatch on re-pull → hard
  fail.
- Dashboard renders only from local state/ledgers; zero outbound network.
- git exec through scrubbed env; remote URL validation (no arbitrary
  `ext::`-style transports).
- **Rollback:** delete `.specd/harness/`; dashboard is read-only.

## 6. Acceptance criteria and validation commands

- Push/pull round-trip e2e against a local bare-repo fixture; local-modify →
  pull refusal → `--force` path.
- Quarantine tests: imported command checks disabled, listed, individually
  enableable; enable recorded.
- Dashboard renders deterministically from fixtures; no outbound calls
  (asserted).
- Remote pack e2e vs local fixture repo; checksum pin mismatch → fail.
- Hostile manifest/pack fixtures (P4.4 cadence, same PR).
- `go test ./internal/core/... ./internal/pack/... ./internal/cmd/... -run 'Harness|Dashboard|Registry' -race -count=2`

## 7. Open decisions and deviations

- Path deviation DV1. MCP marketplace / org web dashboard / partner program
  stay deferred to v0.3.0 (§5.6).
- Open: harness version conflict on pull (local v2 vs remote v1). Decision:
  refuse downgrade without `--force`; versions compared as manifest ints.
