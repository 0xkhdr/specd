# Decisions — MCP Expansion for External Tools

<!--
ADR ledger (append-only). Use `specd decision <spec> "<text>" [--supersedes ADR-NNN]`
to append. Entries are numbered monotonically and never edited. Format:

## ADR-001 — <decision summary> · 2026-06-16
**Context:** <what forced the choice>
**Decision:** <what we chose>
**Consequences:** <trade-offs, what it rules out>
**Supersedes:** <ADR-id or —>
-->

## ADR-001 — Integrate both antigravity CLI and Codex CLI via declarative stdio config, not the HTTP adapter. T1 found antigravity supports stdio MCP servers (command/args/env in ~/.gemini/config/mcp_config.json); T2 found Codex natively supports stdio MCP servers ([mcp_servers.<name>] command/args/env in ~/.codex/config.toml). Per R3.1 the thinnest viable path is a declarative config launching the unchanged 'specd mcp' stdio server for both hosts. The opt-in HTTP/SSE adapter (R4, tasks T4-T6) is still BUILT as additive, loopback-default infrastructure for future non-stdio hosts and to satisfy the spec's parity-test acceptance, but no host config ships pointing at it; both host configs use stdio. SSE is unavailable on Codex and unneeded on antigravity. · 2026-06-16
**Context:** TODO
**Decision:** Integrate both antigravity CLI and Codex CLI via declarative stdio config, not the HTTP adapter. T1 found antigravity supports stdio MCP servers (command/args/env in ~/.gemini/config/mcp_config.json); T2 found Codex natively supports stdio MCP servers ([mcp_servers.<name>] command/args/env in ~/.codex/config.toml). Per R3.1 the thinnest viable path is a declarative config launching the unchanged 'specd mcp' stdio server for both hosts. The opt-in HTTP/SSE adapter (R4, tasks T4-T6) is still BUILT as additive, loopback-default infrastructure for future non-stdio hosts and to satisfy the spec's parity-test acceptance, but no host config ships pointing at it; both host configs use stdio. SSE is unavailable on Codex and unneeded on antigravity.
**Consequences:** TODO
**Supersedes:** —
