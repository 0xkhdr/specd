# Spec: MCP Host Capability Negotiation

> Plan item: **C2** (speculative, future-proof). Wave 4 — depends on
> [mcp-config-tool-filtering](../mcp-config-tool-filtering/spec.md) and
> [mcp-dynamic-tool-list](../mcp-dynamic-tool-list/spec.md).

## 1. Overview

Extend `initialize` so a host can hint `maxTools` and `preferredNamespaces`; the
server respects them when building the tool list. This is **non-standard** (not
in the MCP spec), so it must be strictly additive and degrade silently for hosts
that send nothing.

## 2. Goals / Non-goals

**Goals**
- Parse optional `capabilities.specd.{maxTools,preferredNamespaces}` from `initialize`.
- Cap and prioritise the emitted tool list accordingly.
- Zero behaviour change for hosts that omit the field.

**Non-goals**
- Forcing the standard MCP spec to adopt this (purely a specd extension).
- Overriding `forbiddenTools` (manifest) or config hard-gates.

## 3. Foundational facts (verified)
- `initializeParams` (server.go:98) currently parses only `protocolVersion`; capabilities are ignored. Extend the struct additively.
- Negotiated values must persist for the session (store on the conn alongside cfg).
- Tool ordering today is command-order then intent-order (tools.go:91); namespace prioritisation reorders this view only.
- Namespaces emerge from composite naming: `specd_inspect`/`specd_read`/`specd_query` (read), `specd_orchestrate`/`specd_worker` (orchestration), meta. Define prefix→namespace mapping.

## 4. Requirements (EARS)

- **R1** WHEN `initialize` includes `capabilities.specd.maxTools`, THE SYSTEM SHALL emit at most that many tools, prioritising essential/required ones.
- **R2** WHEN `capabilities.specd.preferredNamespaces` is present, THE SYSTEM SHALL order matching-namespace tools first and prefer them when truncating to `maxTools`.
- **R3** WHEN neither field is present, THE SYSTEM SHALL behave exactly as without this feature (no reorder, no cap).
- **R4** THE SYSTEM SHALL never drop a tool required by config/manifest to satisfy `maxTools`; safety gates win (if required > maxTools, emit required and a stderr diagnostic).
- **R5** THE SYSTEM SHALL persist negotiated preferences for the session and apply them to every subsequent `tools/list` (including dynamic re-fetches).
- **R6** Unknown/garbage values SHALL be ignored safely (treated as absent) — never an error tearing down `initialize`.

## 5. Design

### 5.1 Parse (server.go)
Extend `initializeParams`:
```go
type initializeParams struct {
    ProtocolVersion string `json:"protocolVersion"`
    Capabilities    struct {
        Specd struct {
            MaxTools            int      `json:"maxTools"`
            PreferredNamespaces []string `json:"preferredNamespaces"`
        } `json:"specd"`
    } `json:"capabilities"`
}
```
Store parsed prefs on the conn.

### 5.2 Apply (buildTools)
After config/phase/manifest filtering produces the candidate set:
1. Partition by namespace (prefix→namespace map).
2. Stable-sort: preferred namespaces first (in given order), then default order.
3. Truncate to `maxTools`, but force-keep config/manifest-required tools first (R4).

### 5.3 Degradation
maxTools ≤ 0 or empty namespaces ⇒ no-op (R3, R6). Validation is lenient: clamp,
ignore unknown namespaces.

## 6. Acceptance criteria
- **AC1** `maxTools:5` ⇒ ≤ 5 tools emitted.
- **AC2** `preferredNamespaces:["specd_read"]` ⇒ read tools ordered first.
- **AC3** required tool count > maxTools ⇒ required tools still emitted + stderr line.
- **AC4** no `specd` capability ⇒ identical to feature-off output.
- **AC5** garbage values (negative maxTools, unknown namespace) ⇒ safe no-op.

## 7. Testing
- Unit: truncation + ordering with required-tool protection (AC1–AC3).
- Backward-compat: omit field ⇒ identical golden (AC4).
- Robustness: fuzz/garbage inputs (AC5).

## 8. Risks
- **Non-standard extension** may confuse spec-strict hosts → namespaced under `capabilities.specd`, ignored by others; never required.
- **Truncation hides needed tools** → required-protection (R4) + diagnostics.
- **Low impact / high effort** (plan matrix) → lowest priority; ship only after Waves 1–3 prove value.
