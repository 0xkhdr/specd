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

| Host | Adapter | Detection | Project install | Global install | stdio | HTTP | Verification depth | Known limits |
|---|---|---|---|---|---|---|---|---|
| antigravity | snippet | unsupported | manual | manual | supported | manual | config snippet only | No managed adapter or ownership inspection. |
| claude-code | project | executable or `.mcp.json` | native CLI | unsupported | supported | manual | config shape, root, ownership, MCP probe | Host restart/reload remains user-controlled. |
| claude-desktop | snippet | unsupported | unsupported | manual | supported | manual | config snippet only | Global desktop config is never mutated automatically. |
| codex | project | executable or `.codex/config.toml` | manual | unsupported | supported | manual | TOML entry and project root inspection | Current official CLI lacks safe project-scoped registration. |
| cursor | project | executable or `.cursor/mcp.json` | atomic JSON merge | unsupported | supported | manual | schema, root, ownership, MCP probe | User may need to enable/reload the server in Tools & MCP. |
| gemini | project | executable or `.gemini/settings.json` | native CLI | unsupported | supported | manual | config shape, root, ownership, MCP probe | Host trust/allow settings are preserved but not managed. |
| vscode | project | executable or `.vscode/mcp.json` | atomic JSON merge | unsupported | supported | manual | schema, root, ownership, MCP probe | Workspace trust and server start approval remain user-controlled. |

## Drift and safety policy

- Project adapters never mutate user/global configuration.
- JSON workspace adapters preserve unrelated keys, back up existing files,
  reject symlink escapes, and fall back to manual guidance on invalid or unknown
  schema.
- Unsupported cells are explicit; no transport, install scope, or verification
  depth is inferred from another host.
- Local MCP configuration is executable code. Review and trust project files
  before starting a server.
