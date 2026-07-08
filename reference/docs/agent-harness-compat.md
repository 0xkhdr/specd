# Agent-Harness Compatibility Matrix

This matrix is the tested support contract for coding-agent integration. The
adapter registry, embedded `specd mcp --config` snippets, and the host rows below
are kept in sync by `Compatibility`, `Conformance`, and `Host` tests.

## Transports

| Transport | Status | Notes |
|---|---|---|
| stdio | supported | Default local transport. JSON-RPC 2.0 over stdin/stdout with newline and Content-Length framing. |
| HTTP `/rpc` | supported | Opt-in with `specd mcp --http`; loopback by default. |
| HTTP `/sse` | supported | Opt-in SSE response framing over the same dispatcher. |

## Hosts

`project` in the Adapter column means `specd init --agent <host>` can detect and
inspect that host. `snippet` means only deterministic manual configuration is
shipped. Named host integrations use stdio; HTTP remains a manual endpoint path.
`Brain/Pinky tools` means `specd init --repair` can prove the server exposes
`specd_brain` and `specd_pinky`; it does **not** mean specd can spawn, reload, or
trust a coding-agent host.

| Host | Adapter | Detection | Project install | Global install | stdio | HTTP | Brain/Pinky tools | Host lifecycle requirements | Verification depth | Known limits |
|---|---|---|---|---|---|---|---|---|---|---|
| antigravity | project | executable or `.agents/mcp_config.json` | direct JSON merge | unsupported | supported | manual | server probe + registered config | User reloads the host if tools are not visible. | config shape, root, ownership, MCP probe | Host reload remains user-controlled. |
| claude-code | project | executable or `.mcp.json` | native CLI | unsupported | supported | manual | server probe + registered config | User restarts/reloads host if tools are not visible. | config shape, root, ownership, MCP probe | Host restart/reload remains user-controlled. |
| claude-desktop | snippet | unsupported | unsupported | manual | supported | manual | server probe only | User edits global config and restarts desktop app. | config snippet only | Global desktop config is never mutated automatically. |
| codex | project | executable or `.codex/config.toml` | manual | unsupported | supported | manual | server probe + registered config | User merges/reloads project MCP config according to host support. | TOML entry and project root inspection | Current official CLI lacks safe project-scoped registration. |
| cursor | project | executable or `.cursor/mcp.json` | atomic JSON merge | unsupported | supported | manual | server probe + registered config | User enables/reloads server in Tools & MCP. | schema, root, ownership, MCP probe | User may need to enable/reload the server in Tools & MCP. |
| vscode | project | executable or `.vscode/mcp.json` | atomic JSON merge | unsupported | supported | manual | server probe + registered config | User approves workspace trust and starts/reloads MCP server. | schema, root, ownership, MCP probe | Workspace trust and server start approval remain user-controlled. |

## Orchestration compatibility boundary

`specd init --repair` now separates two evidence layers:

1. **Server capability:** in-process MCP initialize + `tools/list`, requiring the
   baseline CLI tools plus `specd_brain` and `specd_pinky`.
2. **Host lifecycle:** host-managed reload/trust/start behavior. specd may write
   or verify project-scoped MCP configuration for supported adapters, but the
   host remains responsible for starting agents and invoking Pinky tools.

No compatibility row claims provider-specific model access, background agent
spawn control, host process termination, or automatic trust approval.

## Drift and safety policy

- Project adapters never mutate user/global configuration.
- JSON workspace adapters preserve unrelated keys, back up existing files,
  reject symlink escapes, and fall back to manual guidance on invalid or unknown
  schema.
- Unsupported cells are explicit; no transport, install scope, or verification
  depth is inferred from another host.
- Local MCP configuration is executable code. Review and trust project files
  before starting a server.
