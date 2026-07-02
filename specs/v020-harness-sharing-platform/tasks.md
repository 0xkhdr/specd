# V11 Tasks — Harness Sharing & Platform

Plan coverage: P6.1, P6.2, P6.3. Dependencies: V2, V4, V5, V6, V7, V8, V9.
Dependents: V12.

## Wave 1 — Harness bundle + quarantine (P6.1)

- [ ] `.specd/harness/` layout + `harness.json` manifest (name, version,
  provenance) parse/validate.
- [ ] `specd harness push/pull <git-url>` via stdlib-exec git (scrubbed env,
  URL validation); pull refuses local-modification overwrite without
  `--force`.
- [ ] Quarantine: imported `command` checks disabled + listed; explicit
  per-item enable recorded in decision log; tests.
- [ ] Round-trip e2e vs local bare-repo fixture; hostile manifest fixtures.
- **Validation:** `go test ./internal/core/... ./internal/cmd/... -run Harness -race -count=2`

## Wave 2 — Unified dashboard (P6.2, depends on Wave 1)

- [ ] Extend `specd serve`: server-rendered views for conductor sessions,
  waves, eval trends, cost, escalations from state/ledgers; SSE reuse; no
  new deps.
- [ ] `specd dashboard` alias with `--mode` filter.
- [ ] Deterministic render-from-fixtures tests; no-outbound-network
  assertion.
- **Validation:** `go test ./internal/cmd/... -run 'Serve|Dashboard' -race`

## Wave 3 — Pack registry (P6.3, depends on Wave 1)

- [ ] Registry index resolution (git repo index); `init --pack <git-url|name>`.
- [ ] Checksum pinning lockfile; mismatch → hard fail; quarantine rules
  shared with Wave 1.
- [ ] Remote pack e2e vs local fixture repo.
- **Validation:** `go test ./internal/pack/... -run Registry -race -count=2`

## Rollout & cleanup

- [ ] Docs: user-guide harness-sharing section, command-reference
  (harness/dashboard), SECURITY.md quarantine model, CHANGELOG; parity green.
- **Rollback:** delete `.specd/harness/`; registry unused without invocation.
- **Completion evidence:** `make ci` green; quarantine + checksum tests
  committed.
