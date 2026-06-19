# Spec: MCP Prompts (Phase/Role Prompts)

> Plan item: **B2**. Wave 2 — depends on [mcp-config-tool-filtering](../mcp-config-tool-filtering/spec.md).

## 1. Overview

specd's steering files and role prompts (builder/investigator) currently reach an
agent only via tool calls. The MCP `prompts` capability is the native channel for
reusable prompt templates. This spec adds `prompts/list` and `prompts/get` so a
host loads phase-specific and role-specific prompts directly, further shrinking
the tool surface.

## 2. Goals / Non-goals

**Goals**
- Advertise `prompts` capability.
- `prompts/list` enumerates phase prompts + role prompts.
- `prompts/get` returns a prompt's messages, with `slug` substitution where relevant.

**Non-goals**
- Authoring/editing prompts over MCP (read-only).
- Dynamic prompt generation from LLM (deterministic templates only).

## 3. Foundational facts (verified)
- `route()` (server.go:83) is the extension point; capabilities in `initializeResult` (server.go:106).
- Phases in specd: requirements → design → tasks → execute (plan §B2; matches `specd context` phase-scoping and the check gates: ears/design/task-schema, commands.go:138).
- Roles: builder, investigator (plan §B2; `RolesCfg.SubagentMode`, specfiles.go:78).
- Steering content lives under `.specd/steering/` (reasoning.md, workflow.md) — the same files [mcp-resources](../mcp-resources/spec.md) exposes; prompts compose/wrap them into prompt messages.
- MCP prompt shapes: list → `{prompts:[{name, description, arguments:[{name,required}]}]}`; get → `{description, messages:[{role, content:{type:"text", text}}]}`.

## 4. Requirements (EARS)

- **R1** THE SYSTEM SHALL advertise `capabilities.prompts` (`listChanged:false`) in `initialize`.
- **R2** WHEN `prompts/list` is called, THE SYSTEM SHALL return phase prompts (`phase/requirements`, `phase/design`, `phase/tasks`, `phase/execute`) and role prompts (`role/builder`, `role/investigator`).
- **R3** WHEN `prompts/get` is called with a known name, THE SYSTEM SHALL return `messages` assembled from the embedded/steering template.
- **R4** WHERE a prompt declares a `slug` argument, `prompts/get` SHALL substitute the active spec's context into the returned messages.
- **R5** WHEN `prompts/get` is called with an unknown name, THE SYSTEM SHALL return an MCP error (prompt not found).
- **R6** THE SYSTEM SHALL keep prompt content deterministic (no network, no LLM); identical inputs ⇒ identical messages.
- **R7** THE SYSTEM SHALL declare each prompt's `arguments` (name, required) in `prompts/list` so hosts know what to pass.

## 5. Design

### 5.1 New file `internal/mcp/prompts.go`
- A registry `var prompts = []promptDef{...}` — name, description, args, and a `render(root, args) ([]message, error)` builder.
- Phase prompts wrap the relevant steering/workflow text plus a phase instruction.
- Role prompts wrap the builder/investigator role contract.
- `handlePromptsList()` and `handlePromptGet(root, name, args)`.

### 5.2 Content sourcing
Prefer embedding canonical prompt text via `embed` (stdlib) so prompts work even
without `.specd/steering/`; when steering files exist, compose them in. This keeps
prompts deterministic and offline (R6). Coordinate the source files with
[mcp-resources](../mcp-resources/spec.md) to avoid divergence.

### 5.3 Wiring (server.go)
```go
case "prompts/list": return handlePromptsList(), nil
case "prompts/get":  return handlePromptGet(root, name, args)
```

### 5.4 Argument substitution
`phase/*` prompts accept optional `slug`; when present, inject a one-line context
header (phase, slug) without invoking handlers — keep it pure string assembly.

## 6. Acceptance criteria
- **AC1** `initialize` includes `capabilities.prompts`.
- **AC2** `prompts/list` returns the 4 phase + 2 role prompts with declared arguments.
- **AC3** `prompts/get phase/design slug=X` returns design-phase messages mentioning X.
- **AC4** `prompts/get role/builder` returns the builder role contract messages.
- **AC5** unknown prompt name ⇒ error.
- **AC6** same inputs twice ⇒ identical messages (determinism).

## 7. Testing
- Unit: registry completeness; argument declarations match `render` expectations.
- Determinism: golden messages per prompt.
- Integration: list + get round-trip via server.

## 8. Risks
- **Content duplication** with steering files / resources → single embedded source-of-truth, conformance test.
- **Prompt drift** vs actual workflow → golden tests pin content; update deliberately.
