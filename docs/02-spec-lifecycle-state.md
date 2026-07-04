# Spec Lifecycle & State Model

This document describes the lifecycle phases of a specification (spec), the directory structure, the persistence invariants, and the schema of `state.json` in `specd` (v2).

---

## 1. Spec Directory Structure

Every spec created by `specd new <slug>` is self-contained under `.specd/specs/<slug>/`:

```
.specd/specs/<slug>/
├── requirements.md     # EARS-compliant specification requirements
├── design.md           # Module boundaries, data contracts, and invariants
├── tasks.md            # Markdown list of tasks forming the execution DAG
├── state.json          # Machine-readable single source of truth for the spec
└── .lock               # Cross-process advisory lock file
```

---

## 2. The state.json Schema

The `state.json` file is the machine-readable spine of a spec. To prevent the core schema from becoming a dumping ground for plugins, `specd` uses a thin core schema supplemented by an extension map:

```json
{
  "schemaVersion": 1,
  "revision": 0,
  "mode": "simple",
  "status": "requirements",
  "phase": "plan",
  "tasks": [],
  "decisions": [],
  "midReqGates": [],
  "records": {}
}
```

### Core Schema Fields

*   **`schemaVersion` (int):** Reset to `1` in v2.
*   **`revision` (int):** Monotonically increasing revision number used for Compare-And-Swap (CAS) verification.
*   **`mode` (string):** The execution mode. Aligned with the two states described in [The New SDLC with Vibe Coding](file:///var/www/html/rai/up/specd/The_New_SDLC_With_Vibe_Coding.pdf) (p.31):
    *   `simple` (Conductor): Human-in-the-loop, real-time developer environment, no asynchronous worker delegation.
    *   `orchestrated` (Orchestrator): Async multi-agent delegation via Brain/Pinky.
*   **`status` (string):** The current phase status of the spec.
*   **`phase` (string):** Derived phase of the spec (`requirements` | `design` | `plan` | `executing` | `complete`).
*   **`records` (map[string]json.RawMessage):** Pluggable extension map. Optional/plugin evidence (e.g., security scan results, evaluation metrics) attaches here without requiring core schema bumps.

*Origin:* Simplified from the fat `State` struct in [state.go](file:///var/www/html/rai/up/specd/reference/internal/core/state.go).

---

## 3. Spec Phase Ratchet (Forward-Only)

Specs advance along a forward-only phase ratchet defined in `phases.go`. Skipping or backward transitions are rejected:

```
[requirements] ──► [design] ──► [plan] ──► [executing] ──► [complete]
```

### Phase Transition Controls

1.  **Requirements/Design Phase:** Human writes/refines Markdown.
2.  **Approve Gate:** Transitions are performed using `specd approve <slug>`. The command is rejected if the corresponding validation gates fail.
3.  **Executing Phase:** Tasks in `tasks.md` are executed and verified.
4.  **Completion Path:** The spec reaches `complete` once all tasks in the DAG are marked `complete` via evidence-gated verification.

*Origin:* Preserves the forward ratchet logic in [phases.go](file:///var/www/html/rai/up/specd/reference/internal/core/phases.go).

---

## 4. Hard Persistence Invariants

`specd` guarantees data integrity through strict file-handling rules:

### A. Reentrant Advisory Lock (`lock.go`)
Any mutating command must hold a cross-process advisory lock (`.lock` containing the PID and timestamp) under `WithSpecLock`. Mutating actions will panic or fail if executed without a held lock.

### B. Compare-And-Swap (CAS)
When writing, the system loads the current revision on disk, asserts it matches the in-memory version, bumps the revision monotonically, and then saves. If the on-disk revision differs (due to concurrent modification), the write is rejected.

### C. Atomic Writes (`io.go`)
Partial file writes are prevented by writing state changes to a temporary file, flushing via `fsync`, changing permissions (`0644`), and calling `Rename` to replace the target file atomically.

*Origin:* Verified invariants from [lock.go](file:///var/www/html/rai/up/specd/reference/internal/core/lock.go) and [io.go](file:///var/www/html/rai/up/specd/reference/internal/core/io.go).
