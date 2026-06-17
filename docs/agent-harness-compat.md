# Agent-Harness Compatibility Matrix

> Source spec: `regression-agent-harness-value` (R3), built on
> `regression-mcp-transport`. Cells are verified by
> `internal/mcp` tests (`go test ./internal/mcp/ -run 'Host|Transport'`).
> Honest by construction: `TestHostCompatibilityMatrix` fails if this set drifts
> from the host registry in `internal/mcp/hosts.go`.

## Transports

| Transport   | Status    | Notes |
|-------------|-----------|-------|
| stdio       | supported | Default. JSON-RPC 2.0 over stdin/stdout; runs until EOF. Auto-detects newline vs Content-Length framing. |
| HTTP `/rpc` | supported | Opt-in (`--http`). Byte-identical dispatch to stdio (`TestHTTPTransportParity`). Binds loopback by default. |
| HTTP `/sse` | supported | Opt-in. Same dispatch, response wrapped as one SSE `data:` frame. |

## Hosts × transport

All shipped host config snippets (`specd mcp --config <host>`) target **stdio**.
HTTP/SSE is for hosts that cannot spawn a stdio child (browser/remote agents) and
is not tied to a named host snippet.

| Host           | stdio (config snippet) | HTTP/SSE | Notes |
|----------------|------------------------|----------|-------|
| claude-desktop | supported              | n/a      | `claude_desktop_config.json`. |
| cursor         | supported              | n/a      | `.cursor/mcp.json` or global. |
| vscode         | supported              | n/a      | `mcp.servers` key. Some VS Code extensions use Content-Length framing — handled by auto-detect. |
| antigravity    | supported              | n/a      | `.agents/mcp_config.json` workspace-local or per-CLI global. |
| codex          | supported              | n/a      | `~/.codex/config.toml` (TOML, not JSON). |
| browser/remote | unsupported (stdio)    | supported | No stdio child possible; use `specd mcp --http`. No prebuilt snippet — point the host at the loopback endpoint. |

## Known limitations (recorded, not implied away)

- No host snippet ships for HTTP/SSE; integrators wire the loopback endpoint
  manually. R3 records this explicitly rather than implying universal one-command
  setup.
- HTTP defaults to loopback (`127.0.0.1:8765`); exposing it externally requires an
  explicit address and is the integrator's security decision — spec contents stay
  on-host by default.
- Each cell above is asserted by a working `tools/call` (stdio) or transport-parity
  test (HTTP). Cells not exercised by a test are marked `n/a`, never `supported`.
