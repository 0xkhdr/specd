# Design — MCP Deep-Dive & Real-World Integration

## Overview
This feature is documentation-and-test-only; it changes no protocol behaviour. The strategy is
to (1) read `internal/mcp/` and `internal/cmd/mcp.go` end to end, (2) capture the verified wire
contract in a new `docs/mcp-guide.md` host-integration section, (3) ship copy-paste host configs,
and (4) lock the contract with a Go integration smoke test in `internal/mcp/`. We document what
the code actually does — dual framing, `--root` scoping, auto-generated tools — rather than the
idealised behaviour, because the existing `docs/mcp-guide.md` predates the `Content-Length`
framing path and must be reconciled with the source.

## Architecture
```
MCP host (Claude Desktop / Cursor / VS Code)
        │  spawns: specd mcp --root <project>
        ▼
 cmd.RunMCP ──chdir(root)──► mcp.Serve(stdin, stdout, cmd.Dispatch)
        │
        ▼
 conn (transport.go): auto-detect framing  ── newline JSON | Content-Length headers
        │
        ▼
 route(): initialize | ping | tools/list | tools/call
        │                                     │
        │ buildTools() from core.Commands     │ callTool → buildArgv → cli.ParseArgs
        ▼                                     ▼
 tool defs (specd_*)              capture(){ Dispatch(cmd,args) } → toolResult
```
The documentation mirrors this flow file-by-file with `file:line` anchors so a reader can verify
each claim against source. No component is added to the binary except the test file.

## Components and interfaces
- **`docs/mcp-guide.md` (edited):** host-integration chapter — startup, framing, tool surface,
  per-host config blocks, limitations table. Source of truth for Requirements 1–5.
- **`docs/mcp-hosts/` config snippets (new):** `claude-desktop.json`, `cursor.json`,
  `vscode.json` — minimal, copy-paste `mcpServers` blocks parameterised on project path and
  binary path. Backs Requirement 4.
- **`internal/mcp/integration_test.go` (new):** drives `mcp.Serve` over an in-memory pipe with a
  scripted request sequence (`initialize` → `tools/list` → `tools/call specd_status` → unknown
  tool), asserting the responses. Backs Requirement 6. Reuses `internal/testharness` for a
  deterministic spec fixture.
- **No source changes** to `server.go`, `tools.go`, `transport.go`, `cmd/mcp.go`. Contract is
  observed, not modified.

## Data models
- **`initialize` result:** `{protocolVersion:"2024-11-05", capabilities:{tools:{listChanged:false}}, serverInfo:{name:"specd", version:<core.Version>}}`.
- **tool def:** `{name:"specd_<cmd>", description, inputSchema:{type:"object", properties:{args:array<string>, <flag>:<typed>}}, annotations:{readOnlyHint, destructiveHint?}}`.
- **tools/call result:** `{content:[{type:"text", text:<stdout|stderr>}], isError:<code!=0>, structuredContent?:<parsed stdout JSON>}`.
- **host config:** standard MCP `{"mcpServers":{"specd":{"command":"specd","args":["mcp","--root","<path>"]}}}`.

## Error handling
- A malformed JSON request yields a `-32700` parse error; an invalid JSON-RPC envelope yields
  `-32600`; an unknown method yields `-32601`; bad params or unknown tool yields `-32602`. The
  loop never terminates on a bad request (`server.go` `handle`).
- A handler panic is recovered in `capture()` and surfaced as `isError:true` with exit-gate
  semantics; the smoke test asserts the unknown-tool path returns `-32602` without tearing down
  the connection.
- `--root` that cannot be entered returns usage code 2 before the loop starts (`cmd/mcp.go`).
- Documentation errors (claims that disagree with source) are caught by the smoke test, which is
  the executable half of the spec.

## Verification strategy
- **Unit/integration (Req 6):** `internal/mcp/integration_test.go` run under `go test ./internal/mcp/... -race`. Asserts protocol version, tool presence + annotations, `structuredContent`, and the invalid-params path.
- **Doc-lint (Req 1–5):** `specd check mcp-integration` plus a grep-based assertion that
  `docs/mcp-guide.md` contains the framing, protocol-version, and limitations statements.
- **System (Req 4):** a manual-equivalent scripted check that pipes an `initialize` line into
  `specd mcp` and greps the response for `2024-11-05`.
- Requirement→test map: R1→cmd/mcp test + doc grep; R2→transport framing test; R3→tools/list
  assertion; R4→config-file existence test; R5→doc grep; R6→integration_test.go.

## Risks and open questions
- The shipped `docs/mcp-guide.md` may already contain claims that contradict the dual-framing
  reality; reconciliation, not greenfield writing, is the real cost. Mitigation: diff doc claims
  against `transport.go` line by line.
- `capture()` swaps process-global `os.Stdout`; the integration test must run serially (no
  `t.Parallel()`) to avoid clobbering. Documented as a test constraint.
- Host config schemas drift across client versions (Cursor/VS Code MCP support is young). We pin
  to the `2024-11-05` revision and note configs are host-version-sensitive.
