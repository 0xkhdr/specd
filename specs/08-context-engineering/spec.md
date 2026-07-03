# Spec 08 — Context Engineering

> **Authoring order:** 7 / 12 · **Critical path:** yes (feeds 03/07/09)
> **Sources:** `fresh-start/08-context-engineering.md`, paper pp.15–18
> **ADRs:** ADR-0 (package placement), ADR-8
> **Reference:** `reference/internal/context/*` (`BuildContextManifest`), `reference/internal/core/{pinky_context,manifest_tools,context_estimate,context_snapshot}.go`, `reference/internal/cmd/context.go`

For any unit of work, the system assembles exactly the right context — what to read
(full/targeted), what to run, what to reference only if needed — and no more. The paper's core
discipline, made first-class.

---

## 1. Purpose & principles
- **Principles owned:** P2 (plan on disk, not in context), P4 (per-task/per-wave context), P1
  (harness assembles context deterministically).
- **Paper concept:** *context* — "Context engineering: the real skill" (pp.15–18); six context
  types (instructions, knowledge, memory, tools, examples, guardrails); static-vs-dynamic +
  progressive disclosure.

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| Single shared manifest engine → 3 surfaces (`context`, `next --dispatch`, worker brief) | **KEEP** | Prevents drift. `reference/internal/context/*` |
| Package separation `internal/context` ≠ `internal/core` | **KEEP** | "Make it central" = its own package, not folded into core (avoids `core→context→core` cycle). ADR-0 |
| Four item modes | **KEEP** | Map to paper's static/dynamic + progressive disclosure |
| Pure token heuristic (`ceil(len/4)+md surcharge`), no LLM | **KEEP** | No-LLM-in-context-path invariant |
| Budget as validation-only | **REDESIGN** → enforceable `context-budget` opt-in gate (Spec 03) | Context discipline becomes enforced |

**Item modes (exactly):** `read-full`, `read-targeted` (slice), `run-command`,
`reference-if-needed`. **Minimal surface:** command `context <slug> [<task>] [--json]`
(`next --dispatch` reuses the engine); modules `internal/context/{manifest,estimate,budget}.go`
+ core adapter `BuildMissionContextManifest` + `manifest_tools.go`.

## 3. Requirements (EARS)
- **R8.1** The system shall produce `specd context`, `next --dispatch`, and worker briefs from
  one shared manifest engine so the three surfaces are always consistent.
- **R8.2** The system shall support exactly the item modes `read-full`, `read-targeted`,
  `run-command`, and `reference-if-needed`.
- **R8.3** When estimating context size, the system shall use a pure deterministic heuristic
  and shall never invoke a language model or network tokenizer.
- **R8.4** When `SPECD_MAX_CONTEXT_TOKENS` is set and a manifest's estimate exceeds it, the
  context-budget gate (when enabled) shall fail the task.
- **R8.5** When a manifest is built, the system shall validate item order, kind/mode sets, and
  path/command presence and reject a malformed manifest.
- **R8.6** The system shall keep the manifest builder in `internal/context` and expose only an
  adapter from `internal/core` to avoid an import cycle.

## 4. Design

### Module boundaries
- `internal/context` owns `BuildContextManifest`, `EstimateTokens`, budget logic.
- `internal/core/pinky_context.go` adapts it for missions (the only core-side surface).
- `internal/cmd/{context,dispatch}.go` render it.

### Key types
- `ContextManifest{Version, Items[], EstimatedTokens, Budget}`,
  `ContextItem{Path?, Command?, Mode, Kind, TokenHint}`, `ContextManifestTools`.
- Item modes map to paper context types: instructions (role prompt, steering) = static;
  knowledge/examples = `reference-if-needed`; working slice = `read-targeted`; tools = manifest
  tool policy; guardrails = the gates.

### On-disk contracts
- `.specd/specs/<slug>/manifest.json` (items + tool policy). `SPECD_MAX_CONTEXT_TOKENS` env.
- Manifest holds **references + modes only, never inlined content** (keeps state/context small,
  honors P2).

### External interfaces
- The manifest JSON is the contract consumed by Spec 07 (MCP `specd_context`), Spec 09 (worker
  brief), Spec 03 (budget gate), Spec 04 (per-task file set → targeted reads).

## 5. Invariants preserved (ADR-8)
Single engine → three surfaces; no LLM in the estimate path; package separation (no
core→context→core cycle); deterministic manifest ordering.

## 6. Cross-domain dependencies
- Consumed by: Specs 07, 09, 03, 04, 06 (role prompt + steering are manifest items).
- Depends on: Spec 04 (per-task files), Spec 02 (state).

## 7. Risks & open questions
- **Risk:** heuristic diverges from real host tokenization → false budget failures. →
  heuristic is conservative and only *gates* when explicitly enabled (advisory by default,
  enforced by choice).
- **Resolved:** manifest embeds references + modes, never inlined content.
