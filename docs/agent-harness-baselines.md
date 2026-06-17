# Agent-Harness Baselines — Budgets & Performance

> Source spec: `regression-agent-harness-value` (program capstone, wave 5).
> These thresholds gate the context-efficiency (R1) and performance (R2) regression
> tests. Measurements taken on this repo's own `.specd/` as the sample repo.

## Token proxy

specd does not embed a model tokenizer. The proxy for "tokens" is **bytes ÷ 4**, the
widely-used rule of thumb for English/JSON under BPE tokenizers. Budgets below are
expressed in **bytes** (the directly measurable, deterministic quantity); the token
column is the bytes÷4 estimate for human reference only. See ADR-001.

## R1 — Context-engineering budgets

Agent-facing briefings (`--json`) must stay within these byte budgets and must be
free of ANSI/human decoration. Measured baseline is the observed size on the sample
repo; budget includes headroom for spec growth.

| Briefing (`--json`)        | Measured | Budget (bytes) | ~Tokens | ANSI? |
|----------------------------|---------:|---------------:|--------:|-------|
| `context <slug>`           |      898 |          2,048 |     512 | none  |
| `next <slug>`              |      661 |          1,536 |     384 | none  |
| `dispatch <slug>`          |    1,829 |          4,096 |   1,024 | none  |

Rule: if a briefing exceeds its budget the regression test fails and prints the
measured size (R1.3). Agent output (`--json` / MCP) must contain no ANSI escape
sequences (R1.2).

## R2 — Performance baselines

Latency is full-process invocation (spawn + execute) of the agent-facing `--json`
path, best-of-9 on the sample repo. Latency is machine-dependent: the regression
asserts **relative regression vs this stored baseline**, not absolute ms on arbitrary
hardware (see design risks).

| Command (`--json`)   | Baseline (ms) | Budget (ms) |
|----------------------|--------------:|------------:|
| `status`             |           7.5 |          50 |
| `next <slug>`        |           7.1 |          50 |
| `program status`     |           8.7 |          50 |

Hot-path microbenchmarks (`go test -bench`) cover dag/frontier and render and are
tracked so they are not regressed silently (R2.3).

## Determinism

Running the same `--json` command twice on unchanged state must produce byte-identical
output (R2.2), asserted via a double-run golden.

## Provenance

Measured with `go build` of the repo at wave-5 HEAD against `.specd/` as the sample
repo. Re-measure and update this table when the briefing schema changes materially.
