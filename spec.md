# spec.md — Level Up: Context Engineering for Pinky ⇄ Brain ⇄ specd MCP

> Goal: make `specd` deliver the **minimal sufficient context brief** to every model
> it drives — never dump noise, always hand the right slice — across all three
> context-delivery surfaces, governed by one measured token budget.

---

## 1. Scope & Intent

This spec levels up the working relationship between **Brain** (the deterministic
orchestration decision engine), **Pinky** (the host-executed worker contract), and the
**specd MCP tool surface**, optimized end-to-end for context-engineering foundations:

1. **Minimal sufficient context** — load what the task needs, nothing more.
2. **Token budgeting** — every brief carries a measured budget, not a vibe.
3. **Relevance filtering** — phase- and task-scoped, not whole-artifact dumps.
4. **Targeted retrieval** — deliver the *slice* (the task row, the covered
   requirements, the relevant design section) instead of the file.
5. **Deduplication** — never repeat the same role/skill bytes across a fan-out.
6. **Measurement** — account for delivered context and warn when a brief bloats.

The harness already separates "agent reasons / harness enforces." This work extends
that discipline to **context delivery itself**: the harness should compute and enforce
the context budget deterministically, so a non-deterministic model never has to guess
what to load.

---

## 2. Current Flow — Deep Analysis

`specd` has **three divergent context-delivery surfaces**. Only one is genuinely
context-engineered; the other two use ad-hoc hardcoded file lists.

### 2.1 Surface A — `specd context <slug>`  (single-agent / human briefing)

- Source: `internal/cmd/context.go` → `buildBrief()` + `phaseSkill()`.
- Emits a per-phase `brief{ phaseLabel, purpose, load[], focus, next }` plus `SIGNALS`
  (blockers, uncovered requirements, midreq approval gate).
- `load` is a **flat hardcoded path list** per status (e.g. design loads
  `requirements.md, design.md, tech.md, structure.md`), prepended with `baseSteering`
  (`reasoning.md`, `workflow.md`).
- **Weaknesses:**
  - No token budget. Tells the agent "minimal — don't dump the rest" but gives no
    ceiling and no per-file guidance.
  - No required-vs-optional distinction, no rationale, no load mode.
  - Whole-file loads only; no targeting (e.g. execute phase loads entire `memory.md`,
    which grows unbounded → classic context bloat).
  - File set is phase-coupled but not task-coupled — in `executing` it cannot scope to
    the actual next task's files.

### 2.2 Surface B — `specd dispatch <slug>`  (parallel subagent fan-out)

- Source: `internal/cmd/dispatch.go` → `dispatchPacket`.
- For each runnable frontier task emits: `Role`, **`RolePrompt` (full inlined role
  text)**, `Title`, `Why`, `Contract`, `Files`, `Acceptance`, `Verify`, `Depends`,
  `Requirements`, `Completion`.
- **Weaknesses:**
  - **Role prompt is fully inlined into every packet.** A 5-task wave on the same role
    repeats the entire role prompt 5× — pure duplicated context.
  - No context manifest, no token budget, no load ordering.
  - `Files` is a raw string; no per-file mode or rationale.

### 2.3 Surface C — Pinky mission brief  (orchestrated Brain → Pinky)  ← the gold standard

- Source: `internal/core/pinky_context.go` (`BuildMissionContextManifest`,
  `MissionContextManifest`/`MissionContextItem`) + `internal/core/pinky_brief.go`
  (`RenderMissionBrief`).
- Produces a real **context-engineering contract**:
  - `SoftTokenCeiling` (const `12000`, bounds `1000..200000`).
  - `Strategy` string ("load required in order, keep optional collapsed, stop before
    ceiling").
  - Ordered `Items[]`, each `{ Order, Kind, Path|Command, Mode, Required, TokenHint,
    Rationale }`.
  - Kinds: `role, skill, phase-skill, spec-context, scope-file, source-artifact`.
  - Modes: `read-full, run-command, read-targeted, reference-if-needed`.
  - Required items: role contract, Pinky skill, one phase skill, `specd context`,
    scoped files. The three source artifacts are appended `reference-if-needed`.
- **Weaknesses (even here):**
  - **`TokenHint` is hardcoded** (800/1200/1600/1800…). A 30-line and a 3000-line
    `design.md` get identical hints, so the soft ceiling is meaningless against real
    files.
  - **All three source artifacts** (requirements/design/tasks) are attached as
    references on *every* mission regardless of phase relevance.
  - **No targeted slicing** — `read-targeted` is declared as a mode but the manifest
    still points at whole files; nothing extracts just the task's row from `tasks.md`
    or just the requirements the task covers.
  - **No accounting** — nothing sums TokenHints, compares to the ceiling, or warns on
    overflow. `validateMissionContextManifest` checks structure, not budget pressure.
  - **Static ceiling** — `12000` regardless of phase, role (a read-only reviewer vs a
    multi-file builder), or host capability.

### 2.4 Cross-cutting findings

- **Three implementations of "what to load," none shared.** `buildBrief` (A),
  `dispatchPacket` (B), and `BuildMissionContextManifest` (C) each re-derive a load
  set with different vocabularies. Drift is guaranteed; a fix in one never reaches the
  others.
- **MCP already negotiates `maxTools` and `preferredNamespaces`** (see
  `internal/mcp/negotiation.go`, docs/mcp-guide.md) but there is **no equivalent
  context-budget negotiation** — the host cannot say "give me ≤ 8k context tokens."
- **No measurement anywhere.** Despite the README claim "never waste a model's
  context," nothing in the binary estimates or reports delivered context size.

---

## 3. Target Architecture

### 3.1 One context engine (`internal/core/context_manifest.go`)

Promote the Pinky manifest into the **single source of truth** for "what to load,"
consumed by all three surfaces:

```
                    ┌────────────────────────────────┐
                    │  BuildContextManifest(req)      │  ← one engine, pure, no IO-by-default
                    │  - phase + task scoping         │
                    │  - measured token estimates     │
                    │  - required/optional + budget   │
                    │  - dedupe + targeting           │
                    └───────────────┬────────────────┘
        ┌───────────────────────────┼───────────────────────────┐
        ▼                           ▼                           ▼
  specd context (A)          specd dispatch (B)          Pinky mission (C)
  human/single-agent         parallel fan-out            Brain→Pinky orchestrated
```

`ContextRequest` inputs: spec slug, status/phase, optional task id, role, host context
budget, mode (briefing | dispatch | mission). The engine returns the existing
`MissionContextManifest` shape (back-compatible: version stays `1`, fields are additive).

### 3.2 Measured token estimates (replace hardcoded TokenHint)

- Add `core.EstimateTokens(path|bytes)` — a deterministic char/word heuristic
  (e.g. `ceil(bytes/4)` with a Markdown-aware adjustment), no tokenizer dependency.
- `TokenHint` becomes the **measured** estimate of the actual artifact (or the
  targeted slice). Whole-file items measure the file; targeted items measure the slice.

### 3.3 Targeted retrieval (make `read-targeted` real)

Add deterministic slicers in `internal/core`:
- `taskSlice(tasks.md, taskID)` → just that task's block.
- `coveredRequirements(requirements.md, task.Requirements)` → only the EARS lines the
  task implements.
- `designSection(design.md, headings)` → the design section(s) named by the task, when
  the task declares them; else fall back to `reference-if-needed`.
- `recentMemory(memory.md, N)` → bounded recent-entries window instead of whole file.

A manifest item gains an optional `Slice` selector (`{kind, selector}`) so the host
knows the item is a slice, not the whole file. Hosts that ignore it degrade to reading
the whole path (graceful).

### 3.4 Budget + accounting

- `MissionContextManifest` gains computed, additive fields (omitempty for back-compat):
  `EstimatedTokens` (sum of required item hints) and `Budget` (the effective ceiling).
- New gate `context-budget` (opt-in, joins the gate registry) warns/fails `specd check`
  when required-item estimate exceeds budget, naming the heaviest items.
- `specd context --json` and dispatch packets carry the same accounting block so any
  surface can self-report.

### 3.5 Phase- and host-adaptive ceiling

- Derive the soft ceiling from `(phase, role, host budget)` instead of a flat `12000`:
  - planning phases default higher; read-only roles default lower; a multi-file builder
    scales with declared file count.
  - Honor an MCP `capabilities.specd.maxContextTokens` hint (mirrors existing
    `maxTools`) — see §3.6.
- Keep `12000` as the fallback default and the `1000..200000` bounds.

### 3.6 MCP context-budget negotiation

- Extend `internal/mcp/negotiation.go` to accept `capabilities.specd.maxContextTokens`.
- Thread it into the context engine so manifests respect the host's window. Garbage
  values ignored safely (same contract as `maxTools`). Omitting it = byte-identical to
  today.

### 3.7 Dedupe role/skill across a fan-out

- `specd dispatch` stops inlining `RolePrompt` per packet. Instead each packet
  **references** the role asset path (as the manifest already does) and the response
  carries a single `assets` map (`role/<name> → path`) shared across packets.
- Add `--inline-roles` flag to preserve old behaviour for hosts that cannot resolve
  paths (back-compat escape hatch).

---

## 4. Context-Engineering Principles → Enforcement Map

| Principle | Today | After |
|---|---|---|
| Minimal sufficient context | hardcoded per-phase lists (A/B) | one engine, phase+task scoped |
| Token budgeting | none on A/B; static hint on C | measured estimates + adaptive budget + accounting |
| Relevance filtering | all 3 source artifacts always attached | phase-filtered + task-filtered |
| Targeted retrieval | whole-file only | slicers for task/req/design/memory |
| Deduplication | role prompt inlined N× in dispatch | single shared asset map |
| Measurement | none | `context-budget` gate + accounting in every surface |

---

## 5. Non-Goals

- No LLM calls in core (preserve the no-LLM-in-core invariant; estimates are heuristic).
- No new binary dependencies (no tokenizer lib).
- No change to evidence/verify gating — context is advisory, never completion proof.
- No breaking changes to manifest version `1` wire shape — all additions are additive
  and `omitempty`; absent fields reproduce today's bytes.

---

## 6. Acceptance Criteria (EARS)

- **AC-1** WHEN `specd context <slug>` runs, the system SHALL emit a context manifest
  (required/optional items, modes, measured token hints, rationale, budget, estimate)
  produced by the shared context engine.
- **AC-2** WHEN a Pinky mission manifest is built, each item's `TokenHint` SHALL equal
  the measured estimate of the actual artifact or slice it points to (±heuristic), not
  a constant.
- **AC-3** WHEN a task declares `requirements` and/or design sections, the mission
  manifest SHALL deliver only the covered requirement lines / named design sections as
  `read-targeted` slices, not the whole artifact.
- **AC-4** WHEN required-item estimated tokens exceed the effective budget, the
  `context-budget` gate SHALL report a warning/failure naming the heaviest items.
- **AC-5** WHEN `specd dispatch <slug>` emits a multi-task wave on one role, the role
  prompt bytes SHALL appear at most once in the response (unless `--inline-roles`).
- **AC-6** WHEN a host sends `capabilities.specd.maxContextTokens`, the effective
  budget SHALL be capped to it; WHEN omitted, output SHALL be byte-identical to the
  pre-feature path.
- **AC-7** WHEN any new field is absent/zero, manifest version SHALL remain `1` and
  existing consumers SHALL parse unchanged (back-compat).
- **AC-8** The no-LLM-in-core invariant and existing gate/verify behaviour SHALL be
  unchanged; all estimates SHALL be deterministic.

---

## 7. Key Files

| Area | File |
|---|---|
| Context engine (new) | `internal/core/context_manifest.go` |
| Token estimator (new) | `internal/core/context_estimate.go` |
| Slicers (new) | `internal/core/context_slice.go` |
| Pinky manifest builder | `internal/core/pinky_context.go` |
| Mission brief render | `internal/core/pinky_brief.go` |
| Single-agent brief | `internal/cmd/context.go` |
| Dispatch fan-out | `internal/cmd/dispatch.go` |
| MCP negotiation | `internal/mcp/negotiation.go` |
| Gate registry | `internal/core/*gate*.go` (context-budget gate) |
| Docs | `docs/agent-integration.md`, `docs/mcp-guide.md`, `docs/command-reference.md` |

See `tasks.md` for the ordered implementation wave DAG.
