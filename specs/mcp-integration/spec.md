# Spec: MCP Orchestration Surface

Status: proposed — awaiting human review
Scope: expose Brain sessions and Pinky worker operations through the existing generated MCP command surface.

## 1. Outcome

Make orchestration available to Claude Code, Codex, Cursor, Gemini, VS Code, and other MCP hosts without creating hand-maintained MCP-only business logic. CLI remains canonical; MCP remains a thin transport over registered commands.

## 2. Requirements

- R5.1 Add Brain and Pinky CLI command metadata so `internal/mcp/tools.go` generates namespaced tools automatically.
- R5.2 Use existing tool names such as `specd_brain` and `specd_pinky` with subcommands in `args`; do not add an unrelated second naming convention.
- R5.3 Preserve CLI/MCP output and exit-code parity for stdio and HTTP/SSE transports.
- R5.4 Mark read-only operations with MCP read-only annotations and mutating/session-start operations as non-read-only.
- R5.5 Validate all arguments through the existing CLI parser and command handlers; no parallel JSON-schema validator.
- R5.6 Return bounded structured session, worker, decision, progress, blocker, and escalation payloads.
- R5.7 Require an explicit project root/session scope and reject traversal or unknown IDs.
- R5.8 Keep long-running orchestration out of one MCP request: `start` creates/attaches a foreground-capable session, while `step`/`status`/`watch` provide bounded interactions.
- R5.9 Never assume an MCP host can be spawned by specd. The host claims Pinky missions through tools and remains responsible for its own agent lifecycle.
- R5.10 Add server instructions describing the trust boundary, approval policy, verify requirement, and cooperative cancellation semantics.
- R5.11 Preserve current MCP protocol revision, dependency-free transport, framing, error envelopes, host registration, and deterministic tool discovery.
- R5.12 Extend compatibility docs and `doctor` checks only where orchestration introduces a testable server capability.

## 3. Proposed Commands and Generated Tools

| CLI | Generated MCP tool | Mutation |
|---|---|---|
| `specd brain start ...` | `specd_brain` | yes |
| `specd brain status ...` | `specd_brain` | no |
| `specd brain step ...` | `specd_brain` | yes |
| `specd brain pause|resume|cancel ...` | `specd_brain` | yes |
| `specd pinky claim ...` | `specd_pinky` | yes |
| `specd pinky heartbeat|progress|report|block|release ...` | `specd_pinky` | yes |

The generated command schema describes allowed subcommands/flags. If nested schemas become necessary, improve `CommandMeta` once and preserve registry/help/tool parity.

## 4. Invariants

- V1 Every MCP orchestration action has an equivalent CLI invocation and structured result.
- V2 MCP handlers contain transport logic only.
- V3 One bounded call cannot monopolize the server waiting for an agent.
- V4 Tool discovery remains deterministic and registry-driven.
- V5 MCP cannot bypass approval, evidence, lock, CAS, or path validation.
- V6 No host-specific model API enters specd.

## 5. Acceptance

- Tool discovery golden data, CLI/MCP parity, wire contract, stdio, HTTP/SSE, malformed argument, annotation, and cancellation tests pass.
- Existing tools remain byte-compatible unless a documented schema version change is approved.
- `make ci` passes.
