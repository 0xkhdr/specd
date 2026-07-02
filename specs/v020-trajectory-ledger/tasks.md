# V3 Tasks — Trajectory Ledger

Plan coverage: P1.2. Dependencies: V1. Dependents: V5, V6.

## Wave 1 — Ledger core

- [ ] `internal/core/trajectory.go`: event type (digest-only args), append with
  O_APPEND + fsync + spec lock, monotonic `seq`.
- [ ] Reader: line validation (NUL, oversize), corrupt-tail tolerance,
  missing-file = zero events.
- **Validation:** `go test ./internal/core/... -run Trajectory -race -count=1`

## Wave 2 — Producers (depends on Wave 1)

- [ ] Pinky: append events from report/progress paths
  (`internal/core/pinky_report.go`).
- [ ] MCP: middleware in `internal/mcp/tools.go` around specd tool dispatch.
- [ ] New command `specd trace append <spec> --tool --outcome [--task]`:
  `internal/cmd/trace.go` + registry + CommandMeta + JSON contract test.
- **Validation:** `go test ./internal/cmd/... ./internal/mcp/... -run 'Trace|Parity' -race`

## Wave 3 — Stress + adversarial (depends on Wave 2)

- [ ] Concurrent-append stress (extend stress harness): N writers, assert no
  loss/interleave, seq monotonic.
- [ ] Secrets-hygiene test: raw arg values never appear in ledger bytes.
- [ ] FakeClock determinism test; `-count=2` order independence.
- **Validation:** `make stress && go test ./internal/core/... -count=2`

## Rollout & cleanup

- [ ] Docs: `docs/agent-integration.md` (event schema, producer contract),
  command-reference (`trace append`), CHANGELOG; parity tests green.
- **Rollback:** producers behind no-op when ledger dir unwritable; file inert.
- **Completion evidence:** `make ci` green; stress + hygiene tests committed.
