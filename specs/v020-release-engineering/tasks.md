# V12 Tasks — Release Engineering

Plan coverage: P6.4. Dependencies: V1–V11 (terminal spec).

## Wave 1 — Migration tooling

- [ ] `specd migrate`: one-shot config/state migration wrapper (idempotent,
  reports available new config blocks, never writes policy content);
  registry + CommandMeta + parity tests.
- [ ] Upgrade e2e: v0.1.x-initialized fixture repo → `specd migrate` → every
  v0.2.0 command runs correctly.
- **Validation:** `go test ./internal/cmd/... -run 'Migrate|Upgrade' -race -count=2`

## Wave 2 — Docs + benchmarks (depends on Wave 1)

- [ ] Docs sweep: command-reference, user-guide, validation-gates,
  agent-integration, mcp-guide, AGENTS.md template — every new v0.2.0
  command/gate/tool covered; docs-parity tests green.
- [ ] CHANGELOG: Keep-a-Changelog, v0.2.0 section, breaking changes called
  out (target: none).
- [ ] `make bench` vs v0.1.x; refresh `docs/agent-harness-baselines.md`;
  ±10% floor held.
- **Validation:** `make docs-lint && make bench && make cover-check`

## Wave 3 — Release gates (depends on Wave 2)

- [ ] V8 threat-model refresh confirmed merged (hard gate).
- [ ] Success-metrics table verification: each plan Part III metric has a
  green measuring test in CI (verify success, catch rate, mode-switch,
  ingestion coverage, cost attribution, eval coverage, observe→midreq).
- [ ] Install flow: `bash scripts/install_test.sh` SHA256 re-verified;
  goreleaser matrix unchanged (dry run).
- [ ] Full gate: `make ci` green, race-clean, `-count=2` stable, floors
  held/raised.
- **Validation:** `make ci`

## Rollout & cleanup

- [ ] PR to `main`, tag v0.2.0 on `main` post-merge; release notes from
  CHANGELOG.
- **Rollback:** untag/yank release; v0.1.x binaries unaffected (documented
  downgrade caveats for v6 state).
- **Completion evidence:** tagged release; upgrade e2e + metrics verification
  committed.
