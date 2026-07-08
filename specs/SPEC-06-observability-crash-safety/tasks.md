# SPEC-06 Tasks: Observability & Crash-Safety

| Task ID | Title | Description | Acceptance Criteria | Estimated Effort | Status |
|---------|-------|-------------|---------------------|------------------|--------|
| T-06-01 | Prometheus output validity | Test that `report --format prometheus` emits valid textfile-collector metrics. | A parser/format test passes; fails on malformed metric output. | Small | pending |
| T-06-02 | History ordering + schema | Assert `report --history` replays in timestamp order; assert `--json`/`--metrics` schema stability. | Tests pass; fail on out-of-order history or schema drift. | Medium | pending |
| T-06-03 | HUD + error discipline | Verify `context --hud` renders without error; assert exit 1 vs 2 discipline and actionable CAS/lock errors matching troubleshooting.md. | HUD renders; error-code tests pass; troubleshooting.md matches real messages. | Medium | pending |
| T-06-04 | Crash-safety assertions | Define + provide fault-injection assertions for ACP ledger append/replay (stress-acp.sh / stress-checkpoint-fault.sh); wire with SPEC-01 or as targeted tests. | An interrupted-write fault run replays to a consistent state; green in CI. | Large | completed |
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

## Status Notes

- **T-06-04 completed — BD-01 resolved (fast-tracked out of wave order to unblock SPEC-01).**
  The double-dispatch race in `brain resume` had two root causes, both fixed:
  1. **Non-atomic resume critical section** (`internal/cmd/brain_run.go`): `brainResume` did its
     ledger read, `PlanResume`, session CAS, and `AppendDispatch` under *separate* lock scopes, so
     two resumes could interleave — one winning its CAS while reading a stale-empty ledger inside
     the other's not-yet-appended-dispatch window. Fix: the whole critical section now runs inside
     one `core.WithSpecLock` (reentrant, so the nested lock in `SaveSessionCAS` does not deadlock),
     making load→read-ledger→plan→CAS→append atomic w.r.t. other resumes.
  2. **False-stale lock removal** (`internal/core/lock.go`): `acquireFileLock` creates the lock
     file `O_EXCL` and then writes pid+timestamp in a *second* syscall. An observer in that window
     read an empty file, `lockIsStale` deemed a `<2 fields` file stale, and removed a **live**
     lock — letting two processes hold it at once. Fix: an unparseable lock body falls back to
     mtime (`lockMtimeStale`); only a file untouched past the stale window is a genuine orphan.
  Evidence: `internal/cmd.TestBrainResumeRaceDispatchesExactlyOnce` (races N resumes, asserts one
  dispatch; deterministic under `-race`/`-count=2`); the five brain-resume stress scripts pass
  30/30 (previously ~7% flake). No LLM in the path, no evidence-bypass, guardrails preserved.
