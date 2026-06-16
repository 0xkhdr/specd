# Tasks — MCP Deep-Dive & Real-World Integration

## Wave 1
- [ ] T1 — Map the MCP server wire contract from source
  - why: every doc claim must be grounded in source line references (R1, R2, R3, R5)
  - role: investigator
  - files: internal/mcp/server.go, internal/mcp/transport.go, internal/mcp/tools.go, internal/cmd/mcp.go
  - contract: produce a file:line table of startup, framing auto-detect, route methods, tool generation, and advertised capabilities; do NOT modify any source
  - acceptance: a notes table covers initialize/ping/tools-list/tools-call plus newline and Content-Length framing with line anchors
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 3, 5

- [ ] T2 — Audit existing docs/mcp-guide.md for drift
  - why: the shipped guide predates dual framing and may contradict source (R2, R5)
  - role: investigator
  - files: docs/mcp-guide.md
  - contract: list every statement that disagrees with T1 findings; do NOT edit docs in this task
  - acceptance: a drift list enumerating stale or missing claims (framing, protocol version, limitations)
  - verify: N/A
  - depends: T1
  - requirements: 2, 5

## Wave 2
- [ ] T3 — Rewrite the host-integration chapter of docs/mcp-guide.md
  - why: developers need an accurate startup, framing, and tool-surface reference (R1, R2, R3, R5)
  - role: builder
  - files: docs/mcp-guide.md
  - contract: document startup/--root/exit-2, dual framing auto-detection, protocol version 2024-11-05, the specd_ tool naming and annotations, and the limitations table; reconcile every claim with T2 drift list
  - acceptance: guide contains the framing auto-detect, protocol-version, and limitations statements; specd check passes
  - verify: grep -q "Content-Length" docs/mcp-guide.md && grep -q "2024-11-05" docs/mcp-guide.md && grep -qi "stdio" docs/mcp-guide.md
  - depends: T2
  - requirements: 1, 2, 3, 5

- [ ] T4 — Ship copy-paste host config snippets
  - why: each target host needs a verified minimal mcpServers block (R4)
  - role: builder
  - files: docs/mcp-hosts/claude-desktop.json, docs/mcp-hosts/cursor.json, docs/mcp-hosts/vscode.json
  - contract: each file is a valid JSON mcpServers block launching `specd mcp --root <project>`; no host-specific secrets; reference them from docs/mcp-guide.md
  - acceptance: all three files parse as JSON and contain the mcp subcommand
  - verify: for f in docs/mcp-hosts/*.json; do python3 -c "import json,sys;json.load(open(sys.argv[1]))" "$f" || exit 1; done
  - depends: T3
  - requirements: 4

## Wave 3
- [ ] T5 — Add the MCP wire-contract integration smoke test
  - why: documentation must be pinned by an executable contract so it cannot drift (R6)
  - role: builder
  - files: internal/mcp/integration_test.go
  - contract: drive mcp.Serve over an in-memory pipe through initialize, tools/list, tools/call specd_status, and an unknown tool; assert protocol version, specd_status presence and annotations, structuredContent, and the -32602 path; run serially because capture swaps os.Stdout
  - acceptance: go test ./internal/mcp/... -race passes with the new test exercising all four steps
  - verify: go test ./internal/mcp/... -race -count=1
  - depends: T4
  - requirements: 6

- [ ] T6 — Final gate review of the integration spec
  - why: confirm requirements, docs, configs, and test cohere before approval (R1-R6)
  - role: verifier
  - files: docs/mcp-guide.md, docs/mcp-hosts/claude-desktop.json, internal/mcp/integration_test.go
  - contract: run the full check and the smoke test; confirm every requirement is satisfied by a doc section or test; do NOT introduce new behaviour
  - acceptance: specd check mcp-integration reports no violations and the mcp test suite is green
  - verify: ./specd check mcp-integration && go test ./internal/mcp/... -race -count=1
  - depends: T5
  - requirements: 1, 2, 3, 4, 5, 6
