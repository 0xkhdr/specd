# Task DAG & Wave Execution

This document explains how tasks are defined in `tasks.md`, how they are parsed into a Directed Acyclic Graph (DAG), and how the harness schedules parallel execution waves.

---

## 1. The tasks.md Specification Format

Tasks are defined in a human-authored, git-diffable Markdown file (`tasks.md`) using checkbox items. Each task defines a set of metadata annotations:

```markdown
- [ ] Implement query routing interface <!-- id: T1.1 | role: craftsman | files: internal/core/routing.go | depends-on: T1.0 | verify: go test ./internal/core -run TestRouting | acceptance: Interfaces are defined with mocks -->
```

### Supported Metadata Keys

*   **`id`:** Unique identifier matching `T<number>` (e.g. `T1.1`).
*   **`role`:** Injected permission capability (`scout` | `craftsman` | `validator` | `auditor`).
*   **`files`:** Space or comma-delimited list of files this task is authorized to modify.
*   **`depends-on`:** Precedent task IDs that must be completed first.
*   **`verify`:** The exact shell command used to prove completion.
*   **`acceptance`:** Rich human-verifiable criteria for completion.

*Redesign:* Machine annotations (status, verify-ref, latency records) are kept out of `tasks.md` and moved to `state.json` to simplify git merges and keep task lists readable.

---

## 2. The Lossless Parser & Round-Trip Invariants

The tasks parser, implemented in `tasksparser.go`, operates under strict byte-stability guarantees:

*   **Byte-Stable Identity:** `SerializeTasks(ParseTasks(x)) == x` holds true for any valid file. Parsing and re-serializing a file without modifications yields a byte-identical copy.
*   **Single-Line Rewrite:** When a task's state changes (e.g. a checklist box is checked `[ ]` -> `[x]`), `specd` rewrites **only that task's line**. Every other byte, indentation, and newline remains untouched.
*   **Line-Number Stability:** The parser uses `StripHTMLComments` from [md.go](file:///var/www/html/rai/up/specd/reference/internal/core/md.go) to preserve line spacing, keeping IDE error mappings stable.

*Origin:* Key parsing contracts from [tasksparser.go](file:///var/www/html/rai/up/specd/reference/internal/core/tasksparser.go) and [md.go](file:///var/www/html/rai/up/specd/reference/internal/core/md.go).

---

## 3. DAG Graph Validation

During `specd check`, the harness builds the dependency graph and executes validation checks inside [dag.go](file:///var/www/html/rai/up/specd/reference/reference/internal/core/dag.go):

*   **Orphan Dependencies:** Verifies that no task references a parent ID that does not exist.
*   **Cycle Detection:** Executes cycle detection to block circular task dependencies.
*   **Wave Violations:** Ensures tasks are executed in valid wavefront batches (no task scheduled before its parents).

---

## 4. Wave Execution & The Frontier

The "Frontier" represents the set of tasks that are currently runnable because all of their dependencies have transitioned to `complete`.

### Command: specd next
Retrieves the next executable wavefront:

```bash
specd next <slug> [--json]
```

### Next Result Kinds

When query scheduling runs, the system returns one of four scheduler states:

1.  **`task`:** Returns a runnable task. If multiple tasks are runnable, `specd` returns them ordered by their numeric ID ordinal (e.g., `T9` runs before `T10`).
2.  **`all-complete`:** Every task in the DAG is finished.
3.  **`all-blocked`:** The graph is unfinished, but no tasks are runnable (e.g. due to failing verification gates).
4.  **`waiting`:** Tasks are running under lock/lease in orchestration mode.

### Command: specd next --waves
Replaces the old standalone `waves` command. Prints the entire task list grouped into concurrent wave execution blocks.

*Origin:* Consolidates [next.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/next.go) and [waves.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/waves.go).
