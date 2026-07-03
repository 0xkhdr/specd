# Domain: Task DAG & Wave Execution

## 1. Purpose & value mapping
- **Principles served:** P4 (Waves, Not Lines), P2 (specs on disk).
- **Paper concept realized:** *orchestration logic* — the harness computes what may run
  now and in parallel, enabling the async, multi-agent *orchestrator* mode (pp.32–33:
  "Decomposition: breaking large tasks into appropriately sized units for agent
  execution").
- **Core use case:** from a `tasks.md` DAG, `specd next` returns the runnable frontier
  (tasks whose deps are complete), and waves express concurrent batches. A human or a
  Brain dispatches the frontier; nothing runs before its dependencies are proven done.
- **If none → CUT:** N/A — this is how P4 becomes executable.

## 2. Current-state analysis (from specd)
- **Reference files read:** `internal/core/tasksparser.go`, `internal/core/dag.go`,
  `internal/cmd/next.go`, `internal/cmd/waves.go`, `internal/cmd/dispatch.go`,
  `internal/core/frontier.go`, `internal/core/md.go`.
- **What exists today; key contracts/invariants:**
  - `tasksparser.go`: entry point is **`ParseTasks(text)`** (not `ParseTasksMd`) →
    `ParsedTasks`; `SerializeTasks(doc)` round-trips. **Byte-stable round-trip** is a hard
    invariant: `encodeAnnotationField`/`decodeAnnotationField` are lossless;
    `ApplyTaskAnnotation` rewrites only the matched task line, leaving all other bytes
    untouched; `StripHTMLComments` (in `md.go`) preserves length + newlines so line
    numbers stay stable. Mandatory keys, key order, and valid roles are enforced;
    duplicate ids and out-of-wave tasks are rejected as `GateError`.
  - `dag.go` (pure): `OrphanDeps`, `DetectCycle`, `WaveViolations`, `NextRunnable`,
    `RunnableFrontier`, with `ordinal` for numeric tie-break (`T10 > T9`). `NextResult`
    kinds: `task` / `all-complete` / `all-blocked` / `waiting`.
  - `next.go` (155) queries the scheduler; `--dispatch` delegates to `dispatch.go` (171)
    which emits a typed non-empty dispatch frontier; `waves.go` (77) prints wave order.
- **Redundancy / complexity / drift found (evidence):**
  - `waves` is a standalone command that only projects DAG order — a verb that duplicates
    information `next`/`status` already have. Three commands (`next`, `waves`,
    `dispatch`) read one DAG.
  - The Markdown-with-encoded-annotations format buys byte-stability but is bespoke and
    subtle (`\m` for `·`, escaped `\n`/`\r`); it is the price of keeping `tasks.md`
    human-authorable and git-diffable.

## 3. Fresh-start decision
- **Verdict per capability:**
  - Pure DAG functions (`dag.go`) — **KEEP** (clean, well-factored, testable).
  - Byte-stable `tasks.md` parser — **KEEP** as a hard requirement, but **SIMPLIFY** the
    annotation encoding surface. Decision on format below.
  - `next` frontier query — **KEEP**.
  - `waves` command — **CUT as a verb / MERGE** its projection into `next --waves` and
    `status`.
  - `dispatch` — **KEEP** as `next --dispatch` (feeds domains 08/09), not a separate file
    of ceremony.
- **Format decision (the brief's explicit ask — bespoke Markdown vs canonical format):**
  **KEEP agent-authored Markdown `tasks.md` as the source of truth**, because P2 requires
  the plan to be human-readable and git-diffable, and switching to JSON/YAML would move
  authorship away from the agent and reviewers. But **REDESIGN the annotation channel**:
  move machine annotations (status, verify-ref, telemetry) out of inline encoded fields
  and into `state.json` (already machine truth), leaving `tasks.md` as *clean* Markdown
  whose only machine-load-bearing content is the checkbox + the metadata keys. This
  shrinks the lossless-encoding surface (fewer escape rules) while *strengthening* the
  round-trip guarantee (less to round-trip). See `00-decisions.md`.
- **Minimal accurate surface:**
  - Commands: `next [--waves|--dispatch|--json]`.
  - Modules: `tasksparser.go` (parse/serialize + round-trip), `dag.go` (pure graph),
    `frontier.go` (frontier events).
- **Architecture & flexibility improvements:**
  - **Property-based round-trip test** as a first-class gate: `Serialize(Parse(x)) == x`
    for a fuzz corpus — the round-trip stops being a hope and becomes a proof.
  - **Single reader:** one `LoadTasks(root,slug)` used by `next`, `check`, and the context
    engine, so the three cannot diverge on parsing.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. When `tasks.md` is parsed and re-serialized without edits, the system shall produce a
   byte-identical file.
2. When a single task's status changes, the system shall rewrite only that task's line and
   leave every other byte of `tasks.md` unchanged.
3. When the task graph contains a cycle, an orphan dependency, or a task outside its wave,
   the system shall reject the file with a gate error naming the offending id and line.
4. When `next <slug>` is invoked, the system shall return the runnable frontier — all tasks
   whose dependencies are complete — ordered by numeric ordinal.
5. When no task is runnable, the system shall report exactly one of `all-complete`,
   `all-blocked`, or `waiting`.
6. When `next --waves` is invoked, the system shall print concurrent batches; there shall
   be no separate `waves` command.

## 5. Design notes — seed for design.md
- **Module boundaries:** `tasksparser.go` (text ⇄ `ParsedTasks`), `dag.go` (pure graph
  over `DagTask`), `frontier.go` (change events); `next.go` (cmd) orchestrates.
- **Key types:** `ParsedTask{ID,Role,Files,DependsOn,Verify,Acceptance,Status}`,
  `ParsedTasks`, `DagTask`, `NextResult{Kind,Task}`, `NextResultKind`.
- **Data/on-disk contracts:** `tasks.md` (clean Markdown, human-authored, byte-stable);
  machine annotations live in `state.json` (domain 02).
- **Invariants to preserve:** byte round-trip; single-line rewrite; numeric ordinal
  tie-break; pure DAG functions; stable line numbers via `StripHTMLComments`.
- **External interfaces:** `LoadTasks`, `RunnableFrontier`, `NextRunnable`.

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — parser & round-trip
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T4.1 | craftsman | `internal/core/tasksparser.go`, `internal/core/md.go` | — | `go test ./internal/core -run TestTasksRoundTrip` | Serialize∘Parse is identity over fuzz corpus |
| T4.2 | craftsman | `internal/core/tasksparser.go` | T4.1 | `go test ./internal/core -run TestSingleLineRewrite` | status change rewrites one line only |
### Wave 2 — graph & scheduler
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T4.3 | craftsman | `internal/core/dag.go` | T4.1 | `go test ./internal/core -run TestDAG` | cycle/orphan/wave violations detected |
| T4.4 | craftsman | `internal/cmd/next.go`, `internal/core/frontier.go` | T4.3 | `go run . next demo --json` | frontier ordered by ordinal; correct terminal kinds |
| T4.5 | craftsman | `internal/cmd/next.go` | T4.4 | `go run . next demo --waves` | wave projection replaces `waves` command |
| T4.6 | validator | `internal/core/tasksparser_fuzz_test.go` | T4.1 | `go test ./internal/core -run FuzzTasks -fuzztime=30s` | no round-trip violation found |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** moving annotations out of `tasks.md` into `state.json` could desync the two.
  Mitigation: the existing Sync gate (domain 03, Gate 6) already checks checkbox↔state
  agreement; keep it and extend to verify-ref presence.
- **Open question:** do we keep numeric `T<n>` ids or move to `<wave>.<n>`? Proposed: keep
  `T<n>` with numeric ordinal tie-break (already correct); waves are a grouping, not an id
  scheme.
- **Cross-domain deps:** feeds domain 03 (DAG gate), domain 08 (per-task context), domain
  09 (Brain dispatches the frontier), domain 02 (status↔checkbox sync).
