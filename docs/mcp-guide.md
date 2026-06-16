# Model Context Protocol (MCP) Integration Guide

`specd` ships with a native, zero-dependency Model Context Protocol (MCP) server that runs over standard input/output (`stdio`). This allows IDE agents (like Claude Code, Cursor, and Aider) to natively discover, inspect, and invoke the entire `specd` toolset.

The MCP server is a thin JSON-RPC 2.0 transport over the existing CLI handlers. It does not introduce network calls or external APIs, keeping execution fast and fully local.

---

## Launching the MCP Server

Start the server using the `specd mcp` command. It will listen on standard input and respond on standard output.

```bash
specd mcp [--root /path/to/project]
```

* **`--root <path>`**: Resolve specs against the specified directory rather than the current working directory of the process. This is useful when the IDE launches the server from a different path.

---

## Tool Mapping Contract

The server automatically maps every registered command in `specd` (except meta commands `help`, `version`, and `mcp` itself) to an MCP tool.

### Namespacing
All tools are prefixed with `specd_` to prevent collisions in the host agent's tool list (e.g. `specd_status`, `specd_next`, `specd_task`).

### Input Schema
MCP tool calls use the following standard schema for arguments:
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

* **`args`**: A JSON array of strings supplying the ordered CLI positional arguments (e.g. `[slug, id]`).
* **Flags**: Each CLI flag becomes a key-value pair in the `arguments` object. Boolean flags accept `true`/`false`. Other flags accept string values.

### Output Schema
The server redirects and captures command outputs, wrapping them in an MCP response:
```json
{
  "content": [
    {
      "type": "text",
      "text": "Task T1 complete"
    }
  ],
  "isError": false,
  "structuredContent": {
    "spec": "my-feature",
    "task": "T1",
    "status": "complete"
  }
}
```

* **`content`**: Contains the command stdout. On failure, any stderr diagnostics are appended to this block.
* **`isError`**: Evaluates to `true` if the CLI command exited with a non-zero code.
* **`structuredContent`**: If the command emitted valid JSON on stdout, it is automatically parsed and attached as a structured JSON object so the LLM does not need to parse text manually.

---

## Exposed MCP Tools

The following tools are exposed automatically by the server. Refer to the [Command Reference](./command-reference.md) for parameter details:

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

---

## IDE Configurations

### Claude Desktop
Add `specd` to your Claude Desktop configuration file (`~/.config/Claude/claude_desktop_config.json` on macOS or `%APPDATA%\Claude\claude_desktop_config.json` on Windows):

```json
{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp"]
    }
  }
}
```

### Cursor
1. Open Cursor Settings.
2. Navigate to **Features** → **MCP**.
3. Click **+ Add New MCP Server**.
4. Set:
   * **Name**: `specd`
   * **Type**: `stdio`
   * **Command**: `specd mcp --root "${workspaceRoot}"`
5. Save and restart the index.
