# Design — Regression: MCP Server + Transport (stdio/HTTP-SSE, tools, host compat)

## Overview
Lock the agent-facing MCP contract: protocol handshake, tool-schema goldens, stdio/HTTP
parity, and host-config validity. Existing tests (server_test.go, tools_test.go,
transport_test.go, transport_http_test.go, guards_test.go, integration_test.go) form the
base; gaps close so every tool has a schema golden and both transports run the same matrix.

## Architecture
```
server.go ── tools.go (≈40 tools, one per CLI cmd)
   │                │
   ▼                ▼
transport.go (stdio) ── transport_http.go (HTTP/SSE)
   │                          │
   └── same tool matrix ──────┘
hosts.go + embed_hosts/{claude-desktop,cursor,vscode}.json, codex.toml, antigravity.json
guards: precondition checks before mutating ops
```

## Components and interfaces
- **server.go** — MCP lifecycle/handshake. Contract: compliant init + capability advertise.
- **tools.go** — tool registry. Contract: one tool per CLI cmd; stable input JSON-Schema.
- **transport.go / transport_http.go** — stdio + HTTP/SSE. Contract: behavioral parity.
- **hosts.go + embed_hosts/** — config snippet generation. Contract: native-format valid.
- **guards** — preconditions. Contract: actionable block, no partial mutation.

## Data models
Tool input schemas (JSON-Schema) — golden-locked. Host configs — JSON/TOML, parse-validated.
MCP messages — JSON-RPC envelopes; SSE framing for HTTP.

## Error handling
Invalid tool args -> structured MCP error. Malformed HTTP -> HTTP 4xx + MCP error. Guard
violation -> actionable message, operation blocked. Crash is never an acceptable response.

## Verification strategy
- Protocol: handshake + tools/list + tools/call happy & error paths (R1).
- Schema golden: snapshot each tool's input schema; diff fails on drift (R2.3).
- Parity: run a tool matrix over stdio and HTTP/SSE; assert equivalent results (R3).
- Host configs: parse each embedded config in its native format; validate generated snippets (R4).

## Risks and open questions
- HTTP/SSE parity tests need an ephemeral server + client; keep ports ephemeral to avoid CI
  flakes. Tool-CLI parity (R2.2) risks duplication — share a fixture invoking both paths.
  Open: should tool schema changes require a CHANGELOG entry as well as a failing golden? Yes,
  recommend recording in decisions.
