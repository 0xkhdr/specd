# Agent-Harness Gap Analysis & Improvement Ledger

> Source spec: `regression-agent-harness-value` (R4), the program capstone. This is
> the evidence-backed gap analysis comparing specd to the spec-driven / agent-tooling
> field, plus a ranked improvement ledger. Every gap is also recorded as an ADR in
> `.specd/specs/regression-agent-harness-value/decisions.md` (and memory where
> generalizable) so no market-value improvement is left implicit.

## Where specd sits in the field

specd is a **deterministic, file-backed spec substrate for agents**: EARS
requirements → design → tasks → execution, gated (`specd check`) and verified
(`specd verify`), exposed identically over CLI `--json` and MCP (stdio + HTTP/SSE).
Versus the field:

- **vs spec-kit / prose-spec tools** — specd adds machine-verifiable gates and a
  task DAG with frontier/wave scheduling, not just documents.
- **vs IDE-bound agent tools (Kiro, Cursor flows)** — specd is host-agnostic: one
  dispatch behind CLI and MCP, five host config snippets, transport parity.
- **vs ad-hoc agent loops** — specd is deterministic and byte-budgeted: briefings
  fit a documented context budget (R1) and read paths are determinism-golden (R2).

Strengths confirmed this wave: tight agent output (context 898 B / next 661 B /
dispatch 1829 B, no ANSI), fast read paths (status/next/program 7–9 ms), and an
honest, test-enforced compatibility matrix.

## Gaps (evidence + remedy + owner)

| ID | Gap | Evidence | Severity | Remedy | Owner |
|----|-----|----------|----------|--------|-------|
| GAP-2 | No `--config` snippet for HTTP/SSE; browser/remote hosts wire loopback by hand | docs/agent-harness-compat.md; hosts.go ships stdio snippets only | medium-high (compat) | `specd mcp --config http` emitting an endpoint snippet | follow-up transport spec (ADR-003) |
| GAP-1 | Token budgets use bytes/4 proxy, not a real tokenizer | docs/agent-harness-baselines.md; ADR-001 | medium | Optional tokenizer-backed budget behind a flag; keep bytes default | this ledger (ADR-002) |
| GAP-3 | Benchmarks/baselines exist but no CI gate diffs against stored baseline | internal/core/agent_perf_test.go; no benchstat step in CI | medium | Wire benchstat / threshold check over the bench set vs committed baseline | this ledger (ADR-004) |
| GAP-4 | `.specd/steering/product.md` still TODO; market framing not durable | steering/product.md placeholders | low | Fill product/users/value/out-of-scope | this ledger (ADR-005) |

## Criticality judgment (R4.3)

No gap is **reliability- or compatibility-critical** in the blocker sense: GAP-2,
the only compatibility gap, has a documented working workaround (`specd mcp --http`
+ manual endpoint wiring), so it is logged as a follow-up, not a program blocker. No
blocker is linked this wave.

## Improvement ledger — ranked by market-value impact

1. **GAP-2 — HTTP/SSE host snippet** (highest). Removes the one manual step for the
   fastest-growing host class (browser/remote agents); pure additive reach.
2. **GAP-3 — CI perf-regression gate.** Turns the wave-5 baselines into a durable
   guarantee; protects the "fast substrate" claim from silent drift.
3. **GAP-1 — exact token budgets.** Sharpens the headline context-efficiency claim
   from approximate to per-model exact for buyers who care about token cost.
4. **GAP-4 — fill product steering.** Lowest direct value; makes future gap analyses
   grounded rather than re-derived.

Re-run this analysis when a new host class, transport, or competitor materially
changes the landscape.
