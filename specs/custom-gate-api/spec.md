# spec.md — Plugin / Custom-Gate API

**Status:** proposed
**Source:** specd-report.html §8 idea **D2** (impact: high · effort: med · moat: high)
**Date:** 2026-06-16
**Scope:** declared external-gate hook contract layered onto `CheckGates`; `internal/core/gates.go`, `specd check`.

---

## 1. Objective

Let orgs register custom gates (license headers, ADR-required-for-arch-change,
perf budgets) via a declared hook contract with the same exit-code semantics.
Ship the 7 core gates; let the ecosystem add the 8th. Extensibility is how a tool
becomes a platform — a gate API turns specd's discipline into a substrate others
build standards on.

> **Hard invariant:** the 7 core gates and their semantics are unchanged and
> always run. Custom gates are **external executables** declared in config,
> invoked with a deterministic contract (env + JSON on stdin, exit code +
> structured violations out) — specd does not load Go plugins (`plugin` pkg is
> not portable) and adds no runtime dependency. Custom-gate failures integrate
> into the existing violation/exit-code model exactly like a core gate.

## 2. Context

- Core gates are `CheckGate` funcs in `internal/core/gates.go`, run as the
  ordered `CheckGates` pipeline by `specd check`; each returns
  `[]Violation`/warnings. Exit codes are deterministic (`exit.go`).
- `GatesCfg` (`specfiles.go`) already holds gate config (`traceability`,
  `acceptance`) — the natural home for a `custom` list.

## 3. Requirements (EARS)

- **R1 (H)** WHERE `gates.custom` config lists external gate commands, `specd
  check` SHALL invoke each after the core gates, passing the spec context (root,
  slug, artifact paths) as JSON on stdin and well-defined env vars.
- **R2 (H)** A custom gate SHALL report results via exit code (0 pass / non-zero
  fail) and a JSON array of violations on stdout; THE SYSTEM SHALL merge these
  into the existing violation list and overall exit code.
- **R3 (H)** THE 7 core gates SHALL run unchanged and SHALL always execute
  regardless of custom-gate configuration.
- **R4 (M)** WHERE a custom gate is configured `warn`, its failures SHALL be
  warnings; configured `error`, they SHALL fail the check (mirroring the
  traceability gate's model).
- **R5 (M)** IF a custom gate executable is missing, times out, or emits invalid
  JSON, THE SYSTEM SHALL surface a clear gate error and SHALL NOT crash or hang
  indefinitely (bounded timeout).
- **R6 (M)** Custom gates SHALL run with the same env-scrubbing discipline as
  `verify` (allowlist, NUL rejection) since they execute operator-supplied
  commands.
- **R7 (L)** Documentation SHALL specify the stdin schema, env vars, and stdout
  violation schema as a stable contract.

## 4. Design / approach

1. **Contract** — define `CustomGateInput` (root, slug, phase, artifact paths)
   and `CustomGateOutput` (`[]Violation`) JSON schemas in `core`.
2. **Runner** — `internal/core/customgate.go`: for each configured gate, exec
   with bounded timeout, JSON in / JSON out, reusing the verify env-scrub helper.
3. **Pipeline integration** — append a synthetic `CheckGate` that runs the
   configured custom gates and yields merged violations; warn/error per config.
4. **Config** — `gates.custom: [{name, command, level}]` in `GatesCfg`.

## 5. Non-goals

- No Go `plugin`/dlopen loading (non-portable, version-fragile).
- No change to the 7 core gates.
- No network; custom gates are local executables.

## 6. Acceptance criteria

- A configured custom gate runs after the core gates; its JSON violations merge
  into `specd check` output and drive the exit code per its `warn`/`error` level.
- The 7 core gates run unchanged with or without custom gates (regression test).
- Missing/timeout/invalid-JSON custom gate ⇒ clear error, bounded, no hang.
- Custom gates run under the verify env-scrub discipline; contract documented;
  `make ci` green; stdlib-only.
