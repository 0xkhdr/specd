# Validation Gates Engine

The Validation Gates Engine enforces process rules and spec standards at each lifecycle transition.

---

## 1. Pluggable Gate Interface

In `specd` (v2), the gate engine has been redesigned away from hardcoded branches to a unified pluggable interface. Adding a gate requires implementing the `Gate` interface and registering it with the ordered `Registry`.

```go
type Gate interface {
	Name() string
	Run(ctx CheckCtx) []Finding
}
```

### Invariant: IO Purity
To guarantee predictability and speed, all gate bodies are **pure functions**. They perform **no filesystem or network IO**. Instead, the `CheckCtx` struct provides pre-loaded, read-only content of all spec-related files (`requirements.md`, `design.md`, `tasks.md`, `state.json`) and environment metadata.

*Origin:* Redesigned from the hardcoded conditional branches in [gates.go](file:///var/www/html/rai/up/specd/reference/internal/core/gates.go).

---

## 2. The Seven Core Gates

These gates run unconditionally during validation checkouts:

1.  **Gate 1: Requirements Format (`GateEars`):** Checks `requirements.md` syntax for EARS structure (`When <trigger>, the system shall <response>`) via [ears.go](file:///var/www/html/rai/up/specd/reference/internal/core/ears.go).
2.  **Gate 2: Design Document (`GateDesign`):** Ensures `design.md` is present and contains required module boundaries and invariant sections.
3.  **Gate 3: Task Schema (`GateTaskSchema`):** Validates task keys (`id`, `role`, `verify`, `depends-on`, `files`, `acceptance`) in `tasks.md`.
4.  **Gate 4: DAG & Waves (`GateDAG`):** Examines the task dependency graph for cycles, orphan dependencies, and out-of-wave scheduling via [dag.go](file:///var/www/html/rai/up/specd/reference/internal/core/dag.go).
5.  **Gate 5: Evidence Integrity (`GateEvidence`):** Enforces that no task status is marked `complete` without a corresponding `pass` record in the evidence ledger.
6.  **Gate 6: Markdown-State Sync (`GateSync`):** Validates that checkboxes in `tasks.md` match task status fields in `state.json` exactly.
7.  **Gate 7: Traceability (`GateTraceability`):** Asserts every task maps to at least one requirement ID.

---

## 3. Severity & Configuration

Gate behavior and severities are configured globally or per-project in `config.yml` under `gates`:

```yaml
gates:
  ears: error
  context-budget: warn
  security: error
```

### Severity Levels
*   **`off`:** Gate is not registered or run. (Opt-in gates only; core gates cannot be disabled).
*   **`warn`:** Violations are reported but CLI exit status remains `0`.
*   **`error`:** Violations block phase transitions and force a non-zero exit code.

*Byte-Stability Invariant:* When all opt-in gates are configured `off`, validation outputs are byte-identical to a build where those gates were never compiled.

---

## 4. Opt-In & Plugin Gates

### A. Context-Budget Gate
Computes the token manifest size for each task frontier (see [08-context-engineering.md](file:///var/www/html/rai/up/specd/docs/08-context-engineering.md)). Fails validation if the estimate exceeds the configured limit `SPECD_MAX_CONTEXT_TOKENS`.

### B. Pluggable Security Gate
Invoked via `specd check --security`. Registers stdlib-only scanners to check for:
*   Secret exposure in tracked files.
*   Common code injection vulnerabilities.
*   Dependency hijacking/slopsquatting.

Allowlist entries (e.g. mock test secrets) require a reason field in `.specd/security/allow.json` or verification fails.

*Origin:* Integrates security logic from [internal/core/security/](file:///var/www/html/rai/up/specd/reference/internal/core/security/).

### C. Custom External Gates (`customgate.go`)
Allows project teams to run external validation binaries. Communication occurs over stdin/stdout via a strict JSON protocol under restricted execution conditions:

```
Harness (check) ──[JSON context]──► Subprocess (External Gate)
Harness (check) ◄──[JSON findings]── Subprocess (External Gate)
```

*Origin:* Preserved from [customgate.go](file:///var/www/html/rai/up/specd/reference/internal/core/customgate.go).
