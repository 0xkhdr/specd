# Host capability record — OpenAI Codex CLI

> Investigator findings for spec `mcp-expansion`, Task T2 (Requirement 2).
> Captured 2026-06-17 from primary sources. Discovery only — no integration code.

## Capability record

| field         | value                                                                       |
|---------------|-----------------------------------------------------------------------------|
| host          | OpenAI Codex CLI                                                             |
| **native MCP**| **yes** — "Codex supports MCP servers in both the CLI and the IDE extension"|
| **stdio**     | **true** — local process started by a `command`                             |
| **http**      | **true** — Streamable HTTP servers via `url`                                |
| **sse**       | **false** — docs list stdio + Streamable HTTP only; SSE not mentioned       |
| configPath    | `~/.codex/config.toml` (global); `.codex/config.toml` (trusted project scope)|
| configSyntax  | TOML `[mcp_servers.<name>]` tables; see below                               |

## Config syntax

Stdio server (verbatim fields):
```toml
[mcp_servers.<server-name>]
command = "..."                       # required
args    = [...]                       # optional
env     = {...}                       # optional
env_vars = [...]                      # optional
cwd     = "..."                       # optional
```

HTTP server (verbatim fields):
```toml
[mcp_servers.<server-name>]
url                  = "..."          # required
bearer_token_env_var = "..."          # optional
http_headers         = {...}          # optional
env_http_headers     = {...}          # optional
```

Shared options: `startup_timeout_sec`, `tool_timeout_sec`, `enabled`, `required`, tool-approval
settings. Servers can also be managed with `codex mcp`.

### Implication for specd (R2.3)

Codex CLI natively supports stdio MCP servers, so **no non-stdio transport is required**. The
thinnest path (Requirement 3.1) is a declarative `config.toml` `[mcp_servers.specd]` entry with
`command = "specd"`, `args = ["mcp", ...]`. Streamable HTTP (`url`) is available if a remote
deployment is ever wanted; SSE is not offered by Codex.

Copy-paste config: [`codex.config.toml`](codex.config.toml) — merge its `[mcp_servers.specd]`
table into `~/.codex/config.toml` and replace the placeholder `--root` path.

## Sources

- [Model Context Protocol — Codex | OpenAI Developers](https://developers.openai.com/codex/mcp) — official; native support, stdio + Streamable HTTP, config.toml syntax
- [Configuration Reference — Codex | OpenAI Developers](https://developers.openai.com/codex/config-reference) — official; `mcp_servers` table options
- [Codex CLI — ToolUniverse Documentation](https://zitniklab.hms.harvard.edu/ToolUniverse/guide/building_ai_scientists/codex_cli.html) — worked stdio `[mcp_servers.*]` example
