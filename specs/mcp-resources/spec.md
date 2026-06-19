# Spec: MCP Resources (Spec-Artifact-as-Resource)

> Plan item: **B1**. Wave 2 — depends on [mcp-config-tool-filtering](../mcp-config-tool-filtering/spec.md)
> (cfg plumbing + capability advertisement).

## 1. Overview

Today every byte of context flows through tools; reading a spec artifact costs a
`specd_context` tool call. The MCP `resources` capability is the native channel
for readable content. This spec adds `resources/list` and `resources/read` so
hosts read spec artifacts, state, and steering files directly — letting
`specd_context`/`specd_status` shrink to lightweight metadata and removing
artifact-reading from the tool surface.

## 2. Goals / Non-goals

**Goals**
- Advertise `resources` capability in `initialize`.
- `resources/list` enumerates spec artifacts + steering files as URIs.
- `resources/read` returns artifact content by URI.
- Strict path containment: only `.specd/` artifacts are readable.

**Non-goals**
- Writing resources (read-only channel).
- Resource subscriptions / `resources/subscribe` (future; pairs with [mcp-dynamic-tool-list](../mcp-dynamic-tool-list/spec.md)).
- Removing `specd_context` (deprecate later, not in this spec).

## 3. Foundational facts (verified)
- `route()` (server.go:83) switches on method; add `resources/list` + `resources/read` cases. Unknown methods already return `errMethodNotFound`.
- Capabilities advertised in `initializeResult` (server.go:106): currently only `tools.listChanged:false`. Add a `resources` entry.
- Spec artifacts live under `.specd/specs/<slug>/` (six artifacts per `specd new`, commands.go:46). Steering files under `.specd/steering/` (per plan §B1: `reasoning.md`, `workflow.md`).
- The server holds project root via `--root` (`internal/cmd/mcp.go`); resource reads resolve against it.
- MCP resource read result shape: `{contents:[{uri, mimeType, text}]}`.

## 4. Requirements (EARS)

- **R1** THE SYSTEM SHALL advertise `capabilities.resources` (with `listChanged:false` initially) in the `initialize` response.
- **R2** WHEN `resources/list` is called, THE SYSTEM SHALL return one resource entry per existing artifact across all specs plus steering files, each with `uri`, `name`, and `mimeType`.
- **R3** THE SYSTEM SHALL use a stable URI scheme: `specd://specs/<slug>/<artifact>` and `specd://steering/<file>`.
- **R4** WHEN `resources/read` is called with a known URI, THE SYSTEM SHALL return the file content with the correct `mimeType` (`text/markdown` for `.md`, `application/json` for `.json`).
- **R5** WHEN `resources/read` is called with an unknown or non-existent URI, THE SYSTEM SHALL return an MCP error (resource not found) without filesystem disclosure.
- **R6** THE SYSTEM SHALL reject any URI whose resolved path escapes `.specd/` (path traversal); reads stay within the project root.
- **R7** THE SYSTEM SHALL keep `resources/list` ordering deterministic (slug order, then a fixed artifact order).
- **R8** Resources SHALL be read-only: no mutating resource method is exposed.

## 5. Design

### 5.1 New file `internal/mcp/resources.go`
- `handleResourcesList(root) map[string]any` — walk `.specd/specs/*/` for the known artifact filenames + `.specd/steering/*.md`; emit only files that exist (R2).
- `handleResourceRead(root, uri) (map[string]any, *rpcError)` — parse scheme, map to path, `filepath.Clean` + containment check (R6), read, infer mime (R4).
- A small URI parser/validator with a closed set of allowed prefixes.

### 5.2 Wiring (server.go)
Add to `route()`:
```go
case "resources/list":
    return handleResourcesList(root), nil
case "resources/read":
    return handleResourceRead(root, uriFromParams(req.Params))
```
The conn must carry `root` (added in Wave 1's cfg plumbing — extend that struct).

### 5.3 Containment
Resolve `filepath.Join(root, ".specd", rel)`, then `filepath.Clean`, then verify
`strings.HasPrefix(clean, specdRoot+sep)`. Reject otherwise (R6). No symlink
following beyond root.

### 5.4 Artifact set
Reuse the canonical artifact filename list from `core` (the `specd new` six
artifacts + `state.json`). If a single source-of-truth list exists in `core`,
import it; otherwise define one constant and add a conformance test against
`specd new` output to prevent drift.

## 6. Acceptance criteria
- **AC1** `initialize` response includes `capabilities.resources`.
- **AC2** `resources/list` on a project with 2 specs lists each spec's existing artifacts + steering files, deterministically ordered.
- **AC3** `resources/read specd://specs/<slug>/tasks.md` returns the file text with `mimeType:text/markdown`.
- **AC4** `resources/read specd://specs/<slug>/state.json` returns `application/json`.
- **AC5** `resources/read specd://../../etc/passwd` (and any traversal) ⇒ error, no content.
- **AC6** unknown URI ⇒ resource-not-found error.

## 7. Testing
- Unit: URI parse/containment table tests incl. traversal vectors (AC5).
- Integration: temp `.specd/` with specs; list + read round-trip (AC2–AC4).
- Conformance: artifact filename list matches `specd new` output.

## 8. Risks
- **Path traversal** is the key security risk → explicit containment tests required.
- **Capability advertisement** must not break hosts that ignore unknown caps (additive only, safe).
