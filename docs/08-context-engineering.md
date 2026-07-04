# Context Engineering

This document outlines the design and implementation of the context manifest engine in `specd` (v2), which manages agent context sizes.

---

## 1. Principles of Lean Context

As detailed in [The New SDLC with Vibe Coding](file:///var/www/html/rai/up/specd/The_New_SDLC_With_Vibe_Coding.pdf) (pp.15–18), stuffing a model's context window with irrelevant files leads to poor reasoning, high latency, and increased cost. 

`specd` enforces **Context Discipline** by compiling a minimal, targeted context manifest for each task:
*   **Static Context:** System rules, steering constitution, and role prompts.
*   **Dynamic Context:** The specific file slices authorized for the task.
*   **Progressive Disclosure:** Large reference documents are listed as "reference-if-needed" rather than inlined by default.

---

## 2. The Context Manifest Engine

All context surfaces (`specd context`, `next --dispatch`, and the Pinky worker mission briefs) derive from a single shared engine in `internal/context`:

```go
func BuildContextManifest(ctx CheckCtx, taskID string) (*ContextManifest, error)
```

### Package Separation (Core/Context)
To prevent circular imports, the manifest builder lives in `internal/context` (contextpkg), not `internal/core`. The core package uses a thin adapter `BuildMissionContextManifest` (defined in `pinky_context.go`) to feed worker missions.

---

## 3. Manifest Item Modes

A `ContextManifest` lists the precise set of assets the agent is authorized to read or run:

*   **`read-full`:** Read-only access to full specifications (`requirements.md`, `design.md`, `tasks.md`). Maps to static instructions.
*   **`read-targeted`:** Read-only access to targeted code files declared in the task's `files` attribute. Maps to dynamic context.
*   **`run-command`:** Authorization to execute the task's specific `verify` script. Maps to tools.
*   **`reference-if-needed`:** Reference file paths that the agent should only query if specific questions arise. Maps to knowledge bases.

*Origin:* Standardized from the modes in [agent-integration.md](file:///var/www/html/rai/up/specd/reference/docs/agent-integration.md).

---

## 4. Token Estimation Heuristic

To calculate the token usage of a manifest, `specd` uses a deterministic size estimator. 

*No-LLM Invariant:* The token estimator is a pure heuristic function (`ceil(bytes/4) + markdown surcharge`) implemented in [context_estimate.go](file:///var/www/html/rai/up/specd/reference/internal/core/context_estimate.go). It performs **no model invocations and no network calls**, ensuring fast and reproducible runs.

---

## 5. Enforceable Context-Budget Gate

The context budget can be enforced using the opt-in `context-budget` gate (see [03-validation-gates.md](file:///var/www/html/rai/up/specd/docs/03-validation-gates.md)). 

```bash
# Set maximum allowed context tokens
export SPECD_MAX_CONTEXT_TOKENS=8000
specd check <slug>
```

If the task's manifest estimate exceeds the token ceiling, the validation gate fails, blocking phase advances and preventing agents from run-away context sprawl.
