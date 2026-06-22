# Spec 01 — Tasks (`internal/worker`)

> Implement top-to-bottom. Each task lists **files**, **what to do**, and
> **done-when**. Do not advance a wave until the prior wave's tasks are checked
> and the suite is green.

---

## Wave A — Create the seam (no behavior change)

### [x] W1.1 — Create `internal/worker` package skeleton
- **Files:** `internal/worker/worker.go` (new)
- **Do:** Define `Mission`, `Result`, and `Runner` interface exactly as in
  `spec.md §3.1`. Add package doc comment explaining the seam. No logic yet.
- **Done when:** `go build ./...` passes; package compiles with the interface
  and structs only.

### [x] W1.2 — Move the exec mechanism into `ShellRunner`
- **Files:** `internal/worker/shell_runner.go` (new), `internal/cmd/brain.go`
- **Do:**
  1. Create `ShellRunner` with injectable `Stdout, Stderr io.Writer` fields
     (nil → `os.Stdout`/`os.Stderr`).
  2. Move verbatim from `brain.go`: temp mission-file creation, deadline ctx
     (`workerDeadlineContext` → `deadlineContext`), `exec.CommandContext("sh",
     "-c", m.Command)`, `Setpgid`, `Cancel` SIGKILL-of-group, `WaitDelay = 5s`,
     the seven `SPECD_*` env keys, `Run` + flush.
  3. Move `workerArtifactHint` → `artifactHint` (unexported).
  4. Move `workerLineWriter` + `workerOutputMu` → unexported `lineWriter` /
     package mutex with **identical** prefix/flush semantics.
  5. Return `Result{ExitErr, TimedOut, Duration}`; map
     `ctx.Err()==DeadlineExceeded` → `TimedOut=true` + the existing timeout
     error string.
- **Done when:** `internal/worker` builds; `cmd/brain.go` no longer references
  `exec`, `syscall`, `io`, or the line-writer types directly.

### [ ] W1.3 — Rewire `cmd/brain.go` to delegate
- **Files:** `internal/cmd/brain.go`
- **Do:** `brainRunWorker` and `brainRunProgramWorker` become thin adapters:
  build `worker.Mission` from `core.DriverDispatch` and call
  `runner.Run(ctx, m)`. Construct one `worker.ShellRunner` per drive. Keep
  routing/policy/flag-parsing in place.
- **Done when:** `cmd/brain.go` < ~350 LOC; `go test ./...` green (1154 pass);
  manual `specd brain run` smoke still prefixes worker output identically.

---

## Wave B — Characterization & hardening tests (the point of the spec)

### [x] W1.4 — Deadline → SIGKILL-of-process-group test
- **Files:** `internal/worker/shell_runner_test.go` (new)
- **Do:** Mission whose command is `sh -c 'trap "" TERM; sleep 30 & wait'`
  (forks a child that ignores SIGTERM) with a deadline ~200ms out. Assert:
  `Run` returns within `WaitDelay` (5s) + margin; `Result.TimedOut == true`;
  the spawned child PID is gone (poll `syscall.Kill(pid,0)` → ESRCH). Skip on
  non-POSIX via build tag / `runtime.GOOS` guard.
- **Done when:** Test proves the **whole group** dies and `Run` does not hang.

### [x] W1.5 — Pipe-drain (no-hang) test
- **Files:** `internal/worker/shell_runner_test.go`
- **Do:** Command that, after the deadline fires, keeps a backgrounded writer
  emitting to stdout (orphan holding the pipe). Assert `Run` still returns
  within `WaitDelay` margin (proves `WaitDelay` force-closes the pipes and
  `Wait` cannot block on an orphan).
- **Done when:** Test green and would hang without `WaitDelay` (verify by
  temporarily removing it locally).

### [x] W1.6 — Line-writer prefixing test
- **Files:** `internal/worker/line_writer_test.go` (new)
- **Do:** Drive `lineWriter` directly with: (a) multiple writes forming one
  line; (b) a chunk with several `\n`; (c) a trailing chunk with **no** newline
  then `Flush`. Assert every emitted line is prefixed exactly once and no bytes
  are dropped. Add a concurrent-writers case (two writers, shared dst) asserting
  no mid-line interleave (lock works).
- **Done when:** All sub-cases pass; output byte-exact against expected.

### [x] W1.7 — Mission env propagation test
- **Files:** `internal/worker/shell_runner_test.go`
- **Do:** Mission command = a tiny script that echoes each `SPECD_*` var
  (capture via injected stdout buffer). Assert all seven keys
  (`MISSION/SESSION/WORKER/SPEC/TASK/ROLE/ARTIFACT`) are present and equal the
  Mission fields; `SPECD_ARTIFACT` = comma-joined base names (multi-file case);
  `SPECD_MISSION` points at a readable temp file containing the mission JSON.
- **Done when:** Byte-exact env assertions pass, including multi-file artifact.

---

## Wave C — Coverage close-out

### [ ] W1.8 — Cover residual `cmd/brain.go` paths
- **Files:** `internal/cmd/brain_test.go` (new/extend)
- **Do:** Table-test the CLI surface that is now thin: `RunBrain` routing,
  `brainPolicy`/`brainRunPolicy` defaults & flag overrides, flag parse helpers,
  `bootstrapHint`, `brainWhy` happy path. Use a fake `Runner` (implements the
  interface, records the `Mission`) injected into the drive to assert the CLI
  builds the correct mission without spawning real processes.
- **Done when:** `cmd/brain.go` coverage **≥ 80%** (`go test -cover`).

### [ ] W1.9 — Verify package coverage gate
- **Files:** n/a (CI / local)
- **Do:** Run `go test -cover ./internal/worker/...` and confirm **≥ 90%**.
  Fill any gap (error branches: temp-file failure, marshal failure).
- **Done when:** `internal/worker` ≥ 90%; update `specs/progress.md` W1
  checkboxes; suite green with `-race -count=2`.

---

## Definition of done (Spec 01)
- [ ] `internal/worker` exists, ≥90% covered.
- [ ] `cmd/brain.go` < ~350 LOC, ≥80% covered.
- [ ] Env keys byte-identical; output prefixing unchanged.
- [ ] `-race -count=2 ./...` green; stdlib-only intact.
