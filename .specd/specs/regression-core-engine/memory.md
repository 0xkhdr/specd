# Memory — Regression: Core Engine (DAG, gates, state, runner, telemetry)

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

## coverage-baseline-T1
**Pattern:** regression floor: core coverage must stay >= T1 baseline
**Detail:** Baseline core coverage (T1): total 66.4%. Per-file lows to watch: embed 0, output 0, pack_apply 0, prsummary 0, schema_validate 0, taskview 0, md 14.3, paths 20.0, ui 26.7, specfiles 29.1, runner_sandbox 33.2, render 40.1, phases 43.0. Highs: blockers/commitlink/env/exit/slug 100, agents 98.9, runner 98.3, state 97.6, frontier 96.7.
**Source:** T1
**Criticality:** important
**Related:** —
