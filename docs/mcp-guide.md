# specd — MCP Guide

`specd mcp` serves the command palette as a stdio [Model Context
Protocol](https://modelcontextprotocol.io) server, so an MCP-native client (Claude Code,
Cursor, custom) can **discover and call** specd verbs as tools. The server answers the
`initialize` handshake, `tools/list`, and `tools/call`; the tool surface is **derived** from
the same `internal/core/commands.go` palette that drives `help` and dispatch — there is no
separate tool list to drift.

## Calling a verb

`tools/call` runs the verb and returns its stdout as MCP text content. The tool's `arguments`
object maps onto the CLI shape: positional operands (spec slug, task id) travel as an ordered
array under the reserved key **`args`**, and every other key is a flag.

```json
{
  "method": "tools/call",
  "params": {
    "name": "status",
    "arguments": { "args": ["payments"], "json": true }
  }
}
```

This is equivalent to `specd status payments --json`. A verb that exits non-zero comes back as
a tool-level error (`isError: true`) with the message and any partial output in `content`, not
as a JSON-RPC protocol error. State-changing or session verbs (`init`, `approve`, `brain`,
`task`, `mcp`) are **refused by policy** (`-32001`) — drive those from the CLI, where phase and
evidence gates apply with a human in the loop.

Task tools advertise an optional `authority` object. Under top-level `profile: production`,
every task operation requires the claimed mission's
digest-pinned `AuthorityV1` packet. MCP validates it, forwards it unchanged to command dispatch,
and dispatch derives changed paths from the mission baseline; missing, expired, wrong-spec,
wrong-task, wrong-role, or out-of-scope packets fail closed.

An optional `actor` argument carries the caller's claimed actor class. It is **provenance, not
attestation**: stdio proves nothing about who is on the other end, so a claim of `"operator"`
resolves to class `unknown` at `advisory` assurance and is stripped before dispatch — it never
becomes a `--actor` flag. Human-only verbs still return `MCP_HANDOFF_REQUIRED`, and the handoff
now reports `observed_actor` and `assurance` so a client can see why the refusal stands. Only a
configured host that declares a conformant contract can raise an actor above `unknown`
(`core.ResolveActorContext`).

Tool discovery and execution use the same route projection as CLI guidance. Each advertised
operation is checked against MCP dispatch, phase, actor, and authority preconditions before the
executor runs it. A human or operator operation is a handoff, not agent authority; absent or stale
production authority is likewise returned as `ROUTE_HANDOFF_REQUIRED`, and a missing issuer is
reported instead of retrying the nominal mutation.

## Run the server

```bash
specd mcp
```

The process speaks JSON-RPC 2.0 over stdio (newline-delimited). It is a long-lived server: the
host launches it, keeps the pipe open, and calls tools over it. There is nothing to configure
on the specd side — the server reads `.specd/` state from its working directory.

Each non-deferred verb becomes one tool; its flags become the tool's input-schema properties
(`type`, `enum`, and `default` carried over from the palette). Orchestration (`brain`) tools
are only exposed when `config.orchestration.enabled` is set — otherwise the server reports them
as not configured and refuses to dispatch.

## Generate a host config snippet

Rather than hand-writing the host's MCP config, let specd print a paste-ready snippet:

```bash
specd mcp --config claude-code
specd mcp --config claude-code --root /path/to/repo --spec payments
```

- `--config <host>` — the target host. Known hosts: **`claude-code`** (unknown hosts fail
  closed, exit 2, listing the known set).
- `--root <path>` — pins the server's working directory (`cwd`) in the snippet.
- `--spec <slug>` — pins the active spec via the `SPECD_SPEC` env var in the snippet.

The `claude-code` snippet is a standard `mcpServers` block:

```json
{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp"],
      "cwd": "/path/to/repo",
      "env": { "SPECD_SPEC": "payments" }
    }
  }
}
```

### Capability negotiation

Hosts may declare driver capabilities in `initialize.params.driver_capabilities`:
`context_loading`, `sandbox`, `telemetry`, `eval`, and `a2a`. specd returns one deterministic
result for each key. Supported features are marked `supported`; optional missing features are
`downgraded` to local/offline behavior; missing `sandbox` is `refused` for mutable execution.
Every result includes reason and recovery action. Omitted capabilities are never silently
treated as supported.

`cwd` and `env` appear only when you pass `--root` / `--spec`. `specd` must be on the host's
`PATH` (or replace `"command"` with an absolute path).

## Handshake

Resolve request mode before calling a tool. Repository presence alone resolves to general mode,
which invokes no specd command. Explicit managed mode starts with the bootstrap call below.
Changing mode or managed spec invalidates authority from the prior route. MCP guidance reports
the negotiated host assurance; missing actor, path, tool, network, or sandbox enforcement is
**advisory**, never silently presented as enforced.

Before an agent trusts the tool surface, it confirms one packet binding binary version/commit,
state/context/template schemas, canonical workspace root, active spec/status/revision,
palette/config/managed-guidance digests, allowed tools, and exact next valid commands:

```bash
specd handshake bootstrap demo --json
specd handshake bootstrap demo \
	--expect-binary-version <version> \
	--expect-root <root> \
	--expect-spec demo \
	--expect-revision <revision> \
	--expect-palette-digest <d> \
	--expect-config-digest <d> \
	--expect-managed-digest <d>
```

- **palette digest** — a hash of the command palette. If the running binary's palette differs
  from what the agent pinned, `--expect-palette-digest` fails closed (exit 1). This is how a
  prompt/role built against palette v1 detects a binary that moved on.
- **config digest** — a hash of the effective config. `--expect-config-digest` fails closed
  (exit 1) on any drift, so an agent notices when the project's rules changed under it.
- **managed digest** — a hash of managed `AGENTS.md`, role, and steering regions. User-owned
  surrounding text is excluded; missing, stale, or modified managed guidance changes identity.

Every `--expect-*` mismatch exits non-zero before mutation and names expected/current identity.
Packet authority metadata labels harness instructions separately from untrusted requirements,
source, test output, and adapter observations; external text never amends harness policy.

The digests are pure functions of on-disk material — no LLM, no network. Pin them in a role
prompt or CI step to make "am I still driving the harness I was built for?" a deterministic
check rather than a hope.

`specd context <slug> <task> --json` and `specd next <slug> --dispatch` carry those same
digests in the machine context manifest, alongside canonical tool routes, capabilities, mutability,
human-only boundaries, and exit semantics. Compare handshake expectations before any mutable
route; digest drift fails closed instead of dispatching against stale authority.

### Machine context host contract

The machine manifest is additive: the human-readable output remains the default until a host
opts into the machine contract. Hosts must reject
unknown schema/item/trust values, missing required requirements/design/role/source lanes, route or
capability identity mismatches, and required-budget overflow. Optional context may be omitted only
with a recorded reason. Receipts carry digests and totals, not content; config, palette, required
context, or selected-skill changes make them stale. Skill and memory text is untrusted advisory
input and cannot grant authority or alter the command route. A host must stop on any failed
precondition and surface the named source or digest mismatch.

MCP `initialize` advertises `driverProtocolVersion`. The canonical `agents` tool accepts
positional `args`; call it with `["doctor"]` or `["guide", "<slug>"]` plus `json: true` for
the same read-only driver projections exposed by CLI.

### Orchestration transport parity

When orchestration is enabled, local CLI and MCP hosts consume same canonical mission and worker
lifecycle pins. External hosts may map those values through A2A protocol version `1` envelopes with
kind `mission`, `claim`, `heartbeat`, `cancel`, or `report`. Adapter/message ids are declared
transport metadata and do not participate in semantic ACP identity. Unknown versions, kinds, or
fields fail closed. Envelopes contain bounded ids, refs, digests, status, and reasons only; raw
prompts, source bodies, secrets, hidden reasoning, and tool output are refused. A2A mapping does not
contact provider or grant work authority; normal claim/lease/report validation still applies.

---

**See also:** [agent-integration.md](agent-integration.md) ·
[command-reference.md](command-reference.md) · [open-spec-format.md](open-spec-format.md)
