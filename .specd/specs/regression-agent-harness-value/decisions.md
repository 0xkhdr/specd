# Decisions — Regression: Agent-Harness Value (context engineering, perf, market-gap)

<!--
ADR ledger (append-only). Use `specd decision <spec> "<text>" [--supersedes ADR-NNN]`
to append. Entries are numbered monotonically and never edited. Format:

## ADR-001 — <decision summary> · 2026-06-17
**Context:** <what forced the choice>
**Decision:** <what we chose>
**Consequences:** <trade-offs, what it rules out>
**Supersedes:** <ADR-id or —>
-->

## ADR-001 — Token proxy for R1 budgets is bytes/4, not a model tokenizer. specd embeds no tokenizer; bytes is the deterministic, directly-measurable quantity and bytes/4 is the standard BPE rule-of-thumb. Budgets are therefore expressed in bytes (token column is reference-only). Latency budgets are machine-dependent so R2 asserts relative regression vs the stored baseline in docs/agent-harness-baselines.md, not absolute ms. · 2026-06-17
**Context:** TODO
**Decision:** Token proxy for R1 budgets is bytes/4, not a model tokenizer. specd embeds no tokenizer; bytes is the deterministic, directly-measurable quantity and bytes/4 is the standard BPE rule-of-thumb. Budgets are therefore expressed in bytes (token column is reference-only). Latency budgets are machine-dependent so R2 asserts relative regression vs the stored baseline in docs/agent-harness-baselines.md, not absolute ms.
**Consequences:** TODO
**Supersedes:** —

## ADR-002 — GAP-1 (medium): R1 token budgets use a bytes/4 proxy, not a model tokenizer, so token claims are approximate per-model. Remedy: offer optional tokenizer-backed measurement (e.g. tiktoken/anthropic count-tokens) behind a flag for exact budgets; keep bytes as the deterministic default. Owner: this spec's ledger; not blocking — bytes proxy is conservative and CI-stable. · 2026-06-17
**Context:** TODO
**Decision:** GAP-1 (medium): R1 token budgets use a bytes/4 proxy, not a model tokenizer, so token claims are approximate per-model. Remedy: offer optional tokenizer-backed measurement (e.g. tiktoken/anthropic count-tokens) behind a flag for exact budgets; keep bytes as the deterministic default. Owner: this spec's ledger; not blocking — bytes proxy is conservative and CI-stable.
**Consequences:** TODO
**Supersedes:** —

## ADR-003 — GAP-2 (medium-high, compatibility): no prebuilt --config snippet ships for the HTTP/SSE transport, so browser/remote MCP hosts wire the loopback endpoint by hand (docs/agent-harness-compat.md). Has a documented workaround so it is not reliability-critical and earns no blocker; remedy is a follow-up: add `specd mcp --config http` emitting an endpoint snippet. Owner: future transport spec. · 2026-06-17
**Context:** TODO
**Decision:** GAP-2 (medium-high, compatibility): no prebuilt --config snippet ships for the HTTP/SSE transport, so browser/remote MCP hosts wire the loopback endpoint by hand (docs/agent-harness-compat.md). Has a documented workaround so it is not reliability-critical and earns no blocker; remedy is a follow-up: add `specd mcp --config http` emitting an endpoint snippet. Owner: future transport spec.
**Consequences:** TODO
**Supersedes:** —

## ADR-004 — GAP-3 (medium): perf baselines (docs/agent-harness-baselines.md) and benchmarks exist but no CI gate diffs current bench against the stored baseline, so a latency/alloc regression can merge silently. Remedy: wire benchstat (or a threshold check) over BenchmarkWaveGraph/FrontierOf/DetectCycle/NextRunnable in CI against a committed baseline. Owner: ledger; not blocking. · 2026-06-17
**Context:** TODO
**Decision:** GAP-3 (medium): perf baselines (docs/agent-harness-baselines.md) and benchmarks exist but no CI gate diffs current bench against the stored baseline, so a latency/alloc regression can merge silently. Remedy: wire benchstat (or a threshold check) over BenchmarkWaveGraph/FrontierOf/DetectCycle/NextRunnable in CI against a committed baseline. Owner: ledger; not blocking.
**Consequences:** TODO
**Supersedes:** —

## ADR-005 — GAP-4 (low): .specd/steering/product.md is still TODO placeholders, so the program's market-value framing is not captured as durable steering. Remedy: fill product.md (product/users/value/out-of-scope) so gap analysis has a grounded baseline. Owner: ledger; not blocking. · 2026-06-17
**Context:** TODO
**Decision:** GAP-4 (low): .specd/steering/product.md is still TODO placeholders, so the program's market-value framing is not captured as durable steering. Remedy: fill product.md (product/users/value/out-of-scope) so gap analysis has a grounded baseline. Owner: ledger; not blocking.
**Consequences:** TODO
**Supersedes:** —
