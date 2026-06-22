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
| `context <slug>`           |    2,942 |          3,584 |     735 | none  |
| `next <slug>`              |      661 |          1,536 |     384 | none  |
| `dispatch <slug>`          |    3,110 |          4,096 |     777 | none  |

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

## R4/R5 — Onboarding performance & deterministic output

> Source spec: `specd init` onboarding (wave 8, task T26). Gates the smooth-onboarding
> latency target (success metric: MCP health probe < 500 ms p95 locally) and the
> deterministic init-receipt contract (R1.3, R5.2).

Latency is recorded with `make bench` (`go test -bench 'Init|Probe|Detection'`).
Baselines below were measured on this repo with a scaffold-only, offline runtime
(empty host registry, in-process probe). Latency is machine-dependent; **CI does not
gate on wall-clock** — see policy below.

| Operation (`-bench`)      | Baseline (ms) | p95 budget (ms) |
|---------------------------|--------------:|----------------:|
| `BenchmarkInitFresh`      |          17.3 |             500 |
| `BenchmarkInitRerun`      |           1.5 |             100 |
| `BenchmarkAgentDetection` |          0.07 |              50 |
| `BenchmarkProbe`          |          0.81 |             500 |

### Deterministic-output gate (CI)

`make perf-gate` runs the byte-stability checks under `-count=2`:

- `TestInitOutputDeterministic` — a healthy `init --json` rerun emits a
  byte-identical receipt and parses as valid JSON with non-null arrays (R1.3, R5.2).
- `TestInitBenchmarkContract` — host detection is stable across calls and a fresh
  init reaches `status: "ready"`, so latency baselines compare like for like.
- `TestProbeDeterministic` — probe protocol version and tool count are stable across
  runs (latency excluded).

### Performance regression policy

- The deterministic byte checks are the only CI-blocking onboarding gate. They are
  hardware-independent and stable.
- Latency baselines are **reviewed, not enforced**: re-run `make bench`, compare
  against the table, and update it in the same PR when init/probe paths change
  materially. A wall-clock CI gate would be flaky across runners, so it is omitted by
  design (the spec's < 500 ms target is a product success metric, not a unit gate).

## Determinism

Running the same `--json` command twice on unchanged state must produce byte-identical
output (R2.2), asserted via a double-run golden.

## Provenance

Measured with `go build` of the repo at wave-5 HEAD against `.specd/` as the sample
repo. Re-measure and update this table when the briefing schema changes materially.
