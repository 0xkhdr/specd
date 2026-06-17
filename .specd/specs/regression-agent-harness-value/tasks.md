# Tasks — Regression: Agent-Harness Value (context engineering, perf, market-gap)

## Wave 1
- [ ] T1 — Define budgets and baselines
  - why: efficiency and perf claims need documented thresholds first (R1, R2)
  - role: investigator
  - files: docs, .specd/specs/regression-agent-harness-value
  - contract: record token/byte budgets for context/next/dispatch and latency baselines for status/next/program on the sample repo; pick + document the token proxy; do NOT change source
  - acceptance: a budget table + a stored perf baseline + a recorded token-proxy decision
  - verify: N/A
  - depends: —
  - requirements: 1, 2

## Wave 2
- [ ] T2 — Context-efficiency regression tests
  - why: R1 — specd must stay token-lean and decoration-free for agents
  - role: builder
  - files: internal/cmd, internal/core
  - contract: assert context/next/dispatch --json size within budget and free of ANSI; failing test prints measured size
  - acceptance: R1.1-R1.3 pass
  - verify: go test ./internal/cmd/ ./internal/core/ -run 'Context|Budget|Brief'
  - depends: T1
  - requirements: 1

- [ ] T3 — Performance benchmarks + determinism goldens
  - why: R2 — fast, deterministic, regression-tracked hot paths
  - role: builder
  - files: internal/core, internal/cmd
  - contract: add `go test -bench` for dag/frontier/render; double-run determinism golden for status/next/program; assert latency-budget on sample repo as relative regression vs T1 baseline
  - acceptance: R2.1-R2.3 pass; benchmarks runnable
  - verify: go test ./internal/core/ -bench . -run Determinism
  - depends: T1
  - requirements: 2

- [ ] T4 — Compatibility matrix verification
  - why: R3 — proven {transport × host} support with explicit limits
  - role: verifier
  - files: internal/mcp, docs
  - contract: consume regression-mcp-transport results; verify each documented path yields a working tool call; record per-host limitations explicitly
  - acceptance: R3.1-R3.3 satisfied; matrix published with honest limited/unsupported cells
  - verify: go test ./internal/mcp/ -run 'Host|Transport'
  - depends: T1
  - requirements: 3

## Wave 3
- [ ] T5 — Market-gap analysis + prioritized improvement ledger
  - why: R4 — no improvement to market value left implicit
  - role: reviewer
  - files: docs, .specd/specs/regression-agent-harness-value
  - contract: produce gap-analysis comparing specd to spec-driven/agent-tooling field; record each gap as ADR (`specd decision`) and/or memory (`specd memory`); critical gaps become follow-up specs/blockers; end with a ranked improvement ledger
  - acceptance: R4.1-R4.4 satisfied; every gap has evidence + remedy + owner; ledger ranked by market-value impact
  - verify: N/A
  - depends: T2, T3, T4
  - requirements: 4
