# Spec 07 — Windows Orchestration Story

> Wave: **W3 (P2)** · Priority: **P2** · Source: LEVEL_UP_PLAN §1.5, §2 P2.9
> Depends on: **Spec 01** (`Runner` interface is the substitution seam)

## 1. Problem

Worker dispatch is hard-coded to `exec.Command("sh", "-c", workerCmd)` with
`syscall.SysProcAttr{Setpgid: true}` and `syscall.Kill(-pid, SIGKILL)`. This is
**POSIX-only**. The binary *builds* on Windows (cross-OS matrix is green), but
orchestration **silently relies on a bash-like `sh`** that may not exist, and
`Setpgid`/`Kill(-pid)` are not Windows concepts. So on Windows, orchestration is
effectively unsupported and **fails in confusing ways** rather than cleanly.

## 2. Solution — pick one, behind the `Runner` seam

Spec 01 gives a `Runner` interface. Two acceptable resolutions; **choose (A) if
feasible, else (B)**:

### Option A — Native Windows runner (preferred if scoped small)
- Implement a `WindowsRunner` (build-tagged `//go:build windows`) that uses
  `cmd /c <workerCmd>` (or direct exec) with a Windows **Job Object** for
  group-kill semantics (`CREATE_SUSPENDED` + assign to job + `TerminateJobObject`
  on deadline), mirroring the POSIX process-group kill.
- `ShellRunner` becomes build-tagged `//go:build !windows`. Selection is by
  build tag, so each OS compiles only its runner.
- Env propagation + line-writer + deadline ctx are OS-agnostic (already in the
  shared package from Spec 01) and reused unchanged.

### Option B — Explicit POSIX-only fail-fast (acceptable fallback)
- On Windows, the orchestration entrypoints (`brain run`/`step`/program drive)
  **detect Windows and fail fast** with a clear, actionable message:
  `orchestration requires a POSIX shell (sh); not supported on Windows — run
  under WSL or a POSIX environment` and a **non-zero exit code** consistent with
  the existing exit-code contract (`2` usage / `3` environment, per the
  established convention — pick the matching one).
- Document the limitation in `README.md` / `AGENTS.md` orchestration section.
- Single-process spec workflow (non-orchestrated) remains fully supported on
  Windows; only the multi-worker drive is gated.

## 3. Decision criteria

Prefer **A** if the Job Object implementation is contained (one build-tagged
file, ≤~200 LOC, testable on a Windows CI runner). Choose **B** if Job Object
work would balloon scope or Windows CI for the drive path is unavailable — a
clean fail-fast is strictly better than today's silent confusion.

> Record the decision (A or B) at the top of `tasks.md` before implementing.

## 4. Acceptance criteria

**Either:**
- [ ] (A) `WindowsRunner` implements `Runner`, builds under `//go:build
      windows`, kills the whole process tree on deadline (Job Object), and has a
      test on a Windows runner proving deadline-kill + env propagation.

**Or:**
- [ ] (B) Orchestration entrypoints fail fast on Windows with a clear message +
      correct non-zero exit code; documented; non-orchestration paths still work
      on Windows. A test (build-tagged or `GOOS`-guarded) asserts the fail-fast
      message + exit code.

**Both options:**
- [ ] Selection is via build tags / `Runner` injection — no `runtime.GOOS`
      branching scattered through the drive loop.
- [ ] POSIX behavior unchanged (Spec 01 `ShellRunner` intact).
- [ ] Cross-OS build matrix stays green; stdlib-only invariant intact (Windows
      Job Object via `golang.org/x/sys`? — **no**: use `syscall`/`os` stdlib
      only; if Job Object needs `x/sys`, that violates stdlib-only → prefer
      Option B).

## 5. Stdlib-only caveat (important)

Windows Job Object APIs often pull in `golang.org/x/sys/windows`. That **breaks
the stdlib-only runtime invariant**. If native group-kill cannot be done with
`syscall` alone, **choose Option B** — the invariant outranks native Windows
orchestration. Confirm `syscall` coverage before committing to Option A.

## 6. Non-goals

- Full Windows feature parity for orchestration if Option B is chosen.
- Reimplementing the shared worker mechanics (already OS-agnostic post-Spec-01).

## 7. Risks & mitigations

| Risk | Mitigation |
|---|---|
| Job Object needs `x/sys` → breaks stdlib-only | Default to Option B; A only if `syscall`-pure |
| Silent partial Windows support | Fail-fast is explicit and tested; no silent `sh` reliance |
| `runtime.GOOS` branches leak everywhere | Confine OS choice to build-tagged runner files |
