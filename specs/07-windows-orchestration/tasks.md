# Spec 07 — Tasks (Windows Orchestration)

> Prereq: **Spec 01 done** (`Runner` seam). **Record the chosen option (A or B)
> here before implementing** — see `spec.md §3`. Default to **B** if native
> group-kill cannot be done with `syscall` alone (stdlib-only invariant wins).

> **DECISION:** ______ (A native runner / B fail-fast) — fill in.

---

## Wave A — Feasibility gate

### [ ] W3.5 — Decide A vs B
- **Files:** working note → this file's DECISION line
- **Do:** Verify whether Windows whole-tree kill is achievable with **`syscall`
  stdlib only** (no `golang.org/x/sys`). If yes and contained → Option A. If it
  needs `x/sys` or Windows drive-path CI is unavailable → Option B.
- **Done when:** Decision recorded with a one-line justification.

---

## Wave B — Implement (branch on decision)

### [ ] W3.6-A — (Option A) Native `WindowsRunner`
- **Files:** `internal/worker/shell_runner_unix.go` (retag existing
  `//go:build !windows`), `internal/worker/runner_windows.go` (new,
  `//go:build windows`)
- **Do:** Implement `WindowsRunner` satisfying `Runner`: `cmd /c <Command>`,
  group/tree kill on deadline (Job Object via `syscall` only — abort to Option B
  if impossible), reuse the shared env propagation + line writer + deadline ctx.
  Build-tag selection so each OS compiles only its runner.
- **Done when:** Builds on `GOOS=windows`; POSIX build unchanged; shared
  mechanics reused.

### [ ] W3.6-B — (Option B) Fail-fast on Windows
- **Files:** `internal/cmd/brain.go` (orchestration entrypoints) or a
  build-tagged guard `internal/worker/runner_windows.go`
- **Do:** On Windows, the orchestration drive entrypoints return a clear error
  + correct non-zero exit code (match the existing contract — `2` usage / `3`
  environment): `orchestration requires a POSIX shell (sh); not supported on
  Windows — run under WSL`. Non-orchestration spec workflow stays fully working.
- **Done when:** Windows build fails fast on drive with the message + right exit
  code; single-process paths unaffected.

---

## Wave C — Test & document

### [ ] W3.6c — Test the chosen path
- **Files:** `internal/worker/runner_windows_test.go` (A) or
  `internal/cmd/brain_windows_test.go` (B), build-tagged
- **Do:**
  - (A) On a Windows CI runner: assert deadline-kill terminates the whole tree
    and env vars propagate.
  - (B) Assert the fail-fast message + exit code on `GOOS=windows`
    (`//go:build windows` test, or a `GOOS`-guarded table test).
- **Done when:** Test green on the relevant OS in CI.

### [ ] W3.6d — Document
- **Files:** `README.md`, `AGENTS.md` (orchestration section)
- **Do:** State the Windows orchestration support level (A: supported; B:
  POSIX-only, use WSL). One clear sentence in each.
- **Done when:** Docs reflect actual behavior; update `specs/progress.md` W3 +
  exit gate.

---

## Definition of done (Spec 07)
- [ ] Windows orchestration **works (A)** or **fails fast with a clear message
      (B)** — no silent `sh` reliance.
- [ ] OS choice confined to build-tagged runner files (no scattered `GOOS`).
- [ ] Cross-OS matrix green; stdlib-only invariant intact.
- [ ] Behavior documented.
