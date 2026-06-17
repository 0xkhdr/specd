# Host capability record — Antigravity CLI

> Investigator findings for spec `mcp-expansion`, Task T1 (Requirement 1).
> Captured 2026-06-17 from primary/secondary sources. Discovery only — no integration code.

## Capability record

| field         | value                                                                      |
|---------------|----------------------------------------------------------------------------|
| host          | Antigravity CLI (Google, `~/.gemini/antigravity-cli`)                       |
| **stdio**     | **true** — local-command MCP servers via `command` + `args` + `env`        |
| **http**      | **true** — remote MCP servers via `serverUrl`                              |
| **sse**       | **true** — remote `serverUrl` endpoints are HTTP/SSE (streamable)          |
| configPath    | `~/.gemini/config/mcp_config.json` (shared by Antigravity 2.0, IDE, CLI); CLI also reads `~/.gemini/antigravity-cli/mcp_config.json` and workspace-local `.agents/mcp_config.json` |
| configSchema  | top-level `mcpServers` object; see below                                   |

## Config schema

Top-level object `mcpServers`, one key per server. Per-entry fields (verbatim):

- `command` — executable name (stdio servers)
- `args` — array of command arguments (stdio)
- `env` — environment variables object (stdio)
- `serverUrl` — HTTP endpoint for remote servers (replaces the deprecated `httpUrl`)
- `authProviderType` — auth method, e.g. `"google_credentials"`
- `headers` — HTTP headers for remote requests
- `oauth` — `{ clientId, clientSecret }` for OAuth remote servers
- `disabled` — boolean toggle

Stdio note: set `MCP_MODE: "stdio"` / `DISABLE_CONSOLE_OUTPUT: "true"` in `env` so debug
output does not leak onto stdout and corrupt the JSON-RPC stream.

### Implication for specd (R1.3)

Antigravity supports stdio MCP servers, so **no HTTP/SSE transport adapter is required on
Antigravity's account**. The thinnest path (Requirement 3.1) is a declarative
`mcp_config.json` entry launching `specd mcp` over stdio. HTTP remains available via
`serverUrl` if a remote deployment is ever wanted.

Copy-paste config: [`antigravity.config.json`](antigravity.config.json) — merge its
`mcpServers.specd` entry into `~/.gemini/config/mcp_config.json` and replace the placeholder
`--root` path.

## Sources

- [Antigravity Editor: MCP Integration](https://antigravity.google/docs/mcp) — official (JS-rendered; content cross-checked below)
- [Configuring MCP Servers and Skills for Antigravity CLI and IDE — Dazbo, Google Cloud Community / Medium](https://medium.com/google-cloud/configuring-mcp-servers-and-skills-for-antigravity-cli-and-ide-a938c7eebb78) — dated **2026-05-24**; `serverUrl` schema, deprecation of `httpUrl`
- [Google Workspace MCP servers in Google Antigravity — Google Codelabs](https://codelabs.developers.google.com/google-workspace-mcp-antigravity) — `serverUrl` + OAuth remote example, shared central config path
- [n8n-mcp ANTIGRAVITY_SETUP.md](https://github.com/czlonkowski/n8n-mcp/blob/main/docs/ANTIGRAVITY_SETUP.md) — stdio `MCP_MODE`/`DISABLE_CONSOLE_OUTPUT` requirement
