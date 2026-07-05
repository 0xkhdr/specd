# specd — MCP Guide

`specd mcp` serves the whole command palette as a stdio [Model Context
Protocol](https://modelcontextprotocol.io) server, so an MCP-native client (Claude Code,
Cursor, custom) can call specd verbs as tools. The tool surface is **derived** from the same
`internal/core/commands.go` palette that drives `help` and dispatch — there is no separate
tool list to drift.

## Run the server

```bash
specd mcp
```

The process speaks JSON-RPC 2.0 over stdio (newline-delimited). It is a long-lived server: the
host launches it, keeps the pipe open, and calls tools over it. There is nothing to configure
on the specd side — the server reads `.specd/` state from its working directory.

Each non-deferred verb becomes one tool; its flags become the tool's input-schema properties
(`type`, `enum`, and `default` carried over from the palette). Orchestration (`brain`) tools
are only exposed when `config.orchestration.enabled` is set — otherwise the server reports them
as not configured and refuses to dispatch.

## Generate a host config snippet

Rather than hand-writing the host's MCP config, let specd print a paste-ready snippet:

```bash
specd mcp --config claude-code
specd mcp --config claude-code --root /path/to/repo --spec payments
```

- `--config <host>` — the target host. Known hosts: **`claude-code`** (unknown hosts fail
  closed, exit 2, listing the known set).
- `--root <path>` — pins the server's working directory (`cwd`) in the snippet.
- `--spec <slug>` — pins the active spec via the `SPECD_SPEC` env var in the snippet.

The `claude-code` snippet is a standard `mcpServers` block:

```json
{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp"],
      "cwd": "/path/to/repo",
      "env": { "SPECD_SPEC": "payments" }
    }
  }
}
```

`cwd` and `env` appear only when you pass `--root` / `--spec`. `specd` must be on the host's
`PATH` (or replace `"command"` with an absolute path).

## Handshake

Before an agent trusts the tool surface, it can confirm it is talking to the binary and config
it expects. `handshake bootstrap` emits the bootstrap material plus two digests:

```bash
specd handshake bootstrap --json
specd handshake bootstrap \
  --expect-palette-digest <d> \
  --expect-config-digest <d>
```

- **palette digest** — a hash of the command palette. If the running binary's palette differs
  from what the agent pinned, `--expect-palette-digest` fails closed (exit 1). This is how a
  prompt/role built against palette v1 detects a binary that moved on.
- **config digest** — a hash of the effective config. `--expect-config-digest` fails closed
  (exit 1) on any drift, so an agent notices when the project's rules changed under it.

The digests are pure functions of on-disk material — no LLM, no network. Pin them in a role
prompt or CI step to make "am I still driving the harness I was built for?" a deterministic
check rather than a hope.

---

**See also:** [agent-integration.md](agent-integration.md) ·
[command-reference.md](command-reference.md) · [open-spec-format.md](open-spec-format.md)
