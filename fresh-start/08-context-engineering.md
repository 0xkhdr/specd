# Domain: Context Engineering

## 1. Purpose & value mapping
- **Principles served:** P2 (plan on disk, *not* in context), P4 (per-task/per-wave
  context), P1 (harness assembles context deterministically).
- **Paper concept realized:** *context* — "Context engineering: the real skill"
  (pp.15–18). The paper's six context types (instructions, knowledge, memory, tools,
  examples, guardrails) and static-vs-dynamic context; progressive disclosure / Agent
  Skills. specd's job is to assemble exactly the right context for a unit of work and no
  more — the paper's core discipline.
- **Core use case:** for any task, `specd context` / `next --dispatch` / a worker brief
  emit a single, budgeted manifest of what to read (full/targeted), what to run, and what
  to reference only if needed — so the agent works from curated context, not a stuffed
  window ("a context window stuffed with noise," p.30).
- **If none → CUT:** N/A — the brief names this the paper's core skill and mandates it be
  first-class.

## 2. Current-state analysis (from specd)
- **Reference files read:** `internal/cmd/context.go`, `internal/context/*` (the real
  engine, `BuildContextManifest`), `internal/core/pinky_context.go`
  (`BuildMissionContextManifest` adapter), `internal/core/manifest_tools.go`,
  `internal/core/context_estimate.go`, `internal/core/context_snapshot.go`,
  `docs/agent-integration.md` (context manifest engine section).
- **What exists today; key contracts/invariants:**
  - **One shared engine, three surfaces:** `specd context`, `next --dispatch`, and the
    Pinky mission brief are all produced by `BuildContextManifest`, so they cannot drift.
    Item modes: `read-full`, `read-targeted` (slice), `run-command`, `reference-if-needed`.
  - **Package placement:** the builder lives in `internal/context` (contextpkg), *not*
    `internal/core`. `core` exposes only the adapter `BuildMissionContextManifest`
    (`pinky_context.go`) — moving the builder into core would create a documented
    `core→context→core` import cycle.
  - **Budget:** `HostContextBudgetFromEnv()` reads `SPECD_MAX_CONTEXT_TOKENS`;
    `context_estimate.go` estimates tokens with a pure heuristic (`ceil(len/4)+markdown
    surcharge`) — **no LLM/tokenizer** (No-LLM-in-context-path invariant).
    `validateMissionContextManifest` enforces version, soft-ceiling bounds, contiguous item
    order, kind/mode sets, path/command presence, and token-hint bounds.
  - `context_snapshot.go` supports resilience snapshotting for orchestration
    checkpoint/resume.
- **Redundancy / complexity / drift found (evidence):**
  - The engine is genuinely central but **under-surfaced**: `context_estimate.go` (822B)
    and `manifest_tools.go` (1.8K) are tiny, and much of the value is buried behind the
    orchestration adapter. My earlier "thin/scattered" read was wrong — the engine is in
    `internal/context`; the *core-side* footprint is a thin adapter, which is correct but
    makes the engine easy to overlook as a first-class product feature.
  - Token estimation is a heuristic (fine for determinism) but there is no *enforced* budget
    gate — over-budget manifests are validated for shape, not rejected on size in `check`.

## 3. Fresh-start decision
- **Verdict per capability:**
  - Single shared manifest engine feeding all three surfaces — **KEEP** (prevents drift; a
    genuinely good design).
  - Package separation `internal/context` ≠ `internal/core` — **KEEP** (the "make it
    central" mandate means *its own first-class package*, not folded into core; see
    `00-decisions.md`).
  - Four item modes — **KEEP** (they map onto the paper's static/dynamic + progressive
    disclosure).
  - Pure token heuristic, no LLM — **KEEP** (determinism guardrail).
  - Budget as validation-only — **REDESIGN into an enforceable gate**: a
    `context-budget` opt-in gate (domain 03) fails a task whose manifest exceeds the
    configured token budget. Context discipline becomes enforced, not advisory.
- **Minimal accurate surface:**
  - Command: `context <slug> [<task>] [--json]`; `next --dispatch` reuses the engine.
  - Modules: `internal/context/{manifest.go,estimate.go,budget.go}`; `core` adapter
    `BuildMissionContextManifest`; `manifest_tools.go` (per-spec tool policy).
  - On-disk: `.specd/specs/<slug>/manifest.json` (item + tool policy).
- **Architecture & flexibility improvements:**
  - **Promote to a documented first-class module** with its own doc (`docs/context.md`):
    the manifest is the contract every worker/brief/dispatch consumes.
  - **Map item modes to the paper's context types explicitly** so the manifest is
    self-describing: instructions (role prompt, steering) = static; knowledge/examples =
    `reference-if-needed`; the working slice = `read-targeted`; tools = the manifest tool
    policy; guardrails = the gates.
  - **Budget gate + report line:** every dispatch shows estimated tokens vs budget, and the
    gate can block — turning "context stuffed with noise" into a measurable, enforceable
    failure.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. The system shall produce `specd context`, `next --dispatch`, and worker briefs from one
   shared manifest engine so the three surfaces are always consistent.
2. The system shall support exactly the item modes `read-full`, `read-targeted`,
   `run-command`, and `reference-if-needed`.
3. When estimating context size, the system shall use a pure deterministic heuristic and
   shall never invoke a language model or network tokenizer.
4. When `SPECD_MAX_CONTEXT_TOKENS` is set and a manifest's estimate exceeds it, the
   context-budget gate (when enabled) shall fail the task.
5. When a manifest is built, the system shall validate item order, kind/mode sets, and
   path/command presence and reject a malformed manifest.
6. The system shall keep the manifest builder in `internal/context` and expose only an
   adapter from `internal/core` to avoid an import cycle.

## 5. Design notes — seed for design.md
- **Module boundaries:** `internal/context` owns `BuildContextManifest`, `EstimateTokens`,
  budget logic; `internal/core/pinky_context.go` adapts it for missions; `internal/cmd/
  context.go` + `dispatch.go` render it.
- **Key types:** `ContextManifest{Version,Items[],EstimatedTokens,Budget}`,
  `ContextItem{Path?,Command?,Mode,Kind,TokenHint}`, `ContextManifestTools`.
- **Data/on-disk contracts:** `manifest.json` per spec; `SPECD_MAX_CONTEXT_TOKENS` env.
- **Invariants to preserve:** single engine → three surfaces; no LLM in the estimate path;
  package separation (no core→context→core cycle); deterministic manifest ordering.
- **External interfaces:** the manifest JSON is the contract consumed by domain 07 (MCP
  `specd_context`), domain 09 (worker brief), domain 03 (budget gate), domain 04 (per-task
  file set feeds targeted reads).

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — engine
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T8.1 | craftsman | `internal/context/manifest.go` | — | `go test ./internal/context -run TestBuildManifest` | four item modes; deterministic order |
| T8.2 | craftsman | `internal/context/estimate.go` | — | `go test ./internal/context -run TestEstimateNoLLM` | pure heuristic; stable output |
| T8.3 | craftsman | `internal/core/pinky_context.go` | T8.1 | `go build ./... && go vet ./...` | adapter compiles; no import cycle |
### Wave 2 — surfaces & budget
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T8.4 | craftsman | `internal/cmd/context.go`, `internal/cmd/dispatch.go` | T8.1 | `diff <(go run . context demo T1 --json) <(go run . next demo --dispatch --json | jq .items)` | surfaces share the engine |
| T8.5 | craftsman | `internal/context/budget.go`, `internal/core/gates/contextbudget.go` | T8.2,T8.1 | `SPECD_MAX_CONTEXT_TOKENS=10 go run . check demo` | over-budget manifest fails gate |
| T8.6 | craftsman | `docs/context.md` | T8.1 | `grep -q read-targeted docs/context.md` | first-class doc maps modes↔paper context types |
| T8.7 | validator | `internal/context/manifest_test.go` | T8.1 | `go test ./internal/context -run TestManifestValidate` | malformed manifest rejected |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** the token heuristic diverges from real host tokenization, causing false budget
  failures. Mitigation: heuristic is conservative and only *gates* when explicitly enabled;
  the number is advisory-by-default, enforced-by-choice.
- **Open question:** should the manifest embed content or only references? Proposed:
  references + modes (the host reads), never inlined content — keeps `state`/context small
  and honors P2.
- **Cross-domain deps:** consumed by domains 07 (MCP tool), 09 (worker brief), 03 (budget
  gate), 04 (per-task files), 06 (role prompt + steering are manifest items).
