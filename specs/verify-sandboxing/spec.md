# spec.md — Verify Sandboxing (container / namespace isolation)

**Status:** proposed
**Source:** specd-report.html §8 idea **B3** (impact: high · effort: high · moat: high)
**Date:** 2026-06-16
**Scope:** opt-in sandboxed runner backend for `internal/cmd/verify.go`; SECURITY.md update.

---

## 1. Objective

Today `verify` runs `sh -c` as the user (env scrubbed, NUL rejected, command/cwd
printed — see SECURITY.md), but it is still real code execution on
agent-authored input. Offer an **opt-in** sandboxed runner (container, `bwrap`,
or seccomp) so an untrusted `tasks.md` can be verified safely. "Only run verify
on a tasks.md you trust" is a real adoption ceiling for autonomous fleets;
safe-by-default verify unlocks running specs you didn't author.

> **Hard invariant:** stdlib-only in the binary, deterministic, evidence-gated.
> The sandbox is an **opt-in backend selected by config/flag**, shelling out to
> an external isolator (`bwrap`/`docker`/`podman`) that the operator installs —
> specd does not embed a runtime. Default behavior is unchanged. When the
> selected isolator is absent, specd fails closed with a clear error, never
> silently falling back to unsandboxed exec.

## 2. Context

- `internal/cmd/verify.go` runs the task command via `sh -c` with an env
  allowlist + NUL rejection (`internal/core/verify.go`-adjacent logic; see also
  `env.go`). SECURITY.md documents the current threat model explicitly.
- The verify record + evidence gate are unchanged regardless of runner.

## 3. Requirements (EARS)

- **R1 (H)** WHERE `verify.sandbox` config (or `--sandbox <mode>`) selects a
  sandbox backend (`none` default, `bwrap`, `container`), THE SYSTEM SHALL run
  the task's verify command inside that isolation boundary.
- **R2 (H)** WHEN the selected sandbox backend's tool is not installed, THE
  SYSTEM SHALL fail closed with a clear error and SHALL NOT fall back to
  unsandboxed execution.
- **R3 (H)** THE SYSTEM SHALL keep the existing env allowlist, NUL rejection,
  and command/cwd printing inside the sandbox (defense in depth, not replaced).
- **R4 (M)** WHERE `verify.sandbox == none` (default), behavior SHALL be
  byte-identical to today.
- **R5 (M)** THE SYSTEM SHALL record the sandbox mode used into the
  `VerificationRecord` so evidence states *how* the command was run.
- **R6 (M)** THE SYSTEM SHALL pass through exit code + output tail from the
  sandboxed process unchanged, preserving the evidence gate (exit 0 ⇒ pass).
- **R7 (L)** SECURITY.md SHALL document each backend's isolation guarantees and
  the fail-closed contract.

## 4. Design / approach

1. **Runner interface** — extract a `Runner` interface in `internal/core` with
   `Run(cmd, cwd, env) (exit, output)`. Default `shRunner` = today's path.
2. **Sandbox runners** — `bwrapRunner` (`bubblewrap` with ro-bind + tmpfs +
   no-net by default) and `containerRunner` (`docker`/`podman run --rm` with the
   repo bind-mounted). Each detects its tool; absent ⇒ fail closed (R2).
3. **Selection** — config `verify.sandbox` + `--sandbox` flag pick the runner;
   `none` keeps current behavior exactly.
4. **Evidence** — add `Sandbox string` to `VerificationRecord` (`omitempty`).

## 5. Non-goals

- No embedded container runtime; the operator installs `bwrap`/`docker`/`podman`.
- No change to the evidence gate or exit-code contract.
- Default stays `none` — this spec does not flip safe-by-default on (that is a
  later config decision once backends are proven).

## 6. Acceptance criteria

- `--sandbox bwrap`/`container` runs verify inside isolation; exit code +
  output tail flow through to the evidence gate unchanged.
- Missing isolator ⇒ clear fail-closed error, never unsandboxed fallback.
- `verify.sandbox: none` ⇒ byte-identical to today (regression test).
- Verify record states the sandbox mode used; SECURITY.md updated.
- `make ci` green; binary stays stdlib-only.
