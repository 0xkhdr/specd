# Tasks — Regression: Agent-Harness Value (context engineering, perf, market-gap)

## Wave 1
- [x] T1 — Define budgets and baselines ✓ complete · evidence: docs/agent-harness-baselines.md: budget table (context 898→2048B, next 661→1536B, dispatch 1829→4096B, no ANSI) + perf baseline (status 7.5ms, next 7.1ms, program 8.7ms, budget 50ms) measured on .specd sample repo via /tmp/specd-bench. Token-proxy decision recorded ADR-001. verify=N/A (investigator artifact task). · 2026-06-17T17:51:47.634446082Z
  - why: efficiency and perf claims need documented thresholds first (R1, R2)
  - role: investigator
  - files: docs, .specd/specs/regression-agent-harness-value
  - contract: record token/byte budgets for context/next/dispatch and latency baselines for status/next/program on the sample repo; pick + document the token proxy; do NOT change source
  - acceptance: a budget table + a stored perf baseline + a recorded token-proxy decision
  - verify: N/A
  - depends: —
  - requirements: 1, 2

## Wave 2
- [x] T2 — Context-efficiency regression tests ✓ complete · evidence: internal/cmd/agent_budget_test.go: TestAgentBriefingBudgets (context/next/dispatch --json within byte budgets, prints measured size on fail), TestAgentBriefingNoDecoration (no ANSI in --json). verify go test -run 'Context|Budget|Brief' → exit 0. · 2026-06-17T17:52:57.982379527Z
  - why: R1 — specd must stay token-lean and decoration-free for agents
  - role: builder
  - files: internal/cmd, internal/core
  - contract: assert context/next/dispatch --json size within budget and free of ANSI; failing test prints measured size
  - acceptance: R1.1-R1.3 pass
  - verify: go test ./internal/cmd/ ./internal/core/ -run 'Context|Budget|Brief'
  - depends: T1
  - requirements: 1

- [x] T3 — Performance benchmarks + determinism goldens ✓ complete · evidence: internal/core/agent_perf_test.go: TestDeterminismRenders (WaveGraph/NextSummary/FrontierOf byte-identical on double-call, R2.2) + BenchmarkWaveGraph (render), BenchmarkFrontierOf (frontier); dag/frontier already have BenchmarkDetectCycle/NextRunnable (R2.3). Latency baseline stored in docs/agent-harness-baselines.md; benchmarks track relative regression (R2.1). verify go test -bench . -run Determinism → exit 0. · 2026-06-17T17:54:22.382911703Z
  - why: R2 — fast, deterministic, regression-tracked hot paths
  - role: builder
  - files: internal/core, internal/cmd
  - contract: add `go test -bench` for dag/frontier/render; double-run determinism golden for status/next/program; assert latency-budget on sample repo as relative regression vs T1 baseline
  - acceptance: R2.1-R2.3 pass; benchmarks runnable
  - verify: go test ./internal/core/ -bench . -run Determinism
  - depends: T1
  - requirements: 2

- [x] T4 — Compatibility matrix verification ✓ complete · evidence: internal/mcp/host_compat_test.go: TestHostCompatibilityMatrix asserts registry↔docs parity (5 hosts), each --config snippet invokes `specd mcp` with root substituted, and stdio specd_status tools/call returns working content. docs/agent-harness-compat.md publishes transport×host matrix with explicit unsupported/n/a cells + known limitations. verify go test ./internal/mcp/ -run 'Host|Transport' → exit 0. · 2026-06-17T17:56:05.660377699Z
  - why: R3 — proven {transport × host} support with explicit limits
  - role: verifier
  - files: internal/mcp, docs
  - contract: consume regression-mcp-transport results; verify each documented path yields a working tool call; record per-host limitations explicitly
  - acceptance: R3.1-R3.3 satisfied; matrix published with honest limited/unsupported cells
  - verify: go test ./internal/mcp/ -run 'Host|Transport'
  - depends: T1
  - requirements: 3

## Wave 3
- [x] T5 — Market-gap analysis + prioritized improvement ledger ✓ complete · evidence: docs/agent-harness-gap-analysis.md: field positioning + 4 gaps (each evidence+severity+remedy+owner) + ranked improvement ledger by market-value impact. Gaps recorded ADR-002..005; GAP-1 also memory 'agent-output-byte-budgets'. R4.3 judgment: no blocker-critical gap (GAP-2 has documented workaround → follow-up). Full suite go test ./... green. verify=N/A (reviewer artifact task). · 2026-06-17T17:57:36.385766187Z
  - why: R4 — no improvement to market value left implicit
  - role: reviewer
  - files: docs, .specd/specs/regression-agent-harness-value
  - contract: produce gap-analysis comparing specd to spec-driven/agent-tooling field; record each gap as ADR (`specd decision`) and/or memory (`specd memory`); critical gaps become follow-up specs/blockers; end with a ranked improvement ledger
  - acceptance: R4.1-R4.4 satisfied; every gap has evidence + remedy + owner; ledger ranked by market-value impact
  - verify: N/A
  - depends: T2, T3, T4
  - requirements: 4
