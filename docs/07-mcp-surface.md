# MCP & Handshake Surface

This document describes the Model Context Protocol (MCP) server, the handshake bootstrap procedure, and the safety policy limits enforced over exposed tools.

---

## 1. Native stdio JSON-RPC Server

The `specd` binary contains a built-in stdio-based MCP server. It listens for JSON-RPC 2.0 packets over standard input and output streams.

To maintain portability and compile to a single zero-dependency binary, the MCP server is written using **only the Go standard library**.

*Origin:* Ported from [internal/mcp/](file:///var/www/html/rai/up/specd/reference/internal/mcp/) and [mcp.go](file:///var/www/html/rai/up/specd/reference/internal/cmd/mcp.go).

---

## 2. Core Lifecycle Tools

The MCP server exposes a calibrated set of tools mapping to core CLI verbs:

| MCP Tool Name | Target Action | Description |
| :--- | :--- | :--- |
| `specd_check` | `specd check` | Runs the validation gates suite. |
| `specd_next` | `specd next` | Fetches the runnable task frontier. |
| `specd_verify` | `specd verify` | Triggers a sandboxed verification run. |
| `specd_task` | `specd task` | Submits task completion status. |
| `specd_status` | `specd status` | Fetches the current spec status projection. |
| `specd_context` | `specd context` | Retrieves the task's context manifest. |
| `specd_handshake` | `specd handshake`| Performs the bootstrap version handshake. |

### CLI↔MCP Parity Invariant
Every tool is backed by `TestMCPParity` which asserts that the JSON structure returned by an MCP call is byte-equal to the JSON output of the corresponding CLI command when run with identical inputs.

### Redundancy Cuts
The raw passthrough tools `specd_brain` and `specd_pinky` have been **CUT** from the surface. They provided no additional authority beyond intent-level commands.

---

## 3. Handshake & Policy Digest

When an agent host bootstraps, it runs the handshake protocol:

```bash
specd handshake bootstrap --json
```

This returns a `HandshakeBootstrap` payload containing the server's protocol versions and the effective **Policy Digest** (a hash of the steering files, configuration invariants, and allowed tools).

---

## 4. Per-Spec Tool Access Control

A spec's `manifest.json` can declare tool restrictions under `tools`:

```json
{
  "tools": {
    "required": ["specd_check", "specd_verify"],
    "optional": ["specd_status"],
    "forbidden": ["brain_orchestrate"]
  }
}
```

*   **Server-Side Block:** If a tool is listed under `forbidden`, the MCP server rejects calls to it immediately with an authorization error.
*   **Fail-Safe Enforcement:** If `manifest.json` is missing or corrupt, `specd` defaults to a **fail-closed, empty policy** (allowing only basic status and check tools).

---

## 5. Orchestration Tools (Opt-in)

If orchestration is enabled in the configuration (`orchestration.enabled: true`), the MCP server registers three intent-level tools:

1.  `brain_orchestrate`: Spawns a Brain controller session.
2.  `brain_status`: Inspects active worker leases and task claims.
3.  `brain_approve`: Authorizes a phase advance.

These tools are not registered when orchestration is disabled.
