# Spec 03 — Validation Gates Engine

> **Authoring order:** 6 / 12 · **Critical path:** yes
> **Sources:** `fresh-start/03-validation-gates-engine.md`, paper pp.29–30
> **ADRs:** ADR-4, ADR-5, ADR-8
> **Reference:** `reference/internal/core/{gates,ears,specfiles,dag,customgate}.go`, `reference/internal/cmd/{check,security}.go`, `reference/docs/validation-gates.md`

`specd check <slug>` runs a fixed battery of pure gates over the spec artifacts + state and
returns violations/warnings deterministically. This is the enforcement half of P1.

---

## 1. Purpose & principles
- **Principles owned:** P1 (harness enforces), P3 (Evidence Gates Every State Change).
- **Paper concept:** *guardrails* + the "Testing & QA / Feedback Loop" (pp.29–30): "The
  harness runs deterministic hooks."

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| 7 core gates as pure functions | **KEEP** | P3 made mechanical. `reference/internal/core/gates.go` |
| Gate IO-purity invariant (`CheckCtx → []Finding`) | **KEEP** | Determinism |
| Hardcoded opt-in branches (gates 8–13) | **REDESIGN** → pluggable `Gate` interface + registry | ADR-4; kills four-place drift |
| Custom external gates | **KEEP** | Clean external contract already |
| Security scanners | **KEEP-as-plugin-gate** (stdlib-only, off by default) | ADR-4/ADR-5; the model case for the interface |
| Eval/Review/Ingest gates | **DEFER commands, keep gate hooks** | ADR-5; interface proven against real modules |

The **7 core gates:** EARS shape (1), Design headers (2), Task schema (3), DAG orphans/cycles/
wave-order (4), Evidence — no complete without passing verify record (5), Sync — checkboxes ⇔
`state.json` (6), Traceability — req IDs exist (7).

**Minimal surface:** command `check <slug> [--security] [--strict]`; module
`internal/core/gates` with a `Registry`, the 7 core gates, and `Finding{Gate,Severity,Message,
Ref}`; pure predicates stay in `ears.go`/`dag.go`/`specfiles.go`.

## 3. Requirements (EARS)
- **R3.1** When `check <slug>` runs, the system shall execute all registered gates in a fixed
  deterministic order and aggregate their findings.
- **R3.2** The system shall implement every gate as a pure function of `CheckCtx` performing no
  filesystem or network IO in its body.
- **R3.3** When all opt-in gates are configured `off`, the system shall produce output
  byte-identical to a build in which those gates are not registered.
- **R3.4** When a gate returns a finding of severity `error`, the system shall exit non-zero;
  when the highest severity is `warn`, it shall report but exit `0`.
- **R3.5** When a new gate is added, the system shall require exactly one registration call and
  no change to `check.go`.
- **R3.6** When `--security` is passed, the system shall register and run the stdlib-only
  security scanners as ordinary gates, honoring the same `off|warn|error` severity.
- **R3.7** When a complete task lacks a passing verify record, the evidence gate shall fail;
  this gate is never opt-out (a gate may pin a severity floor that config can raise but not
  lower).

## 4. Design

### Module boundaries
- `internal/core/gates/{registry.go, core.go, security/…}`. Pure predicates in
  `ears.go`/`dag.go`/`specfiles.go`. `internal/cmd/check.go` only builds `CheckCtx`, runs the
  registry, and formats findings.

### Key types
- `Gate interface { Name() string; Run(CheckCtx) []Finding }`; ordered `Registry`;
  `Finding{Gate, Severity, Message, Ref}`; `Severity` enum (`off|warn|error`).
- Core gates register unconditionally; opt-in gates register only when their config is
  non-`off`. **One registration point** = `registry.Register`.

### On-disk contracts
- `config.gates` map (uniform severities, one config block).
- `.specd/security/allow.json` — allowlist; **reason mandatory** (hard error if absent).

### External interfaces
- Custom-gate stdin/stdout JSON contract (unchanged); the `Gate` interface is the internal
  extension point through which the deferred flywheel (Spec 12) re-enters.

## 5. Invariants preserved (ADR-8)
Gate purity; deterministic order; evidence gate non-optional; **byte-identical output when
opt-ins off**.

## 6. Cross-domain dependencies
- Consumes: `PhaseReadiness`/`CheckCtx` (Spec 02), DAG predicates (Spec 04), verify records
  (Spec 05), the context manifest (Spec 08 — context-budget gate).
- Deferred flywheel modules (Spec 12) re-enter *only* through this interface.

## 7. Risks & open questions
- **Risk:** a registry lets third-party gates smuggle in non-determinism/IO. → `Gate.Run`
  receives only `CheckCtx` (no fs/net handles); external effects only via the vetted
  custom-gate subprocess.
- **Resolved:** gates may pin a minimum severity floor (e.g. evidence gate always `error`);
  config can raise but not lower it.
