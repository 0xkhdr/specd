# Model Context Protocol (MCP) Integration Guide

`specd` ships with a native, zero-dependency Model Context Protocol (MCP) server that runs over
standard input/output (`stdio`). This allows MCP hosts (Claude Desktop, Cursor, VS Code, and
others) to discover, inspect, and invoke the entire `specd` toolset without network calls or
external APIs.

The MCP server is a thin JSON-RPC 2.0 transport over the existing CLI handlers; it adds no
business logic and introduces no new binary dependencies.

---

## Startup & Invocation Contract

Start the server with the `specd mcp` command:

```bash
specd mcp [--root /path/to/project] [--http [<addr>]] [--config <host>]
```

- **`specd mcp`** starts a JSON-RPC 2.0 stdio server and runs until stdin closes (EOF). No
  network port is bound; all communication is byte-for-byte on `stdin`/`stdout`.
  (`internal/mcp/server.go`, `Serve` function)
- **`--root <dir>`** scopes every tool call to the named project directory via a one-time
  `chdir` before the loop starts. Without it, the working directory of the spawning process
  is used as the spec root — the same fallback the CLI uses.
  (`internal/cmd/mcp.go:23-28`)
- **`--http [<addr>]`** opts into the HTTP/SSE transport instead of stdio. Bare `--http`
  (no value) defaults to `127.0.0.1:8765`. The server runs until it stops; exits `0` on clean
  shutdown, `1` on bind error. See [HTTP/SSE Transport](#httpsse-transport) below.
  (`internal/mcp/transport_http.go`, `ServeHTTP`)
- **`--config <host>`** prints a ready-to-paste config snippet for the named host and exits
  immediately without starting any server. (`internal/cmd/mcp.go:25-27`)
- If the `--root` path cannot be entered, the server exits with **usage code 2** before any
  protocol bytes are emitted. The error message goes to stderr.
  (`internal/cmd/mcp.go:27`, `core.ExitUsage = 2`)
- **Only protocol bytes appear on stdout.** All diagnostic messages (errors, warnings) go to
  stderr, which MCP hosts typically capture separately.

---

## Wire Framing

`specd mcp` speaks JSON-RPC 2.0 and **auto-detects the framing style** from the first
non-whitespace byte received — a single connection-lifetime decision:

| First byte | Framing selected | Description |
|---|---|---|
| `C` | **Content-Length** (LSP-style) | `Content-Length: N\r\n\r\n<body>` |
| anything else | **Newline-delimited** | One JSON object per line |

(`internal/mcp/transport.go:73-98`, `readMessage`)

### Newline-delimited JSON (default)
Most MCP SDK clients and CLI tools send newline-delimited JSON:
```
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}\n
```
Each response is likewise one JSON object followed by `\n`.

### Content-Length header framing (LSP)
Hosts that speak the Language Server Protocol wire format (some VS Code extensions) send:
```
Content-Length: 42\r\n
\r\n
{"jsonrpc":"2.0","id":1,"method":"initialize",...}
```
The `Content-Length` header value is the byte length of the JSON body. Responses use the same
format for the connection's lifetime. **No specd-side configuration is needed** — framing is
detected automatically.

### Protocol version
The `initialize` result always advertises protocol version **`2024-11-05`**:
```json
{
  "protocolVersion": "2024-11-05",
  "capabilities": { "tools": { "listChanged": false } },
  "serverInfo":   { "name": "specd", "version": "<core.Version>" }
}
```
(`internal/mcp/server.go:84-90`)

### Error handling
A malformed JSON request or invalid framing never terminates the loop; the server replies with
a JSON-RPC error and keeps reading:

| Condition | Error code |
|---|---|
| Malformed JSON body | `-32700` (parse error) |
| Valid JSON but invalid JSON-RPC envelope | `-32600` (invalid request) |
| Unknown method | `-32601` (method not found) |
| Bad params or unknown tool name | `-32602` (invalid params) |

(`internal/mcp/transport.go`, `server.go:handle`)

---

## HTTP/SSE Transport

For MCP hosts that cannot spawn stdio processes (browser-based tools, remote agents, hosts
that require an HTTP endpoint), `specd mcp --http` starts an HTTP server exposing the same
JSON-RPC 2.0 dispatch as the stdio path.

### When to use `--http` vs stdio

Use stdio (default) for all local desktop hosts (Claude Desktop, Cursor, VS Code). Use
`--http` when the host requires a network endpoint — a browser extension, a remote agent, or
a host process that cannot manage child process I/O directly.

### Routes

| Route | Method | Behavior |
|---|---|---|
| `/rpc` | `POST` | Single JSON-RPC 2.0 request body → JSON-RPC response. HTTP 200 even on JSON-RPC errors (errors are in the response body envelope). |
| `/sse` | `GET` or `POST` | Same dispatch; response wrapped as one SSE `data:` frame (`data: <json>\n\n`). An empty body is a valid no-op that opens the stream. |

(`internal/mcp/transport_http.go`, `httpHandler`)

### Default address and security

Bare `--http` (no value) binds `127.0.0.1:8765`. A port-only address like `--http :9000` is
rewritten to `127.0.0.1:9000`. Loopback binding is enforced unless an operator supplies an
explicit external IP — spec contents never leave the host by default.

(`internal/mcp/transport_http.go`, `loopbackAddr`)

### Concurrent call serialization

The stdio loop is serial by nature. The HTTP path allows concurrent requests, which would
interleave captured stdout. A mutex serializes all tool calls regardless of how many
concurrent HTTP requests arrive — behavior is identical to stdio, just multi-client.

(`internal/mcp/transport_http.go`, `dispatchLocked`)

### Example

```bash
# Bind default loopback:8765
specd mcp --http --root /path/to/project

# Bind explicit port (still loopback)
specd mcp --http :9000 --root /path/to/project

# Single JSON-RPC call via curl
curl -s -X POST http://127.0.0.1:8765/rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"specd_status","arguments":{}}}'
```

---

## Tool Mapping Contract

The server automatically maps every registered command in `specd` — except the meta commands
`help`, `version`, and `mcp` — to an MCP tool at startup. **No separate registration is
needed**: a new command added to `core.Commands` surfaces as a tool with the next binary build.
(`internal/mcp/tools.go:81-90`, `buildTools`)

### Naming
All tools are prefixed `specd_` to prevent collisions in the host's shared tool list:
`specd_status`, `specd_next`, `specd_task`, etc.
(`internal/mcp/tools.go:7`, `toolPrefix`)

### Input schema
Each tool's input schema has:
- **`args`** — an ordered string array for positional CLI arguments (matches the command's
  usage synopsis).
- **One property per flag** — booleans as `boolean`, everything else as `string`.

```json
{
  "name": "specd_task",
  "arguments": {
    "args": ["my-feature", "T1"],
    "status": "complete",
    "evidence": "Tests passed"
  }
}
```

### Annotations
Every tool carries MCP annotations so a host can signal risk before invoking:

| Annotation | Value | Commands |
|---|---|---|
| `readOnlyHint` | `true` | `status`, `waves`, `context`, `check`, `next`, `dispatch`, `report`, and read-only variants (`serve`, `watch`, `validate`, `replay`, `diff`) |
| `readOnlyHint` | `false` | All other commands (state-mutating) |
| `destructiveHint` | `true` | `uninstall`, `update` (mutate the install itself) |

(`internal/mcp/tools.go:16-23`)

### Output schema
Tool results follow the MCP `tools/call` contract:

```json
{
  "content": [{ "type": "text", "text": "<command stdout>" }],
  "isError": false,
  "structuredContent": { ... }
}
```

- **`content`**: The command's stdout. On failure (`isError: true`), stderr diagnostics are
  appended so the host sees a complete error picture.
- **`isError`**: `true` when the CLI command exits with a non-zero code.
- **`structuredContent`**: When stdout is valid JSON, it is parsed and attached as a structured
  object — the host LLM does not need to parse text manually.

(`internal/mcp/server.go:134-160`, `toolResult`)

---

## Exposed MCP Tools

The following tools are exposed automatically. Refer to the
[Command Reference](./command-reference.md) for full parameter details:

| MCP Tool | CLI Analog | Primary Purpose | Read-Only |
|---|---|---|---|
| `specd_init` | `specd init` | Scaffold `.specd/` directory and role configurations | No |
| `specd_new` | `specd new` | Create a new spec with artifact stubs | No |
| `specd_approve` | `specd approve` | Clear phase approval gate | No |
| `specd_decision` | `specd decision` | Record architectural decision (ADR) | No |
| `specd_midreq` | `specd midreq` | Log a mid-flight requirement change | No |
| `specd_memory` | `specd memory` | Record or promote a learning | No |
| `specd_next` | `specd next` | Print the next runnable task(s) | Yes |
| `specd_dispatch` | `specd dispatch` | Emit subagent ready-to-run packets | Yes |
| `specd_verify` | `specd verify` | Execute task verification command | No |
| `specd_task` | `specd task` | Flip task status with evidence | No |
| `specd_status` | `specd status` | Render status board or list specs | Yes |
| `specd_check` | `specd check` | Run the validation gates | Yes |
| `specd_waves` | `specd waves` | Render task wave dependency graph | Yes |
| `specd_context` | `specd context` | Retrieve minimal phase briefing and signals | Yes |
| `specd_report` | `specd report` | Generate progress reports | Yes |
| `specd_serve` | `specd serve` | Run read-only HTTP server | Yes |
| `specd_watch` | `specd watch` | Stream runnable-frontier changes | Yes |
| `specd_validate` | `specd validate` | Conformance check against JSON Schema | Yes |
| `specd_replay` | `specd replay` | Render spec audit event timeline | Yes |
| `specd_diff` | `specd diff` | Show git diff of spec artifacts | Yes |
| `specd_program` | `specd program` | Manage inter-spec dependency graph | No |
| `specd_update` | `specd update` | Update the specd binary | No (destructive) |
| `specd_uninstall` | `specd uninstall` | Remove specd from the system | No (destructive) |

---

## Host Configuration

### Automated setup (recommended)

For the managed adapters — **claude-code, codex, cursor, gemini, vscode** — you do not
need to edit any config by hand. `specd init` detects the host and registers the
server for you, **project-scoped**:

```bash
specd init --agent auto            # detect and configure the unambiguous host
specd init --agent claude-code --yes
specd init --agent all --yes       # every detected host
specd doctor                       # verify registration + MCP handshake
```

Where the host ships a stable CLI, specd uses it (`claude mcp add --scope project`,
`gemini mcp add --scope project`); otherwise it performs a targeted JSON merge that
preserves your other servers. See [agent-harness-compat.md](agent-harness-compat.md)
for the per-host install method and verification depth.

#### Trust boundaries

- **Project scope by default.** specd writes only inside your repo. Global/user host
  config is never modified without `--scope global` **and** explicit consent.
- **Fail closed.** Existing host config must parse before any change; on a mutation
  specd writes a timestamped backup first and reports its path.
- **Preservation.** Unrelated MCP servers and settings are never removed or rewritten.
- **No secrets.** Environment secrets are never copied into generated config.
- **Host-native trust stands.** specd never bypasses a host's own trust/approval
  prompt, and never starts or controls the agent. Restart/reload of the host stays
  user-controlled.
- specd records what it created in `.specd/integrations.json` so repair/uninstall only
  touches specd-owned entries.

### Manual snippets (air-gapped / unmanaged hosts)

`specd mcp --config <host>` prints a ready-to-paste config snippet for any supported host and
exits without starting the server. Use it when there is no host CLI, for the
snippet-only hosts (**antigravity**, **claude-desktop**), or when you prefer to merge
config yourself. Combine with `--root` to substitute your project path:

```bash
specd mcp --config cursor
specd mcp --config claude-desktop --root /path/to/your/project
```

Supported values: `antigravity`, `claude-code`, `claude-desktop`, `codex`, `cursor`, `gemini`, `vscode`.

---

### Claude Desktop

Add the following block to your Claude Desktop configuration file:
- **macOS/Linux:** `~/.config/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp", "--root", "/path/to/your/project"]
    }
  }
}
```

Replace `/path/to/your/project` with the absolute path to your specd project root, or run
`specd mcp --config claude-desktop --root /path/to/your/project` to get a pre-filled snippet.

### Cursor

Add the following to Cursor's MCP configuration (`.cursor/mcp.json` in your project, or the
global MCP settings):

```json
{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp", "--root", "/path/to/your/project"]
    }
  }
}
```

Run `specd mcp --config cursor --root /path/to/your/project` to get a pre-filled snippet.

**Note:** Cursor sends newline-delimited JSON framing. `Content-Length` auto-detection means no
specd-side change is needed if Cursor switches framing in a future version.

### VS Code

Add `specd` to the workspace MCP file at `.vscode/mcp.json` under the top-level
`servers` key:

```json
{
  "servers": {
    "specd": {
      "type": "stdio",
      "command": "specd",
      "args": ["mcp", "--root", "/path/to/your/project"]
    }
  }
}
```

Run `specd mcp --config vscode --root /path/to/your/project` to get a pre-filled snippet.

### Antigravity CLI

Merge the following into your Antigravity MCP config file. Antigravity reads from three
locations in priority order:
1. `.agents/mcp_config.json` (workspace-local, project-scoped)
2. `~/.gemini/antigravity-cli/mcp_config.json` (per-CLI global)
3. `~/.gemini/config/mcp_config.json` (shared with Antigravity IDE and 2.0)

```json
{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp", "--root", "/path/to/your/project"],
      "env": {
        "MCP_MODE": "stdio",
        "DISABLE_CONSOLE_OUTPUT": "true"
      }
    }
  }
}
```

The `MCP_MODE` and `DISABLE_CONSOLE_OUTPUT` env vars prevent Antigravity's debug output from
leaking onto stdout and corrupting the JSON-RPC stream. Replace `/path/to/your/project`
with the absolute path to your specd project root.

Run `specd mcp --config antigravity --root /path/to/your/project` to get a pre-filled snippet.

### OpenAI Codex CLI

Merge the following into your Codex config file:
- **Global:** `~/.codex/config.toml`
- **Project-scoped (trusted):** `.codex/config.toml`

```toml
[mcp_servers.specd]
command = "specd"
args = ["mcp", "--root", "/path/to/your/project"]
```

Replace `/path/to/your/project` with the absolute path to your specd project root.
Codex natively supports stdio MCP servers — no extra environment variables or transport
adapters are needed.

Run `specd mcp --config codex --root /path/to/your/project` to get a pre-filled snippet.

---

## Limitations

| Limitation | Detail |
|---|---|
| **Loopback HTTP only** | `specd mcp --http` binds loopback (`127.0.0.1`) by default and is not an authenticated remote endpoint. Do not expose it on a public interface without an authenticating proxy in front. |
| **No resources or prompts** | Only the `tools` capability is advertised. MCP resources and prompts are not yet implemented. |
| **Static tool list** | `listChanged` is `false`. The tool list is fixed for the process lifetime; a host must restart the server to pick up newly compiled commands. |
| **Serial tool calls** | Tool calls are processed one at a time. Concurrent calls from the same host are serialised by the stdio loop. |
| **Host config schema drift** | MCP support in Cursor and VS Code is evolving. Config examples in this guide target the `2024-11-05` protocol revision and may need adjustment for future host versions. |
