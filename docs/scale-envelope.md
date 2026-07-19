# specd — Scale Envelope

Driver: `TestFrontierScalesSubQuadratically` (`internal/core/bench_test.go:70`) — fails if quadrupling task count costs more than 9x on the frontier hot path.

The intended operating limits for a `specd`-managed workspace, and the measured numbers that
back them. `specd` is a local, single-binary harness over a local filesystem; the envelope is
generous relative to how large a hand-authored spec ever gets, but it is stated so nobody has to
guess where the cliffs are.

> Numbers below were measured with `go test -bench=. -benchmem ./internal/core/ ./internal/context/`
> on a 12-core x86-64 dev machine, Go 1.26. Re-run to refresh; treat the shape (how cost grows
> with size), not the absolute nanoseconds, as the contract.

## Intended limits

| Dimension | Recommended | Practical ceiling | What binds it |
|---|---|---|---|
| Tasks per spec | ≤ 500 | ~2000 | Wave projection is O(n²) for a dependency chain (see below) |
| Specs per program | ≤ 100 | — | Program operations iterate specs linearly; no cross-spec quadratic |
| Steering / memory files | tens | — | Manifest assembly reads the steering dir once, stats each file once |

These are guidance, not enforced caps — nothing in the harness rejects a larger spec. They mark
where hand-authored specs stop being sensible and where the one quadratic path starts to show.

## Measured costs

### Runnable frontier — linear (the orchestration hot path)

`core.Frontier` / `RunnableFrontier` recomputes the set of tasks whose deps are resolved. It is a
single O(n · deps) scan and scales linearly; a 4× input costs ~5× time (the extra above 4× is map
growth, not super-linear work). This is the function the Brain calls every wave, so it is the one
that must stay cheap — and does.

| Tasks | ns/op | allocs/op |
|---|---|---|
| 100 | ~30,000 | 25 |
| 500 | ~194,000 | 37 |
| 2000 | ~1,100,000 | 71 |

`TestFrontierScalesSubQuadratically` pins this: quadrupling the task count must cost < 9× time
(measured ~4.8×), which rejects quadratic growth while tolerating CI-runner noise.

### Wave projection — quadratic, bounded (the reporting path)

`core.ProjectWaves` / `TopologicalWaves` groups the whole DAG into ordered waves. For a pure
dependency chain it re-scans the remaining tasks each wave, so it is **O(n²)**. This is fine for
the reporting/visualization use it serves and stays sub-100 ms well past any realistic spec, but
it is why the per-spec recommendation is ≤ 500 tasks.

| Tasks | ns/op |
|---|---|
| 100 | ~214,000 |
| 500 | ~3,900,000 |
| 2000 | ~62,000,000 |

Upgrade path if a spec ever genuinely needs thousands of chained tasks: replace the re-scan with
Kahn's algorithm (indegree queue) for O(n + edges). Not worth the code today — no real spec is
that large.

### Context manifest — flat, no N+1

`context.BuildManifest` references a fixed set per task (spec, tasks, task, role, plus steering);
its cost is independent of how many tasks the spec has. `TestBuildManifestNoN1FileReads` pins
that the item count does not grow with task count — no per-task file amplification.

- BuildManifest (2000-task spec): ~2,560 ns/op, 18 allocs/op — flat in task count.
- Disabled-mode budget check (A4): ~1.9 ns/op, **0 allocs/op** — `TestCheckBudgetDisabledZeroAllocs`
  pins the O(0) claim.

## Determinism & cleanup

Independent of scale, resource cleanup is deterministic: a failed verify under `--revert-on-fail`
restores the working tree and releases the per-spec lock, leaving no temp files
(`TestRevertOnFail`, `TestVerifyFailureLeavesCleanTree`). See [SECURITY.md](../SECURITY.md) for
the verify isolation contract.
