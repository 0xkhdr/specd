# Spec 10 — CLI Architecture & Foundations

> **Authoring order:** 2 / 12 · **Critical path:** yes
> **Sources:** `fresh-start/10-cli-architecture-foundations.md`, paper pp.28–30
> **ADRs:** ADR-0, ADR-2, ADR-8
> **Reference:** `reference/main.go`, `reference/internal/cli/args.go`, `reference/internal/cmd/registry.go`, `reference/internal/core/{io,lock,paths,config_loader,config_validate,commands,help,exit,env,slug}.go`

The floor every other domain sits on: one flat dispatch registry, help that can never drift
from dispatch, atomic writes, a reentrant per-spec lock, and deterministic config cascade.

---

## 1. Purpose & principles
- **Principles served:** P1 (deterministic, auditable binary); substrate for all others.
- **Paper concept:** the harness as *tools + sandboxes + orchestration scaffolding* delivered
  as one calibrated artifact (pp.28–30). Zero-dep + determinism is what makes "most failures
  are configuration failures" auditable — nothing hidden in a dependency tree.

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| Zero-dep custom parser + flat registry | **KEEP** | Validated decision; 16-verb surface makes Cobra even less justified. `reference/internal/cli/args.go`, `registry.go` |
| Registry↔help single-source guard | **KEEP** (extend to drive MCP tools, Spec 07) | `TestRegistryMatchesHelp` |
| Atomic writes / reentrant lock / CAS primitives | **KEEP** | Hard invariants. `reference/internal/core/{io,lock}.go`; ADR-8 |
| Config format (YAML subset, fail-loud) | **KEEP YAML, SIMPLIFY** | Drop legacy `config.json` + `config_migrate.go`. ADR-2 |
| `commands.go` help metadata (~52.9K) | **SIMPLIFY by construction** | Fewer verbs; generate usage from registry |
| Program lock | **DEFER** with program tier | ADR-3; Spec 09 |
| Postgres/Redis backends | **CUT** to optional build tags | ADR-9 |

**Minimal accurate surface:** `main.go` + `internal/cli/args.go` + `internal/cmd/registry.go`
(16 entries) + `internal/core/{io,lock,paths,config_loader,config_validate,commands,help,exit,env,slug}.go`.

## 3. Requirements (EARS)
- **R10.1** The system shall build with zero runtime Go module dependencies and parse its own
  CLI arguments without a third-party library.
- **R10.2** The system shall fail its test suite if the dispatch registry and the help
  metadata disagree on the set of commands.
- **R10.3** When writing any file, the system shall write atomically via temp-file + fsync +
  rename so a partial write can never replace the target.
- **R10.4** When executing any mutating command, the system shall hold the reentrant per-spec
  advisory lock for the entire read-modify-write.
- **R10.5** When the `.specd/` root cannot be found by walking up, the system shall exit with
  the not-found code (3).
- **R10.6** When loading configuration, the system shall layer global→project→env
  deterministically, validate and secret-scrub the `orchestration` block, and fail loud on a
  truncated scalar.
- **R10.7** When a `SPECD_*` integer variable is malformed, the system shall clamp to range
  and warn once rather than silently defaulting.

## 4. Design

### Module boundaries
- `internal/cli` — parse. `internal/cmd` — dispatch + verbs. `internal/core` —
  io/lock/paths/config/exit/env/slug primitives.

### Key types
- `Command{Name, Summary, Handler, Positional, Flags}` — the single source feeding **three
  consumers**: dispatch, help, and the MCP tool list (Spec 07). Kills two drift classes.
- `Config` + `Diagnostic`; `SpecdError` / exit codes; lock handles.
- `LoadConfig(paths, env) → (Config, []Diagnostic)` is a **pure function**; only `paths.go`
  touches the filesystem — testable without disk.

### On-disk contracts
- `.specd/config.yml` (YAML two-space-indent subset only).
- `.lock` files: mode 0644, `pid+unix-ms`, stale reclaim `SPECD_LOCK_STALE_MS` (30s), acquire
  timeout `SPECD_LOCK_TIMEOUT_MS` (5s).
- `state.json` schema resets to `SchemaVersion: 1` for the fresh tree (ADR-2).

### External interfaces
- Exit codes `ExitOK/ExitGate/ExitUsage/ExitNotFound` (0/1/2/3); `SPECD_*` env vars; the
  `Command` table (drives Spec 07 tool list).

## 5. Invariants preserved (ADR-8)
Zero-dep; help↔dispatch parity; atomic write; reentrant lock + stale reclaim; CAS (Spec 02);
fail-loud config; env clamping.

## 6. Cross-domain dependencies
Every domain sits on these primitives. Spec 02 (CAS uses io+lock), Spec 07 (registry drives
tools), Spec 09 (config authority block + file backend). Depends on Spec 01 (charter).

## 7. Risks & open questions
- **Risk:** hand-maintained help drifts. → single `Command` table drives dispatch+help+MCP;
  parity test is CI-blocking.
- **Open:** fully generate help from the registry vs keep rich hand-written help. **Proposed:**
  registry-generated usage + short summary; long-form help only where a verb needs examples.
