# Memory — Regression: Agent-Harness Value (context engineering, perf, market-gap)

<!--
Source-attributed, generalizable learnings (append-only). Use
`specd memory <spec> add --key <slug> --pattern "<one-line>" --body "<detail>"
  --source "<Turn N, Task T?, role>" --criticality <minor|important|critical> [--related k,k]`.
Only generalizable patterns, never raw observations. Promote to project steering at 3+ specs via
`specd memory <spec> promote --key <slug>`. Format:

## <key-slug>
**Pattern:** <one-line generalizable claim>
**Detail:** <why it's true; the mechanism>
**Source:** Task T3, Turn 2, discovered by investigator
**Criticality:** important
**Related:** [[other-key]]
-->

## agent-output-byte-budgets
**Pattern:** Gate agent-facing --json briefings on a byte budget; use bytes/4 as the token proxy.
**Detail:** specd embeds no tokenizer, so the deterministic, CI-stable way to bound context cost is bytes (token proxy = bytes/4, the BPE rule of thumb). Set budgets as upper bounds with headroom over the measured baseline so normal spec growth doesn't flake, while decoration leaks / duplicated payloads still trip. Assert no ANSI in --json. See docs/agent-harness-baselines.md, ADR-001/GAP-1.
**Source:** Turn 1, Task T2/T5, reviewer
**Criticality:** important
**Related:** —
