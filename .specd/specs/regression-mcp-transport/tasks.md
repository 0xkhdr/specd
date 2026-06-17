# Tasks — Regression: MCP Server + Transport (stdio/HTTP-SSE, tools, host compat)

## Wave 1
- [x] T1 — Inventory MCP tools, transports, and host configs vs coverage ✓ complete · evidence: Investigator (verify N/A). INVENTORY: tools.go generates 24 MCP tools (27 core.Commands − 3 meta: help/version/mcp) one-per-CLI-cmd via buildTools; input schema = {args:array, +typed flag props}; annotations readOnlyHint (11 read-only) + destructiveHint (uninstall/update). TRANSPORTS: stdio (transport.go, newline + Content-Length framing autodetect) and HTTP/SSE (transport_http.go: POST /rpc → JSON-RPC 200, GET|POST /sse → one `data: ...\\n\\n` frame, loopback default, mutex-serialised dispatch). HOSTS: 5 embedded — claude-desktop.json, cursor.json, vscode.json, antigravity.json (JSON), codex.toml (TOML); HostConfig() does /path/to/your/project→root substitution, returns ok=false for unknown. COVERAGE GAPS: (R2.3) NO per-tool golden schema snapshot — tools_test only checks count/structure → T2. (R2.2) NO shared-fixture CLI↔MCP result parity → T5. (R3.3) malformed-HTTP→400 path UNTESTED; SSE framing only implicitly unwrapped → T3. (R4.1/R4.3) NO test parses each embedded host config in native JSON/TOML; (R4.2) guard actionable-block message untested (guards_test.go is actually TestStdlibOnly, misnamed) → T4. Covered today: handshake/tools-list/tools-call happy+error, HTTP↔stdio parity for status+list, loopback default. · 2026-06-17T17:38:42.84086287Z
  - why: freeze the agent-facing contract only after fully listing it (R1-R4)
  - role: investigator
  - files: internal/mcp/tools.go, internal/mcp/server.go, internal/mcp/embed_hosts
  - contract: list every tool + input schema, each transport's covered tool set, each host config; mark gaps; do NOT edit
  - acceptance: tables {tool -> schema-golden?, CLI-parity?} and {host -> config-valid test?}
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 3, 4

## Wave 2
- [x] T2 — Protocol compliance + tool-schema golden tests ✓ complete · evidence: go test ./internal/mcp/ -run 'Server|Tools|Schema' exit 0 (verified, 708ms). Added TestToolSchemaGolden (R2.3: snapshots 24 tools' name→inputSchema to testdata/tool_schemas.golden.json, -update regen, diff fails on drift), TestToolSchemaValidity (R2.1: every tool object-typed, args array-of-string, flag props string|boolean|array) in tools_test.go; added invalid-arg structured-error subtest to server_test.go TestToolsCall (R1.3: args:"not-an-array" → -32602, server answers next ping). Handshake/capabilities (R1.1) + tools/list validity (R1.2/R2.1) already covered by existing Server/Tools tests, now joined by golden lock. · 2026-06-17T17:40:38.035502098Z
  - why: R1, R2 — compliant handshake/list/call and drift-proof schemas
  - role: builder
  - files: internal/mcp/server_test.go, internal/mcp/tools_test.go
  - contract: test handshake/capabilities, tools/list schema validity, invalid-arg structured error; snapshot each tool input schema as golden
  - acceptance: R1.1-R1.3 and R2.1, R2.3 pass; schema drift fails a test
  - verify: go test ./internal/mcp/ -run 'Server|Tools|Schema'
  - depends: T1
  - requirements: 1, 2

- [x] T3 — Transport parity stdio vs HTTP/SSE ✓ complete · evidence: go test ./internal/mcp/ -run 'Transport|Integration' -race exit 0 (verified, 3300ms, no flake). Added to transport_http_test.go: TestHTTPTransportMatrix (R3.1: status/waves/check tools byte-equal stdio vs HTTP /rpc via reflect.DeepEqual, stdio refs gathered before server start to avoid os.Stdout race), TestHTTPMalformedRequest (R3.3: GET /rpc → 405, garbage JSON body → 200 with JSON-RPC -32700 parse-error envelope), TestSSEFraming (R3.2: Content-Type text/event-stream, body data: prefix + \\n\\n terminator, unwrapped payload valid JSON-RPC result). All use ephemeral freePort() — no fixed-port flake. · 2026-06-17T17:41:45.325054794Z
  - why: R3 — transport must be an operational, not behavioral, choice
  - role: builder
  - files: internal/mcp/transport_test.go, internal/mcp/transport_http_test.go, internal/mcp/integration_test.go
  - contract: run one tool matrix over both transports; assert equivalent results, SSE framing, malformed-HTTP error; use ephemeral ports
  - acceptance: R3.1-R3.3 pass; no fixed-port flake
  - verify: go test ./internal/mcp/ -run 'Transport|Integration' -race
  - depends: T1
  - requirements: 3

- [x] T4 — Host-config validity + guard tests ✓ complete · evidence: go test ./internal/mcp/ -run 'Host|Guard' exit 0 (verified, 1331ms). Added to guards_test.go: TestHostConfigsValidNativeFormat (R4.1/R4.3: all 5 hosts emit non-empty snippet, placeholder substituted with root, JSON hosts parse via encoding/json, codex parses via assertParseableTOML stdlib-only structural check — table headers + key=value + specd server table present), TestHostConfigGuardUnknown (R4.2: unknown host → ok=false + empty dest/content, no panic/partial; HostNames() returns sorted non-empty set caller uses for actionable "Available:" message, HostNames()[0] resolvable). TOML validated structurally because specd is zero-dependency (TestStdlibOnly) — no third-party TOML parser available. · 2026-06-17T17:42:56.258917103Z
  - why: R4 — copy-paste-reliable host setup and safe guards
  - role: builder
  - files: internal/mcp/hosts.go, internal/mcp/guards_test.go
  - contract: parse each embedded host config in native format (JSON/TOML); validate generated snippets; assert guard blocks with actionable message
  - acceptance: R4.1-R4.3 pass for all five hosts
  - verify: go test ./internal/mcp/ -run 'Host|Guard'
  - depends: T1
  - requirements: 4

- [x] T5 — CLI↔MCP semantic parity tests ✓ complete · evidence: go test ./internal/mcp/ -run Parity exit 0 (verified, 634ms). Added TestCLIMCPParity to integration_test.go (package mcp_test — see ADR-001, internal package can't import cmd due to cycle). Shared fixture: captureCLI() runs cmd.Dispatch under SPECD_JSON + --json capturing stdout; mcpStructured() drives a tools/call and extracts structuredContent. Asserts reflect.DeepEqual for representative read-only set specd_status/waves/check/next over seeded 'parity' spec — both re-enter cmd.Dispatch so divergence is a real bug. CLI ref captured before MCP drive (shared os.Stdout swap). · 2026-06-17T17:45:32.607963439Z
  - why: R2.2 — a tool must match its CLI command's result
  - role: builder
  - files: internal/mcp/tools_test.go
  - contract: for a representative set of tools, invoke CLI and MCP via a shared fixture; assert equivalent results
  - acceptance: R2.2 passes for the chosen tool set; divergences flagged as bugs
  - verify: go test ./internal/mcp/ -run Parity
  - depends: T2
  - requirements: 2

## Wave 3
- [x] T6 — Review MCP regression for protocol gaps and flake ✓ complete · evidence: go test ./internal/mcp/... -race -count=2 exit 0 (verified, 2164ms, no flake). REVIEW T2-T5 vs acceptance — all met: (1) every tool schema golden-LOCKED via TestToolSchemaGolden (24-tool map vs testdata/tool_schemas.golden.json, drift fails, -update regen). (2) Both transports — stdio (Serve) + HTTP/SSE (TestHTTPTransportParity+Matrix byte-equal, TestSSEFraming data:/\\n\\n+text/event-stream, TestHTTPMalformedRequest 405+-32700). (3) All 5 hosts — TestHostConfigsValidNativeFormat (HostNames==5, JSON parse x4 + structural TOML codex) + TestHostConfigGuardUnknown (R4.2). (4) Error paths — invalid-arg→-32602+survives, parse envelope. No fixed ports (freePort ephemeral). FINDINGS (no blockers): low: freePort TOCTOU window (waitReady mitigates, pre-existing); low: captureCLI ignores exit code (parity proven on success paths only, set is read-only); low: HTTP subtests leak ServeHTTP goroutine (harmless in test proc). No critical/high/medium. Confidence high. · 2026-06-17T17:46:28.479215705Z
  - why: a non-compliant or flaky MCP server destroys agent-harness value
  - role: reviewer
  - files: internal/mcp
  - contract: review T2-T5 for missing error paths, fixed ports, schema goldens not enforced, untested hosts; flag only
  - acceptance: all five hosts + both transports covered; every tool schema golden-locked
  - verify: go test ./internal/mcp/... -race -count=2
  - depends: T2, T3, T4, T5
  - requirements: 1, 2, 3, 4
