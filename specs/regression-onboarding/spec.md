# S6 — Onboarding Regression

## 1. Purpose and requirement coverage

Guarantee `init` is idempotent and emits byte-identical receipts, packs apply
deterministically, and host detection/MCP registration is stable. Covers **R6**.

## 2. Verified current state

- `init` implementation: `internal/cmd/init.go` (large surface), tested by
  `init_test.go`, `init_claude_runtime_test.go`, `initpack_test.go`,
  `onboarding_test.go`. Benchmarks in `init_benchmark_test.go`.
- Init planning/scaffold: `internal/core/initplan.go`, `scaffold.go`,
  `config_scaffold.go`; embedded packs in `internal/pack/` and
  `internal/core/embed_templates/` (byte-identical embeds — `embed_drift_test.go`).
- Host detection + registration: `internal/integration/`, MCP host embeds in
  `internal/mcp/embed_hosts/`, `handshake` command (`internal/cmd/handshake.go`).
- Deterministic-output gate: `make perf-gate` runs
  `-run 'Deterministic|BenchmarkContract|ManifestDisabledMode' -count=2` over
  `internal/cmd`, `internal/mcp`, `internal/context` (Makefile:86).

## 3. Proposed design and end-to-end flow

Tests assert: running `init` twice yields byte-identical receipts (no
timestamps/random ordering); embedded pack templates are byte-stable
(`embed_drift_test.go`); host detection resolves the same host given the same
environment; MCP registration writes a stable config. The deterministic gate is
the primary signal via `make perf-gate` at `-count=2`.

## 4. Interfaces, contracts, data, configuration, dependencies

- **Stable:** init receipt bytes; embedded template bytes; host-detection output;
  MCP registration config shape.
- **Dependencies:** S5 (MCP registration depends on stable MCP surface).

## 5. Invariants, security, errors, observability, compatibility, rollback

- Idempotency: re-running `init` does not mutate existing config unexpectedly.
- **Compatibility:** embedded packs frozen unless intentionally versioned.
- **Rollback:** `init` should be safe to re-run; tests are additive.

## 6. Acceptance criteria and validation commands

- `make perf-gate` passes (byte-identical receipts at `count=2`).
- `go test ./internal/cmd/... -run 'Init|Onboarding|Pack' -race -count=1` passes.
- `go test ./internal/core/... -run 'EmbedDrift|Scaffold|InitPlan' -race` passes.

## 7. Open decisions and deviations

- Plan lists `internal/cmd/init.go` and `make perf-gate` — both verified.
- Deviation: onboarding now spans `handshake` + host embeds under
  `internal/mcp/embed_hosts/`, not just `init`; scope widened accordingly.
