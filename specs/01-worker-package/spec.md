# Spec 01 — Extract & Test `internal/worker` Package

> Wave: **W1 (P0)** · Priority: **P0 — highest** · Source: LEVEL_UP_PLAN §1.3, §2 P0.1/P0.2

## 1. Problem

`internal/cmd/brain.go` is **692 LOC at 0% test coverage** and is the
single biggest risk in the codebase. It is the autonomous orchestration driver
that actually runs in the field. It mixes three unrelated responsibilities in
one file:

1. CLI arg parsing / subcommand routing (`RunBrain`, `brainPolicy`, flag helpers)
2. Orchestration policy assembly (`brainRunPolicy`, `brainPolicyAndConfig`)
3. **OS process management** — spawning `sh -c <worker-cmd>`, process-group
   ownership, deadline contexts, SIGKILL escalation, pipe draining, and
   line-prefixed output muxing.

Responsibility (3) is the dangerous part. It is the code that produces **zombie
processes, hung pipes, and deadline races in production**, and it has zero
tests. The deterministic *engine* beneath it (`orchestration_engine.go`,
`program_orchestration.go`) is well-tested, but the process layer that wraps it
is invisible to the suite.

The concrete uncovered process-layer functions today (`internal/cmd/brain.go`):

| Func | Line | Role |
|---|---|---|
| `brainRunWorker` | 392 | builds the dispatch closure: temp mission file, deadline ctx, `sh -c`, pgid, SIGKILL cancel, `WaitDelay`, env, line writers |
| `workerDeadlineContext` | 454 | RFC3339Nano deadline → bounded context; clock-skew tolerant |
| `workerArtifactHint` | 465 | base-names mission files → `SPECD_ARTIFACT` |
| `newWorkerLineWriter` / `Write` / `Flush` | 485/489/507 | per-line prefix muxing under a shared output lock |
| `brainRunProgramWorker` | 315 | program-level analogue of `brainRunWorker` |

These are pure mechanism with clean inputs/outputs but are trapped inside a CLI
file with `cli.Args`-shaped seams, so they cannot be unit-tested in isolation.

## 2. Root cause

No seam. The process-execution mechanism is expressed as closures returned from
`brain*` functions and wired directly to `os.Stdout`/`os.Stderr`/`exec.Command`.
There is no interface to substitute a fake runner, no injectable output sink,
and no way to drive a single dispatch without standing up the whole CLI.

## 3. Solution

Introduce a small, focused `internal/worker` package that owns process
execution behind a `Runner` interface. `cmd/brain.go` shrinks to arg-parsing +
policy and delegates all exec to `worker`.

### 3.1 Package shape

```go
// internal/worker/worker.go
package worker

// Mission is the execution unit handed to a Runner. It is the existing
// core.DriverDispatch.Mission flattened to what the runner actually needs;
// keep field names identical to today's env keys to avoid churn.
type Mission struct {
    Command   string   // the worker command (was workerCmd)
    MissionID string
    SessionID string
    WorkerID  string
    Spec      string
    TaskID    string
    Role      string
    Files     []string // → SPECD_ARTIFACT hint
    Deadline  string   // RFC3339Nano
}

type Result struct {
    ExitErr   error  // non-nil iff the command failed / timed out
    TimedOut  bool
    Duration  time.Duration
}

// Runner executes one Mission to completion (or deadline). Implementations
// must own the child process group and guarantee no orphan outlives Run.
type Runner interface {
    Run(ctx context.Context, m Mission) (Result, error)
}
```

### 3.2 Concrete runner

`ShellRunner` (POSIX) is the lift-and-shift of today's behavior, made testable:

- Stdout/stderr sinks are **injectable** (`io.Writer`), defaulting to
  `os.Stdout`/`os.Stderr`. Tests pass buffers.
- The shared output mutex moves into the package; the line writer becomes
  `worker.lineWriter` (unexported) with the *exact* current prefixing/flush
  semantics — including partial / no-trailing-newline handling.
- Deadline context logic moves verbatim (`workerDeadlineContext`), keeping the
  clock-skew tolerance (unparseable/past deadline → short non-zero budget).
- Process-group ownership (`Setpgid`), `Cancel` = `syscall.Kill(-pid, SIGKILL)`,
  and `WaitDelay = 5s` move verbatim.
- Env propagation moves verbatim — **keep all seven keys byte-identical**:
  `SPECD_MISSION/SESSION/WORKER/SPEC/TASK/ROLE/ARTIFACT`.

### 3.3 CLI wiring

`cmd/brain.go`'s `brainRunWorker`/`brainRunProgramWorker` become thin adapters
that build a `worker.Mission` from `core.DriverDispatch` and call
`runner.Run(ctx, m)`. The `sh -c` string, temp mission file creation, and env
keys are the runner's job. CLI keeps: routing, policy, flag parsing.

> **Stdlib-only invariant preserved** — `os/exec`, `syscall`, `context`, `io`
> only. No new dependency.

## 4. Acceptance criteria

- [ ] `internal/worker` package exists with `Runner` interface + `ShellRunner`.
- [ ] `cmd/brain.go` drops below **~350 LOC** (from 692).
- [ ] `internal/worker` coverage **≥ 90%**.
- [ ] `cmd/brain.go` coverage **0% → ≥ 80%**.
- [ ] Env propagation keys are byte-identical to today (regression-asserted).
- [ ] All 1154 existing tests still pass; no behavior change in the field path.
- [ ] Stdlib-only invariant intact.

## 5. Non-goals

- Windows runner — that is **Spec 07** (`07-windows-orchestration`), built on
  this `Runner` seam.
- Changing dispatch semantics, deadline math, or output format. This is a
  **mechanical extraction with characterization tests**, not a redesign.

## 6. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Subtle behavior drift during move | Write characterization tests *first* against current behavior where feasible; diff env/output byte-for-byte |
| Output interleave regression | Keep the single package-level mutex; test concurrent writers |
| Hidden coupling to `cli.Args` | Flatten to `Mission` struct at the CLI boundary only |
