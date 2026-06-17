# Design — Regression: Agent-Harness Value (context engineering, perf, market-gap)

## Overview
This is the program capstone. It runs only after the four component specs are green, then
measures specd as an agent-harness product across three axes — context efficiency,
performance, compatibility — and ends with an evidence-backed market-gap ledger. The output
is both passing tests (budgets/benchmarks) and durable artifacts (ADRs, memory, follow-up
specs) so improvements are tracked, not lost.

## Architecture
```
[core] [backends] [packs/schema] [cli/cmd] [mcp]  (all green, upstream deps)
                         │
                         ▼
   ┌─ context budgets (token size of briefings)        R1
   ├─ perf benchmarks + determinism (bench, golden)     R2
   ├─ compatibility matrix {transport × host}           R3
   └─ gap analysis ──► ADRs + memory + follow-up specs  R4
```

## Components and interfaces
- **briefing sizers** — measure bytes/approx-tokens of context/next/dispatch output. Contract:
  within documented budget, agent output decoration-free.
- **benchmarks** — `go test -bench` over dag/frontier/render hot paths. Contract: exposed and
  tracked, not regressed silently.
- **compat matrix** — derived from regression-mcp-transport results; documents per-host limits.
- **gap ledger** — markdown artifact + `specd decision` / `specd memory` entries.

## Data models
- Budget table: {command -> max bytes/tokens}. Latency table: {command -> max ms on sample repo}.
- Compat matrix: {transport × host -> pass | limited(note) | unsupported}.
- Gap ledger: {gap, severity, evidence, remedy, owner-spec}.

## Error handling
Over-budget briefing -> failing test with measured size. Nondeterministic output -> failing
golden. Compat regression -> matrix cell flips to fail. Critical gap -> blocker / follow-up spec.

## Verification strategy
- R1: size assertions on context/next/dispatch `--json`; assert no ANSI in agent output.
- R2: `go test -bench`, latency budget on sample repo, double-run determinism golden.
- R3: consume mcp-transport matrix; assert each documented path yields a working tool call.
- R4: generate gap-analysis md; require each gap recorded as ADR/memory; critical -> follow-up spec.

## Risks and open questions
- "Token" budgets are approximations (byte/heuristic, not a model tokenizer) — document the
  proxy used. Latency budgets are machine-dependent; assert on relative regression vs a stored
  baseline, not absolute ms on arbitrary hardware. Open: which competitors define the
  market-gap baseline? Decision to be recorded as ADR before T-gap runs.
