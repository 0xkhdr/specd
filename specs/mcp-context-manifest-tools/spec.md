# Spec: MCP Context-Manifest-Driven Tool Loading

> Plan item: **C1**. Wave 4 — depends on
> [mcp-config-tool-filtering](../mcp-config-tool-filtering/spec.md) and
> [mcp-dynamic-tool-list](../mcp-dynamic-tool-list/spec.md).

## 1. Overview

specd already produces a context manifest concept (`specd pinky brief`). This
spec lets a spec's manifest declare `requiredTools` / `optionalTools` /
`forbiddenTools`; the MCP server reads the active spec's manifest and filters the
tool list accordingly — the most precise, per-spec control layer on top of the
config and phase filters.

## 2. Goals / Non-goals

**Goals**
- Read a per-spec `contextManifest` tool policy.
- Intersect manifest policy with the config/phase exposure plan.
- `forbiddenTools` always wins (hard exclude).

**Non-goals**
- Authoring manifests (consumed, not written, here).
- Changing the `pinky brief` output contract beyond adding the optional fields.

## 3. Foundational facts (verified)
- `pinky brief` exists as a `pinky` sub-action (commands.go:260); it already emits a mission/context brief. The manifest tool fields are an additive extension.
- Wave 1's `resolveMCPExposure` is the single filter chokepoint — manifest filtering composes after it.
- Active spec is derivable from the request context (slug arg) or the watcher's tracked spec (Wave 3).
- Manifest lives with the spec under `.specd/specs/<slug>/` (read-only to MCP).

## 4. Requirements (EARS)

- **R1** WHERE a spec declares a `contextManifest` with tool fields, THE SYSTEM SHALL apply them when building the tool list for that spec's context.
- **R2** THE SYSTEM SHALL include `requiredTools` (forced present, subject to R4) and `optionalTools` (allowed) and exclude everything else when a manifest is present and `expose` permits manifest mode.
- **R3** `forbiddenTools` SHALL be excluded unconditionally, overriding required/optional/config.
- **R4** WHEN a `requiredTool` is also meta/orchestration-gated off by config, THE SYSTEM SHALL exclude it and emit a stderr diagnostic (config gate wins over manifest "required" for safety).
- **R5** WHEN no manifest exists, THE SYSTEM SHALL fall back to the config/phase exposure plan unchanged.
- **R6** THE SYSTEM SHALL keep manifest parsing read-only and deterministic.

## 5. Design

### 5.1 Manifest schema (additive)
```json
{ "contextManifest": {
    "requiredTools":  ["specd_inspect","specd_verify","specd_task"],
    "optionalTools":  ["specd_decision","specd_memory"],
    "forbiddenTools": ["specd_approve"] } }
```
Tool names use the MCP `specd_*` namespace (post-composite). Validate names
against the known tool set; unknown names ⇒ stderr warning, ignored.

### 5.2 Filter composition (precedence)
`forbidden` > config-gate > `required`/`optional` allowlist > phase plan. Implement
as a final pass over the exposure plan: start from config/phase set, restrict to
`required∪optional` (if manifest present), then subtract `forbidden`, then re-apply
config gates (R4).

### 5.3 Reading the manifest
Add `core.LoadContextManifest(root, slug)` returning the tool policy (empty when
absent). The MCP server calls it for the active spec; integrates with the Wave 3
watcher so manifest changes can also trigger `list_changed` (optional, reuse R3
of dynamic spec).

## 6. Acceptance criteria
- **AC1** Manifest `requiredTools` present ⇒ exactly those (+ optional) exposed.
- **AC2** `forbiddenTools` excludes a tool even if config would include it.
- **AC3** A `requiredTool` gated off by config ⇒ excluded + stderr line (AC R4).
- **AC4** No manifest ⇒ identical to config/phase output.

## 7. Testing
- Unit: precedence matrix (forbidden vs required vs config gate).
- Integration: spec with manifest ⇒ asserted tool subset.
- Conformance: unknown tool names ignored with warning.

## 8. Risks
- **Manifest/config conflict** surprises users → explicit precedence + diagnostics.
- **Name drift** after composite renames → validate against live tool set.
