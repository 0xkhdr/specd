# SPEC-06 Tasks: Observability & Crash-Safety

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-06-01 | Prometheus output validity | Test that `report --format prometheus` emits valid textfile-collector metrics. | A parser/format test passes; fails on malformed metric output. | Small | pending |
| T-06-02 | History ordering + schema | Assert `report --history` replays in timestamp order; assert `--json`/`--metrics` schema stability. | Tests pass; fail on out-of-order history or schema drift. | Medium | pending |
| T-06-03 | HUD + error discipline | Verify `context --hud` renders without error; assert exit 1 vs 2 discipline and actionable CAS/lock errors matching troubleshooting.md. | HUD renders; error-code tests pass; troubleshooting.md matches real messages. | Medium | pending |
| T-06-04 | Crash-safety assertions | Define + provide fault-injection assertions for ACP ledger append/replay (stress-acp.sh / stress-checkpoint-fault.sh); wire with SPEC-01 or as targeted tests. | An interrupted-write fault run replays to a consistent state; green in CI. | Large | pending |
| T-06-05 | Observability docs | Document CLI logging levels/telemetry strategy and where worker `--tokens`/`--cost`/`--duration-ms` surface in reports. | Docs published and accurate; linked from docs index. | Small | pending |

## Task Dependency Graph

```
T-06-01 (parallel)
T-06-02 (parallel)
T-06-03 (parallel)
T-06-04 ─→ (coordinates with SPEC-01 T-01-04)
T-06-05 (parallel)
```
All output/doc tasks are independent; T-06-04's assertions coordinate with SPEC-01's stress-script
decision.
