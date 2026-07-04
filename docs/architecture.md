# Architecture — Package Map & Data Flow

> This document describes the compiled Go package structure, the data-flow through a
> typical command, the on-disk layout enforced by the harness, and the hard invariants
> every implementation must preserve. For the *why* behind every design decision, see
> `PROJECT.md` §4 (ADRs) and §6 (domain decisions).

---

## 1. The core equation

```
Agent = Model + Harness
```

`specd` is the harness half. The model (any coding agent — Claude Code, Cursor, Codex,
Aider, or any MCP client) supplies reasoning. `specd` supplies:

| Harness component | What it does |
|---|---|
| **instructions** | Role-prompt files + steering constitution handed to the agent. |
| **tools** | Deterministic CLI verbs the agent can call as tools via MCP. |
| **sandboxes** | Scrubbed, bounded `verify:` execution environments. |
| **orchestration** | DAG / wave / Brain controller that sequences work. |
| **guardrails** | Gates that block a state change without proof or human approval. |
| **observability** | Truthful projections of `state.json` — never LLM-generated. |
| **context** | Bounded, progressive-disclosure context manifests. |

---

## 2. Package map

```
github.com/0xkhdr/specd
│
├── main.go                     entry point — arg parse → cmd.Run
│
├── internal/cli/               thin arg parser (~40 lines; no Cobra)
│   └── ParseArgs, Usage
│
├── internal/core/              the harness spine (zero dependencies)
│   ├── commands.go             Commands[] — the one source of truth for help + MCP
│   ├── state.go                State{} model, SaveStateCAS, LoadState, StampRecord
│   ├── phases.go               Status/Phase enums + forward-only ratchet
│   ├── tasksparser.go          ParseTasksMd / SerializeTasks (byte-round-trip)
│   ├── dag.go                  OrphanDeps, DetectCycle, WaveViolations (pure)
│   ├── frontier.go             Frontier / NextRunnable (pure)
│   ├── task_complete.go        CompleteTask — the one completion path
│   ├── evidence.go             AppendEvidence / LoadEvidence (append-only ledger)
│   ├── io.go                   AtomicWrite, AppendFile (temp→fsync→rename)
│   ├── lock.go                 WithSpecLock — reentrant advisory lock + stale reclaim
│   ├── scaffold.go             WriteScaffold — init templates
│   ├── config_loader.go        LoadConfig (pure YAML-subset, fail-loud)
│   ├── config_validate.go      config validation + secret-scrubbing
│   ├── paths.go                SpecdDir, StatePath, EvidencePath …
│   ├── roles.go                embedded role-prompt files
│   ├── memory.go               steering-memory add/promote
│   ├── report.go               RenderStatus / BuildReportModel (pure projection)
│   ├── report_metrics.go       RenderMetrics — Prometheus textfile projection
│   ├── prsummary.go            PRSummary — GitHub PR-ready Markdown summary
│   ├── slug.go                 ValidateSlug — ^[a-z0-9][a-z0-9-]*$
│   │
│   ├── gates/                  pluggable gate registry (ADR-4)
│   │   ├── registry.go         Gate interface + ordered Registry
│   │   ├── core.go             CoreRegistry() — 7 core gates registered unconditionally
│   │   ├── ears.go             Gate 1 — EARS requirement shape
│   │   ├── approval.go         Gates 2/6 — design-stub + approval/sync checks
│   │   ├── sync.go             Gate 6 — checkbox ↔ state.json agreement
│   │   ├── contextbudget.go    Gate opt-in — context token budget
│   │   └── security/           Gate opt-in — secrets/injection scanner (--security)
│   │
│   └── verify/                 verify executor
│       └── Run — scrubbed env, exit-code capture, sandbox opt-in
│
├── internal/cmd/               thin command handlers (no business logic)
│   ├── registry.go             executable map + Run dispatcher
│   ├── lifecycle.go            new, approve, task, verify, midreq, decision, help …
│   ├── memory.go               memory add/promote handlers
│   ├── brain.go / brain_run.go brain orchestration verbs
│   └── brain_worker.go         brain worker seam (test-only)
│
├── internal/context/           context manifest engine (ADR-0: kept separate to avoid cycle)
│   ├── BuildManifest           one function, two surfaces (CLI + MCP)
│   ├── estimate.go             EstimateText — pure heuristic ceil(len/4)
│   └── RenderHUD               operator HUD renderer
│
├── internal/mcp/               stdio JSON-RPC 2.0 MCP server
│   ├── Serve                   main loop
│   └── CoreTools()             tool set derived from Commands[]
│
├── internal/integration/       host adapter interface
│   └── HostAdapter             Detect/Plan/Install/Inspect/Verify
│
└── internal/orchestration/     Brain/Pinky — opt-in, inert by default (ADR-3)
    ├── Decide(Snapshot) → Decision   pure function, zero IO, zero randomness
    ├── Sense                         builds snapshot from state + frontier + leases
    └── file-backed ACP               acp/*.jsonl, session.json (CAS-guarded)
```

---

## 3. Command data-flow

A typical command follows this path — using `specd verify <slug> <task>` as the example:

```
os.Args
  │
  ▼
cli.ParseArgs          (pure, no IO)
  │ Command="verify", Pos=["myspec","T3"], Flags={}
  ▼
cmd.Run(".", "verify", …)
  │ looks up Registry["verify"] → runVerify
  ▼
runVerify
  ├─ requireTaskGate()        reads state.json → fails if not approved
  ├─ loadSpec()               reads tasks.md   → ParseTasksMd (round-trip-safe)
  ├─ verifyexec.Run()         sh -c task.Verify in scrubbed env
  ├─ gitHead()                git rev-parse HEAD
  ├─ core.AppendEvidence()    AtomicWrite to evidence.jsonl (append-only)
  └─ exit code                0 = pass, 1 = fail, 2 = usage
```

Every command that mutates state goes through `core.WithSpecLock` + `core.SaveStateCAS`:

```
WithSpecLock (reentrant advisory lock)
  └─ LoadState                read current revision
  └─ [business logic]
  └─ SaveStateCAS(expectedRevision, newState)
       ├─ re-reads revision on disk (CAS check)
       ├─ increments revision
       └─ AtomicWrite          temp → fsync → chmod 0644 → rename
```

---

## 4. On-disk layout

```
<project-root>/
└── .specd/
    ├── roles/
    │   ├── scout.md            read-only explore role (embedded template)
    │   ├── craftsman.md        write + verify role
    │   ├── validator.md        read-only test-run role
    │   └── auditor.md          read-only diff-audit role
    │
    ├── steering/
    │   ├── reasoning.md        agent reasoning constitution
    │   ├── workflow.md         workflow rules
    │   ├── product.md          product context (agent-authored)
    │   ├── tech.md             tech stack context (agent-authored)
    │   ├── structure.md        repo structure (agent-authored)
    │   └── memory.md           global steering memory
    │
    └── specs/
        └── <slug>/
            ├── requirements.md   EARS-shaped requirements (agent-authored)
            ├── design.md         module boundaries + invariants (agent-authored)
            ├── tasks.md          task DAG in pipe-table Markdown (agent-authored)
            ├── state.json        machine truth: status, phase, revision, records
            ├── evidence.jsonl    append-only evidence ledger (exit code + git HEAD)
            ├── memory.md         per-spec steering memory
            └── .lock             advisory lock file (pid + unix-ms)
```

### `state.json` schema (SchemaVersion: 1)

```json
{
  "schema_version": 1,
  "slug": "my-spec",
  "mode": "default",
  "status": "tasks",
  "phase": "plan",
  "revision": 7,
  "records": {
    "approval:requirements": { "kind": "approval", "gate": "requirements", "timestamp": "…", "git_head": "…", "actor": "…" },
    "approval:design":       { "kind": "approval", "gate": "design",       "timestamp": "…", "git_head": "…", "actor": "…" },
    "decision:0":            { "kind": "decision",  "text": "chose X because Y", "timestamp": "…", "git_head": "…", "actor": "…" }
  },
  "task_status": {
    "T1": "complete",
    "T2": "pending"
  }
}
```

> **Mode values:** currently `"default"` (simple/conductor mode) and `"agent"` (agent-driven
> mode). ADR-7 specifies the final enum as `"simple"` / `"orchestrated"`; the rename is
> an open finding (F5, Wave P1). Code reference: `internal/core/state.go` `ModeDefault` /
> `ModeAgent`.

### `tasks.md` row schema (6-key pipe table)

```markdown
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T1 | craftsman | src/foo.go | - | go test ./... | all unit tests pass |
| T2 | validator | - | T1 | go test -run TestFoo | TestFoo is green |
```

- `id` — `T<n>`, unique per spec; wave order is numeric (`T10 > T9`).
- `role` — exactly one of `scout | craftsman | validator | auditor`.
- `files` — comma-separated paths touched by this task.
- `depends-on` — comma-separated task IDs, or `-`.
- `verify` — shell command (`sh -c`); must be runnable; NUL bytes rejected.
- `acceptance` — human-readable completion criterion.

---

## 5. The lifecycle (phase ratchet)

```
requirements  →  design  →  tasks  →  executing  →  verifying  →  complete
   perceive      analyze     plan      execute        verify        reflect

      ▲              ▲         ▲
      │              │         │
   approve        approve   approve
 (requirements)  (design)  (tasks)
```

- **Forward-only.** `AdvanceStatus` rejects backward or skipping transitions.
- **Gate-gated.** `approve` runs the full gate registry before writing state.
- **Blocking gates.** Any `error`-severity gate finding blocks the approval.
- **Human-only approval.** MCP `ForbiddenTool` policy prevents agents from calling `approve`.

---

## 6. The gate registry (ADR-4)

```go
type Gate interface {
    Name() string
    Run(ctx CheckCtx) []Finding
}
```

`CoreRegistry()` registers 7 gates unconditionally. Gates are **pure functions** over
`CheckCtx` — no IO, no network. Adding a gate = one `registry.Register(myGate{})` call.

| # | Gate | Severity | Can opt-out? |
|---|---|---|---|
| 1 | EARS | error | No |
| 2 | Design stub | error | No |
| 3 | Task schema | error | No |
| 4 | DAG (orphans/cycles/wave order) | error | No |
| 5 | Evidence | error | **Never** |
| 6 | Sync (checkboxes ↔ state) | error | No |
| 7 | Traceability (req IDs) | warn | No |
| opt | Context budget | warn/error | Yes (enable in config) |
| opt | Security (secrets/injection) | error | Yes (`--security` flag) |

---

## 7. Hard invariants (ADR-8)

These invariants must hold at every commit. Any change requires a new recorded ADR.

| Invariant | Enforcement |
|---|---|
| **Atomic writes** | `AtomicWrite`: temp → fsync → chmod 0644 → rename |
| **CAS on revision** | `SaveStateCAS` re-reads revision before write; panics in test builds if `SaveState` runs unlocked |
| **Reentrant advisory lock** | `WithSpecLock`: goroutine-id reentrancy, stale reclaim at 30s, 5s acquire timeout |
| **Parser byte round-trip** | `Serialize(Parse(x)) == x`; property + fuzz tested; single-line rewrite on status change |
| **Embedded templates** | One `go:embed` in `embed_templates/`; no disk-relative reads at runtime |
| **Zero runtime dependencies** | `go.mod` has no `require`; single static binary |
| **Evidence integrity** | No task completes without a passing verify record referencing a real git HEAD |
| **Determinism** | Gates, DAG, reports are pure functions of on-disk state; no LLM or network in any render/decide path |

---

## 8. Context manifest engine (ADR-0 / Domain 08)

One shared function powers three surfaces:

```
specd context <slug> <task>        ──┐
                                     ├── context.BuildManifest(root, slug, tasks, taskID, budget)
next --dispatch <slug>             ──┤
MCP `context` tool                 ──┘
```

**Parity guarantee:** all three surfaces call `BuildManifest` with the same arguments
so their output is byte-identical for the same input state.

**Four disclosure modes** (cheapest → most expensive):

| Mode | What is returned |
|---|---|
| `reference` | File path only — agent reads if needed |
| `read-summary` | Computed digest (heading/first-line level) |
| `read-targeted` | Only the slice that matters (e.g. one task row) |
| `read-full` | Whole file — used only for role prompts |

**Token estimation** (pure, LLM-free):
```
estimated_tokens(text) = ceil(len(text) / 4)
```
Config key `context.max_tokens` (env `SPECD_CONTEXT_MAX_TOKENS`), default 12000.

---

## 9. Orchestration tier (ADR-3, Domain 09)

The Brain is a **deterministic controller that never calls an LLM**:

```go
Decide(Snapshot) → Decision{Action, Task?, Reason}
```

- `Sense` builds the snapshot from state + frontier + leases.
- Actions: `dispatch | wait | await-approval | escalate | policy-violation | complete`.
- All Brain↔worker interaction via **file-backed ACP** (append-only `acp/*.jsonl`).
- **Fail-closed**: disabled unless `orchestration.enabled: true` in config **and** `mode: orchestrated` in state.

---

## 10. MCP server (Domain 07)

```
specd mcp   →   stdio JSON-RPC 2.0   →   any MCP client
```

Tool set is data-driven from `Commands[]` — the same registry that drives help.
Parity test: `tool result == CLI JSON for the same input`.

**ForbiddenTool policy** (agent cannot call):
`approve · init · mcp · brain`

Registered only when orchestration is enabled:
`brain_orchestrate · brain_status · brain_approve`
