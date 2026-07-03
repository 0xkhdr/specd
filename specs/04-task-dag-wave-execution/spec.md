# Spec 04 — Task DAG & Wave Execution

> **Authoring order:** 4 / 12 · **Critical path:** feeds 03/08/09
> **Sources:** `fresh-start/04-task-dag-wave-execution.md`, paper pp.32–33
> **ADRs:** ADR-1, ADR-8
> **Reference:** `reference/internal/core/{tasksparser,dag,frontier,md}.go`, `reference/internal/cmd/{next,waves,dispatch}.go`

From a `tasks.md` DAG, the system computes the runnable frontier and concurrent waves. A human
or a Brain dispatches the frontier; nothing runs before its dependencies are proven done.

---

## 1. Purpose & principles
- **Principles owned:** P4 (Waves, Not Lines), P2 (specs on disk).
- **Paper concept:** *orchestration logic* — "Decomposition: breaking large tasks into
  appropriately sized units for agent execution" (pp.32–33).

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| Pure DAG functions (`dag.go`) | **KEEP** | Clean, well-factored, testable |
| Byte-stable `tasks.md` parser (`ParseTasks`) | **KEEP** (hard invariant), **SIMPLIFY** annotation surface | ADR-1; corrected name is `ParseTasks` (ADR-0) |
| Annotation channel (inline encoded fields) | **REDESIGN** → machine state to `state.json` | ADR-1; leaves clean Markdown |
| `next` frontier query | **KEEP** | `reference/internal/cmd/next.go` |
| `waves` command | **CUT as verb / MERGE** into `next --waves` | Duplicate projection |
| `dispatch` | **KEEP** as `next --dispatch` | Feeds Specs 08/09 |

**Format decision (the brief's explicit ask):** KEEP agent-authored Markdown `tasks.md` as
source of truth (P2 requires human-readable, git-diffable); do **not** switch to JSON/YAML
(would move authorship away from the agent). REDESIGN only the annotation *channel*: machine
annotations (status, verify-ref, telemetry) move to `state.json`, shrinking the lossless-encode
surface while *strengthening* the round-trip guarantee. (ADR-1.)

**Minimal accurate surface:** command `next [--waves|--dispatch|--json]`; modules
`tasksparser.go`, `dag.go`, `frontier.go`.

## 3. Requirements (EARS)
- **R4.1** When `tasks.md` is parsed and re-serialized without edits, the system shall produce
  a byte-identical file.
- **R4.2** When a single task's status changes, the system shall rewrite only that task's line
  and leave every other byte of `tasks.md` unchanged.
- **R4.3** When the task graph contains a cycle, an orphan dependency, or a task outside its
  wave, the system shall reject the file with a gate error naming the offending id and line.
- **R4.4** When `next <slug>` is invoked, the system shall return the runnable frontier — all
  tasks whose dependencies are complete — ordered by numeric ordinal (`T10 > T9`).
- **R4.5** When no task is runnable, the system shall report exactly one of `all-complete`,
  `all-blocked`, or `waiting`.
- **R4.6** When `next --waves` is invoked, the system shall print concurrent batches; there
  shall be no separate `waves` command.

## 4. Design

### Module boundaries
- `tasksparser.go` — text ⇄ `ParsedTasks`. `dag.go` — pure graph over `DagTask`.
- `frontier.go` — change events. `next.go` (cmd) orchestrates.
- **Single reader:** one `LoadTasks(root, slug)` used by `next`, `check`, and the context
  engine, so the three cannot diverge on parsing.

### Key types
- `ParsedTask{ID, Role, Files, DependsOn, Verify, Acceptance, Status}`, `ParsedTasks`,
  `DagTask`, `NextResult{Kind, Task}`, `NextResultKind`.

### On-disk contracts
- `tasks.md` — clean Markdown, human-authored, byte-stable. Machine annotations live in
  `state.json` (Spec 02). `StripHTMLComments` preserves length + newlines so line numbers stay
  stable.

### External interfaces
- `LoadTasks`, `RunnableFrontier`, `NextRunnable`.

## 5. Invariants preserved (ADR-8)
Byte round-trip (`Serialize(Parse(x)) == x`, now **property/fuzz-tested**); single-line
rewrite; numeric ordinal tie-break; pure DAG functions; stable line numbers.

## 6. Cross-domain dependencies
- Feeds: Spec 03 (DAG gate), Spec 08 (per-task context), Spec 09 (Brain dispatches frontier),
  Spec 02 (status↔checkbox sync).
- Depends on: Spec 02 (state), Spec 10 (io).

## 7. Risks & open questions
- **Risk:** moving annotations out of `tasks.md` desyncs the two. → the Sync gate (Spec 03,
  Gate 6) checks checkbox↔state agreement; extend to verify-ref presence.
- **Resolved:** keep `T<n>` ids with numeric ordinal tie-break; waves are a grouping, not an
  id scheme.
