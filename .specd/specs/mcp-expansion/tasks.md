# Tasks — MCP Expansion for External Tools

## Wave 1
- [x] T1 — Discover antigravity CLI MCP client capabilities ✓ complete · evidence: docs/mcp-hosts/antigravity.md records stdio=true,http=true,sse=true; configPath ~/.gemini/config/mcp_config.json; mcpServers schema (command/args/env/serverUrl/oauth); 4 dated sources incl Medium 2026-05-24. R1.3: stdio supported, no adapter required for antigravity. · 2026-06-16T23:19:42.064611943Z
  - why: integration approach must be grounded in antigravity's real transport support (R1)
  - role: investigator
  - files: docs/mcp-hosts/antigravity.md
  - contract: record from primary sources whether antigravity supports stdio/HTTP/SSE MCP servers, its config path and schema, each with a dated citation; do NOT write integration code
  - acceptance: a capability record with stdio/http/sse booleans, config schema, and sources
  - verify: N/A
  - depends: —
  - requirements: 1

- [x] T2 — Discover Codex CLI MCP capabilities ✓ complete · evidence: docs/mcp-hosts/codex.md: native MCP yes; stdio=true,http(Streamable)=true,sse=false; configPath ~/.codex/config.toml; [mcp_servers.<name>] syntax (command/args/env | url/bearer_token_env_var); 3 sources incl official developers.openai.com/codex/mcp. R2.3: stdio supported, no adapter required for Codex. · 2026-06-16T23:20:01.023957111Z
  - why: integration approach must be grounded in Codex's real MCP/tool mechanism (R2)
  - role: investigator
  - files: docs/mcp-hosts/codex.md
  - contract: record from primary sources whether Codex supports MCP servers natively or needs an adapter, its config syntax and transport, each with a dated citation; do NOT write integration code
  - acceptance: a capability record with native-MCP yes/no, config syntax, transport, and sources
  - verify: N/A
  - depends: —
  - requirements: 2

- [x] T3 — Record the integration-path decision per host ✓ complete · evidence: ADR-001 in decisions.md: both hosts → declarative stdio config (R3.1); HTTP adapter built as additive future infra but no host config points at it. Grounded in T1/T2 capability records. · 2026-06-16T23:20:19.205601079Z
  - why: the stdio-vs-HTTP choice is a decision gate that must precede any transport code (R3)
  - role: investigator
  - files: .specd/specs/mcp-expansion/decisions.md
  - contract: append an ADR selecting stdio-config or HTTP-adapter for each host based on T1/T2; do NOT begin implementation until recorded
  - acceptance: decisions.md carries an ADR naming the chosen path for antigravity and Codex
  - verify: N/A
  - depends: T1, T2
  - requirements: 3

## Wave 2
- [x] T4 — Implement the opt-in HTTP/SSE transport adapter ✓ complete · evidence: ServeHTTP in transport_http.go: POST /rpc + GET|POST /sse route via dispatchOnce→conn.handle→route (same dispatch as stdio); loopbackAddr defaults to 127.0.0.1 (R4.2); mutex serialises calls (capture swaps global stdout); --http flag in cmd/mcp.go, absent=byte-identical stdio (R4.3); stdlib-only. verify go build+vet exit 0. · 2026-06-16T23:21:33.350035539Z
  - why: hosts that cannot speak stdio need an HTTP front door onto the same dispatch (R4)
  - role: builder
  - files: internal/mcp/transport_http.go, internal/cmd/mcp.go
  - contract: add ServeHTTP routing POST /rpc and GET /sse through the existing conn.handle/route; bind loopback by default; gate behind a --http flag so the stdio path stays byte-identical when absent; stdlib-only; serialise tool calls because capture swaps os.Stdout
  - acceptance: with --http set, POST /rpc tools/list returns the buildTools set; without it, stdio behaviour is unchanged
  - verify: go build ./... && go vet ./internal/mcp/...
  - depends: T3
  - requirements: 3, 4, 7

- [x] T5 — Author declarative per-host configs ✓ complete · evidence: antigravity.config.json (mcpServers.specd, stdio command/args/env) parses via json.load exit 0; codex.config.toml ([mcp_servers.specd] stdio) parses via tomllib. Both stdio per ADR-001, no secrets, referenced from antigravity.md/codex.md. · 2026-06-16T23:22:00.999298117Z
  - why: users need copy-paste configs matching the chosen transport (R5)
  - role: builder
  - files: docs/mcp-hosts/antigravity.config.json, docs/mcp-hosts/codex.config.toml
  - contract: write a host-native MCP server config for antigravity and Codex pointing at the transport selected in T3; no secrets; reference from the per-host docs
  - acceptance: both config artifacts parse and name the specd mcp server with the chosen transport
  - verify: python3 -c "import json;json.load(open('docs/mcp-hosts/antigravity.config.json'))"
  - depends: T4
  - requirements: 5

## Wave 3
- [x] T6 — Add sandboxed transport-parity integration test ✓ complete · evidence: transport_http_test.go: TestHTTPTransportParity asserts HTTP /rpc tools/list==stdio, tools/call specd_status==stdio, /sse frame==stdio (R6.2); TestHTTPLoopbackDefault asserts bare port binds 127.0.0.1 (R4.2); uses testharness fixture, stdio ref gathered before HTTP server starts (avoids os.Stdout swap race). go test -race -count=1 exit 0. · 2026-06-16T23:23:09.799681709Z
  - why: the HTTP path must provably expose the same tools as stdio without network flakiness (R6)
  - role: builder
  - files: internal/mcp/transport_http_test.go
  - contract: use internal/testharness for a deterministic fixture; assert HTTP tools/list equals stdio tools/list and an HTTP tools/call specd_status matches the stdio result; keep race-clean
  - acceptance: go test ./internal/mcp/... -race passes including the new parity test
  - verify: go test ./internal/mcp/... -race -count=1
  - depends: T5
  - requirements: 6, 7

- [x] T7 — Final gate review of the expansion spec ✓ complete · evidence: check green + mcp suite race-clean (exit 0). Req map: R1→antigravity.md, R2→codex.md, R3→ADR-001+configs, R4→transport_http.go(ServeHTTP/--http), R5→antigravity.config.json+codex.config.toml, R6→transport_http_test.go(parity,-race), R7→serialise mutex+preserved capture panic-recovery. No behaviour changed. · 2026-06-16T23:23:25.039193963Z
  - why: confirm discovery, decision, adapter, configs, and tests cohere before approval (R1-R7)
  - role: verifier
  - files: internal/mcp/transport_http.go, docs/mcp-hosts/antigravity.md, docs/mcp-hosts/codex.md
  - contract: run the full check and the mcp test suite; confirm every requirement maps to a doc, config, or test; do NOT change behaviour
  - acceptance: specd check mcp-expansion reports no violations and the mcp suite is green
  - verify: ./specd check mcp-expansion && go test ./internal/mcp/... -race -count=1
  - depends: T6
  - requirements: 1, 2, 3, 4, 5, 6, 7
