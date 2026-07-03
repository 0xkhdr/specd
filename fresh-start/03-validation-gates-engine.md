# Domain: Validation Gates Engine

## 1. Purpose & value mapping
- **Principles served:** P1 (harness enforces), P3 (Evidence Gates Every State Change).
- **Paper concept realized:** *guardrails* + the "Testing & QA / Feedback Loop"
  (pp.29–30) — deterministic checks that route failures back rather than trusting
  model output. The paper: *"The harness runs deterministic hooks."*
- **Core use case:** `specd check <slug>` runs a fixed battery of pure gates over the
  spec artifacts + state and returns violations/warnings deterministically. This is the
  mechanism that lets a human safely delegate the 80% — the harness catches the classes
  of failure the paper names (missing edge cases, drift, unverified completion).
- **If none → CUT:** N/A — the gate engine is the enforcement half of P1.

## 2. Current-state analysis (from specd)
- **Reference files read:** `docs/validation-gates.md`, `internal/core/gates.go`,
  `internal/core/ears.go`, `internal/core/specfiles.go`, `internal/core/dag.go`,
  `internal/cmd/check.go`, `internal/cmd/security.go`, `internal/core/customgate.go`.
- **What exists today; key contracts/invariants:**
  - **7 core gates** as pure functions over a shared read-only `CheckCtx`:
    `RunGates(c) → (violations, warnings []Violation)`. Individual gates:
    `GateEars` (Gate 1, EARS shape via `ears.go`), `GateDesign` (2), `GateTaskSchema`
    (3), `GateDAG` (4, orphans/cycles/wave order via `dag.go`), `GateEvidence` (5,
    no complete task without a passing verify record), `GateSync` (6, `tasks.md`
    checkboxes match `state.json`), `GateTraceability` (7, req IDs exist).
  - **Opt-in gates 8–13** already exist but are keyed off config and no-op by default,
    so `check` output is byte-identical to a build without them: Acceptance (8), Scope
    (9), Eval (10), Review (11), Security (12), Ingest (13). Plus deploy preconditions
    and harness-import quarantine (enforced outside `check`), and fully external
    `customgate.go` (`config.gates.custom`, scrubbed env, bounded timeout).
  - **Invariant:** gate bodies do no IO — `CheckCtx → ([]Violation,[]Violation)`.
- **Redundancy / complexity / drift found (evidence):**
  - The opt-in gates are wired as **hardcoded conditional branches** in the check
    pipeline keyed by config strings (`config.gates.acceptance`, `.scope`, `.eval`,
    etc.), each with bespoke on/warn/error handling. This is exactly the "hardcoded
    branches" the brief flags — adding a gate touches `check.go` + `gates.go` +
    `approve.go` + config in four places (`docs/contributor-guide.md`, extension recipe).
  - Severity semantics are inconsistent across gates (some `off/warn/error`, some
    `off/*/warn/error`, review is boolean `required`).

## 3. Fresh-start decision
- **Verdict per capability:**
  - 7 core gates as pure functions — **KEEP** (they are P3 made mechanical).
  - Gate IO-purity invariant — **KEEP**.
  - Hardcoded opt-in branches — **REDESIGN** into a **pluggable gate interface**: a gate
    is a value implementing `Gate{ Name() string; Run(CheckCtx) []Finding }` registered
    into an ordered registry; severity is uniform (`off|warn|error`) and lives in one
    config block. Core gates register unconditionally; opt-in gates (acceptance, scope,
    context-budget, security, and later eval/review/ingest) register only when their
    config is non-`off`.
  - Custom external gates — **KEEP** (already a clean external contract).
  - Security scanners — **KEEP-as-plugin-gate** (stdlib-only, off by default — the model
    case for the new interface).
  - Eval/Review/Ingest gates — **DEFER their commands** (domain 12) but **keep their gate
    hooks** so the interface is proven against real modules from day one.
- **Minimal accurate surface:**
  - Command: `check <slug> [--security] [--strict]`.
  - Module: `internal/core/gates` with a `Registry`, the 7 core gates, and a
    `Finding{ Gate, Severity, Message, Ref }` type; `ears.go`, `dag.go`, `specfiles.go`
    supply the pure predicates.
- **Architecture & flexibility improvements:**
  - **One registration point.** Adding a gate = implement `Gate` + `registry.Register`.
    No edits to `check.go`. Kills the four-place drift.
  - **Uniform severity + one config block** (`gates: { <name>: off|warn|error }`).
  - **Ordered, deterministic run** so `check` output is byte-stable (critical for the
    "byte-identical when all opt-ins off" property from `docs/validation-gates.md`).
  - **Context-budget gate** (new): a first-class opt-in gate that fails when a task's
    projected context manifest exceeds the configured token budget (domain 08) — turns
    context discipline into an enforceable guardrail.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. When `check <slug>` runs, the system shall execute all registered gates in a fixed
   deterministic order and aggregate their findings.
2. The system shall implement every gate as a pure function of `CheckCtx` performing no
   filesystem or network IO in its body.
3. When all opt-in gates are configured `off`, the system shall produce output
   byte-identical to a build in which those gates are not registered.
4. When a gate returns a finding of severity `error`, the system shall exit non-zero;
   when the highest severity is `warn`, it shall report but exit `0`.
5. When a new gate is added, the system shall require exactly one registration call and
   no change to `check.go`.
6. When `--security` is passed, the system shall register and run the stdlib-only
   security scanners as ordinary gates, honoring the same `off|warn|error` severity.
7. When a complete task lacks a passing verify record, the evidence gate shall fail
   (this gate is never opt-out).

## 5. Design notes — seed for design.md
- **Module boundaries:** `internal/core/gates/{registry.go,core.go,security/…}`; pure
  predicates stay in `ears.go`/`dag.go`/`specfiles.go`; `check.go` (cmd) only builds
  `CheckCtx`, runs the registry, and formats findings.
- **Key types:** `Gate` interface; `Finding{Gate,Severity,Message,Ref}`; `Severity`
  enum; `Registry` (ordered).
- **Data/on-disk contracts:** `config.gates` map (uniform severities);
  `.specd/security/allow.json` allowlist (reason mandatory — hard error if absent).
- **Invariants to preserve:** gate purity; deterministic order; evidence gate
  non-optional; byte-identical output when opt-ins off.
- **External interfaces:** custom-gate stdin/stdout JSON contract (unchanged); the
  `Gate` interface as the internal extension point.

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — interface & core gates
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T3.1 | craftsman | `internal/core/gates/registry.go` | — | `go test ./internal/core/gates -run TestRegistryOrder` | deterministic ordered run |
| T3.2 | craftsman | `internal/core/gates/core.go`, `internal/core/ears.go`, `internal/core/dag.go` | T3.1 | `go test ./internal/core/gates -run TestCoreGates` | 7 core gates pass/fail correctly, no IO |
| T3.3 | craftsman | `internal/cmd/check.go` | T3.1,T3.2 | `go run . check demo` | check runs registry only; exit codes correct |
### Wave 2 — opt-in modules on the new interface
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T3.4 | craftsman | `internal/core/gates/security/*` | T3.1 | `go run . check demo --security` | scanners run as gates; allowlist reason mandatory |
| T3.5 | craftsman | `internal/core/gates/contextbudget.go` | T3.1 | `go test ./internal/core/gates -run TestContextBudgetGate` | fails when manifest over budget |
| T3.6 | validator | `internal/core/gates/parity_test.go` | T3.2 | `go test ./internal/core/gates -run TestByteIdenticalWhenOptInsOff` | output byte-identical with opt-ins off |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** a registry lets third-party gates smuggle in non-determinism/IO. Mitigation:
  `Gate.Run` receives only `CheckCtx` (no fs/net handles); external effects only via the
  vetted custom-gate subprocess path.
- **Open question:** should severity be per-gate-config only, or can a gate declare a
  minimum severity floor (e.g. evidence gate is always `error`)? Proposed: gates may pin
  a floor; config can raise but not lower it.
- **Cross-domain deps:** consumes `PhaseReadiness`/`CheckCtx` (domain 02), the DAG
  predicates (domain 04), verify records (domain 05), and the context manifest (domain
  08); the deferred flywheel modules (domain 12) re-enter *only* through this interface.
