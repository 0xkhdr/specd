# Design — cmd-mcp-sync

## Overview
cmd-mcp-sync keeps the MCP command-mirror surface generated from `core.Commands` aligned with the optimized CLI palette. The MCP generator skips hidden/retired entries, preserves survivor flag schemas, and keeps intent tools routed to surviving Brain/Pinky entrypoints. A parity test is the durable guard against future drift.

## Architecture
The MCP tool builder iterates `core.Commands` and converts each visible survivor into a `specd_<command>` tool. Hidden, retired, and server/meta commands are filtered before schema generation. Semantic intent tools remain separate wrappers, but each translator returns a surviving command and argv shape. Tests exercise generation directly and over the server wire.

## Components and interfaces
- **Command registry (`internal/core/commands.go`)**: source of command names, `Hidden` flags, positionals, flags, and descriptions.
- **MCP tool generator (`internal/mcp/tools.go`)**: converts visible command metadata to MCP `toolDef` values and applies configured exposure filters.
- **Intent aliases (`internal/mcp/intent.go`)**: maps `brain_*` tools to deterministic CLI commands and flags.
- **Parity tests (`internal/mcp/parity_test.go`, `tools_test.go`, `intent_test.go`)**: compare command-mirror tools to CLI survivors, verify hidden exclusions, and prove aliases resolve to survivors.

## Data models
No on-disk data model changes. MCP tool descriptors keep the existing shape: `name`, `description`, `inputSchema`, and `annotations`. Survivor command schemas inherit absorbed flags from `CommandMeta`, such as `report` exposing `serve`, `watch`, `history`, and `diff` options.

## Error handling
Hidden and retired command metadata is skipped before a tool definition is created, so removed commands are method-not-found at MCP call time. Intent translators validate required arguments and return MCP invalid-params errors before dispatch. Tests fail loudly with symmetric diffs when command parity drifts.

## Verification strategy
- `go test ./internal/mcp/ -run TestHiddenExcluded` verifies hidden and retired command tools are absent.
- `go test ./internal/mcp/ -run TestIntentAliasResolve` verifies intent aliases resolve only to survivor commands.
- `go test ./internal/mcp/ -run TestCLIMCPParity` verifies CLI survivor and MCP command-mirror set equality.
- `specd check cmd-mcp-sync` validates requirements, design, tasks, DAG, evidence, sync, and traceability gates.

## Risks and open questions
Intent tools have hand-written schemas, so they can drift from command flags. The alias resolution test mitigates command-target drift, while schema golden tests force deliberate schema changes. HTTP/SSE transports generate tools on server start and share the same builder path, so parity tests cover both stdio and configured generation indirectly.
