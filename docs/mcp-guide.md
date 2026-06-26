# Model Context Protocol (MCP) Integration Guide

`specd` ships with a native, zero-dependency Model Context Protocol (MCP) server that runs over
standard input/output (`stdio`) or optional loopback HTTP/SSE. This allows MCP hosts (Claude
Desktop, Cursor, VS Code, and others) to discover, inspect, and invoke the entire `specd`
toolset without provider SDKs or external APIs.

The MCP server is a thin JSON-RPC 2.0 transport over the existing CLI handlers; it adds no
business logic, performs no LLM calls, and introduces no new binary dependencies.

---

## Startup & Invocation Contract

Start the server with the `specd mcp` command:

```bash
specd mcp [--root /path/to/project] [--http [<addr>]] [--config <host>]
```

- **`specd mcp`** starts a JSON-RPC 2.0 stdio server and runs until stdin closes (EOF). No
  network port is bound; all communication is byte-for-byte on `stdin`/`stdout`.
  (`internal/mcp/server.go`, `Serve` function)
- **`--root <dir>`** scopes every tool call to the named project directory via a one-time
  `chdir` before the loop starts. Without it, the working directory of the spawning process
  is used as the spec root — the same fallback the CLI uses.
  (`internal/cmd/mcp.go:23-28`)
- **`--http [<addr>]`** opts into the HTTP/SSE transport instead of stdio. Bare `--http`
  (no value) defaults to `127.0.0.1:8765`. The server runs until it stops; exits `0` on clean
  shutdown, `1` on bind error. See [HTTP/SSE Transport](#httpsse-transport) below.
  (`internal/mcp/transport_http.go`, `ServeHTTP`)
- **`--config <host>`** prints a ready-to-paste config snippet for the named host and exits
  immediately without starting any server. (`internal/cmd/mcp.go:25-27`)
- If the `--root` path cannot be entered, the server exits with **usage code 2** before any
  protocol bytes are emitted. The error message goes to stderr.
  (`internal/cmd/mcp.go:27`, `core.ExitUsage = 2`)
- **Only protocol bytes appear on stdout.** All diagnostic messages (errors, warnings) go to
  stderr, which MCP hosts typically capture separately.

---

## Wire Framing

`specd mcp` speaks JSON-RPC 2.0 and **auto-detects the framing style** from the first
non-whitespace byte received — a single connection-lifetime decision:

| First byte | Framing selected | Description |
|---|---|---|
| `C` | **Content-Length** (LSP-style) | `Content-Length: N\r\n\r\n<body>` |
| anything else | **Newline-delimited** | One JSON object per line |

(`internal/mcp/transport.go:73-98`, `readMessage`)

### Newline-delimited JSON (default)
Most MCP SDK clients and CLI tools send newline-delimited JSON:
```
{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}\n
```
Each response is likewise one JSON object followed by `\n`.

### Content-Length header framing (LSP)
Hosts that speak the Language Server Protocol wire format (some VS Code extensions) send:
```
Content-Length: 42\r\n
\r\n
{"jsonrpc":"2.0","id":1,"method":"initialize",...}
```
The `Content-Length` header value is the byte length of the JSON body. Responses use the same
format for the connection's lifetime. **No specd-side configuration is needed** — framing is
detected automatically.

### Protocol version
The `initialize` result always advertises protocol version **`2024-11-05`**:
```json
{
  "protocolVersion": "2024-11-05",
  "capabilities": { "tools": { "listChanged": false } },
  "serverInfo":   { "name": "specd", "version": "<core.Version>" }
}
```
(`internal/mcp/server.go:84-90`)

### Error handling
A malformed JSON request or invalid framing never terminates the loop; the server replies with
a JSON-RPC error and keeps reading:

| Condition | Error code |
|---|---|
| Malformed JSON body | `-32700` (parse error) |
| Valid JSON but invalid JSON-RPC envelope | `-32600` (invalid request) |
| Unknown method | `-32601` (method not found) |
| Bad params or unknown tool name | `-32602` (invalid params) |

(`internal/mcp/transport.go`, `server.go:handle`)

---

## HTTP/SSE Transport

For MCP hosts that cannot spawn stdio processes (browser-based tools, remote agents, hosts
that require an HTTP endpoint), `specd mcp --http` starts an HTTP server exposing the same
JSON-RPC 2.0 dispatch as the stdio path.

### When to use `--http` vs stdio

Use stdio (default) for all local desktop hosts (Claude Desktop, Cursor, VS Code). Use
`--http` when the host requires a network endpoint — a browser extension, a remote agent, or
a host process that cannot manage child process I/O directly.

### Routes

| Route | Method | Behavior |
|---|---|---|
| `/rpc` | `POST` | Single JSON-RPC 2.0 request body → JSON-RPC response. HTTP 200 even on JSON-RPC errors (errors are in the response body envelope). |
| `/sse` | `GET` or `POST` | Same dispatch; response wrapped as one SSE `data:` frame (`data: <json>\n\n`). An empty body is a valid no-op that opens the stream. |

(`internal/mcp/transport_http.go`, `httpHandler`)

### Default address and security

Bare `--http` (no value) binds `127.0.0.1:8765`. A port-only address like `--http :9000` is
rewritten to `127.0.0.1:9000`. Loopback binding is enforced unless an operator supplies an
explicit external IP — spec contents never leave the host by default.

(`internal/mcp/transport_http.go`, `loopbackAddr`)

### Concurrent call serialization

The stdio loop is serial by nature. The HTTP path allows concurrent requests, which would
interleave captured stdout. A mutex serializes all tool calls regardless of how many
concurrent HTTP requests arrive — behavior is identical to stdio, just multi-client.

(`internal/mcp/transport_http.go`, `dispatchLocked`)

### Example

```bash
# Bind default loopback:8765
specd mcp --http --root /path/to/project

# Bind explicit port (still loopback)
specd mcp --http :9000 --root /path/to/project

# Single JSON-RPC call via curl
curl -s -X POST http://127.0.0.1:8765/rpc \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"specd_status","arguments":{}}}'
```

---

## Tool Mapping Contract

The server automatically maps every registered command in `specd` — except the meta commands
`help`, `version`, and `mcp` — to an MCP tool at startup. **No separate registration is
needed**: a new command added to `core.Commands` surfaces as a tool with the next binary build.
(`internal/mcp/tools.go:81-90`, `buildTools`)

### Naming
All tools are prefixed `specd_` to prevent collisions in the host's shared tool list:
`specd_status`, `specd_next`, `specd_task`, `specd_brain`, `specd_pinky`, etc.
(`internal/mcp/tools.go:7`, `toolPrefix`)

Brain and Pinky use the same generated-command shape as every other command: call
`specd_brain` or `specd_pinky` and pass the CLI subcommand in `args`. There is no second
MCP-only naming convention.

### Input schema
Each tool's input schema has:
- **`args`** — an ordered string array for positional CLI arguments (matches the command's
  usage synopsis).
- **One property per flag** — booleans as `boolean`, everything else as `string`.

```json
{
  "name": "specd_task",
  "arguments": {
    "args": ["my-feature", "T1"],
    "status": "complete",
    "evidence": "Tests passed"
  }
}
```

### Annotations
Every tool carries MCP annotations so a host can signal risk before invoking:

| Annotation | Value | Commands |
|---|---|---|
| `readOnlyHint` | `true` | `status`, `waves`, `context`, `check`, `next`, `dispatch`, `report`, and read-only variants (`serve`, `watch`, `validate`, `replay`, `diff`) |
| `readOnlyHint` | `false` | All other commands (state-mutating) |
| `destructiveHint` | `true` | `uninstall`, `update` (mutate the install itself) |

(`internal/mcp/tools.go:16-23`)

### Output schema
Tool results follow the MCP `tools/call` contract:

```json
{
  "content": [{ "type": "text", "text": "<command stdout>" }],
  "isError": false,
  "structuredContent": { ... }
}
```

- **`content`**: The command's stdout. On failure (`isError: true`), stderr diagnostics are
  appended so the host sees a complete error picture.
- **`isError`**: `true` when the CLI command exits with a non-zero code.
- **`structuredContent`**: When stdout is valid JSON, it is parsed and attached as a structured
  object — the host LLM does not need to parse text manually.

(`internal/mcp/server.go:134-160`, `toolResult`)

---

## Exposed MCP Tools

The following tools are exposed automatically. Refer to the
[Command Reference](./command-reference.md) for full parameter details:

| MCP Tool | CLI Analog | Primary Purpose | Read-Only |
|---|---|---|---|
| `specd_init` | `specd init` | Scaffold `.specd/` directory and role configurations | No |
| `specd_doctor` | `specd doctor` | Diagnose scaffold, MCP server, and host registration health | Yes |
| `specd_new` | `specd new` | Create a new spec with artifact stubs | No |
| `specd_approve` | `specd approve` | Clear phase approval gate | No |
| `specd_decision` | `specd decision` | Record architectural decision (ADR) | No |
| `specd_midreq` | `specd midreq` | Log a mid-flight requirement change | No |
| `specd_memory` | `specd memory` | Record or promote a learning | No |
| `specd_next` | `specd next` | Print the next runnable task(s) | Yes |
| `specd_dispatch` | `specd dispatch` | Emit subagent ready-to-run packets | Yes |
| `specd_verify` | `specd verify` | Execute task verification command | No |
| `specd_task` | `specd task` | Flip task status with evidence | No |
| `specd_status` | `specd status` | Render status board or list specs | Yes |
| `specd_check` | `specd check` | Run the validation gates | Yes |
| `specd_waves` | `specd waves` | Render task wave dependency graph | Yes |
| `specd_context` | `specd context` | Retrieve minimal phase briefing and signals | Yes |
| `specd_report` | `specd report` | Generate progress reports | Yes |
| `specd_serve` | `specd serve` | Run read-only HTTP server | Yes |
| `specd_watch` | `specd watch` | Stream runnable-frontier changes | Yes |
| `specd_validate` | `specd validate` | Conformance check against JSON Schema | Yes |
| `specd_replay` | `specd replay` | Render spec audit event timeline | Yes |
| `specd_diff` | `specd diff` | Show git diff of spec artifacts | Yes |
| `specd_program` | `specd program` | Manage inter-spec dependency graph | No |
| `specd_brain` | `specd brain` | Start, step, inspect, pause, resume, or cancel deterministic Brain sessions | No for mutations; status is read-only |
| `specd_pinky` | `specd pinky` | Host worker claim, heartbeat, progress, report, block, and release operations | No |
| `specd_schema` | `specd schema` | Emit embedded spec-format JSON Schema | Yes |
| `specd_update` | `specd update` | Update the specd binary | No (destructive) |
| `specd_uninstall` | `specd uninstall` | Remove specd from the system | No (destructive) |

---

## Configuring the Tool Surface

By default every tool above is advertised. For hosts that benefit from a smaller
surface, an optional `mcp` block in `.specd/config.json` filters what
`tools/list` returns. **The block is opt-in: omit it and the advertised set is
byte-identical to the default — no behavioural change.**

```jsonc
{
  "mcp": {
    "expose": "essential",                      // "all" (default) | "essential" | "phase"
    "essentialTools": ["specd_inspect",          // command/composite/intent names kept under "essential"
                       "specd_read", "specd_query",
                       "verify", "task", "approve"],
    "includeMeta": false,                        // expose update/uninstall/schema (default false)
    "includeOrchestration": null                 // null => derive from orchestration.enabled
  }
}
```

| Field | Effect |
|---|---|
| `expose` | `"all"` advertises every non-meta tool; `"essential"` advertises only the `essentialTools` set; `"phase"` advertises a subset that adapts to the active spec's lifecycle status (see below). An unknown value degrades to `"all"` with one stderr diagnostic (never on the protocol stream). |
| `essentialTools` | Names kept under `expose:"essential"`. Empty ⇒ built-in default set: `specd_inspect, specd_read, specd_query, verify, task, approve` (the composites cover the read surface). |
| `includeMeta` | When false (default) the install-maintenance tools `specd_update`, `specd_uninstall`, and the spec-pack-author tool `specd_schema` are hidden from MCP (they remain available on the CLI). |
| `includeOrchestration` | A `*bool`: `null`/absent derives from `orchestration.enabled`; an explicit `true`/`false` overrides it. When excluded, `specd_brain`, `specd_pinky`, and every `brain_*` intent tool are hidden. |

Filtering only ever *hides* tools — it never grants new authority, and tool
order stays deterministic (command order, then intent order).

### Phase-adaptive surface (`expose: "phase"`)

`"phase"` makes the advertised tool list track the active spec's lifecycle
status, so a host sees the affordances that matter *now* instead of the whole
surface:

| Status band | Advertised tools |
|-------------|------------------|
| `requirements` / `design` / `tasks` (planning) | `specd_inspect`, `specd_read`, `specd_query`, `specd_check`, `specd_approve`, `specd_context`, `specd_status`, `specd_waves` |
| `executing` / `verifying` / `blocked` | `specd_inspect`, `specd_read`, `specd_next`, `specd_dispatch`, `specd_verify`, `specd_task`, `specd_status`, `specd_context` |

The subset is always a *narrowing* of what `expose` would otherwise permit — the
`includeMeta`/`includeOrchestration` gates still apply. The "active" status is the
furthest-along in-progress spec in the project (executing outranks planning).

In this mode the server advertises `capabilities.tools.listChanged: true` and runs
a background watcher that polls `state.json` (same `SPECD_WATCH_INTERVAL_MS` knob
as `specd watch`, default 1000ms, plus a short debounce). When the active phase
changes it swaps the list and emits a `notifications/tools/list_changed` so the
host re-fetches `tools/list`. The watcher stops cleanly when the stdio stream
closes — no goroutine leak.

**Host caveats:**
- Hosts that ignore `notifications/tools/list_changed` simply keep the initial
  subset — behaviour degrades gracefully, never breaks.
- The HTTP/SSE transport is request/response with no standing server→client
  channel, so it cannot *push* notifications. Under `--http` the list still
  updates (the watcher keeps the shared registry current) but the host must poll
  `tools/list` to observe it. Use stdio for live push.

### Per-spec tool policy (`contextManifest`)

A spec can declare a per-spec tool policy in `.specd/specs/<slug>/manifest.json`
— the most precise filter layer, composed *after* the config/phase plan. It only
ever further narrows what the config already permits.

```jsonc
{
  "contextManifest": {
    "requiredTools":  ["specd_inspect", "specd_verify", "specd_task"],
    "optionalTools":  ["specd_decision", "specd_memory"],
    "forbiddenTools": ["specd_approve"]
  }
}
```

| Field | Effect |
|---|---|
| `requiredTools` | Forced present (subject to config gates — see below) and, with `optionalTools`, forms the allowlist: everything not named is dropped. |
| `optionalTools` | Allowed alongside `requiredTools`. |
| `forbiddenTools` | Hard exclude — overrides required/optional/config unconditionally. |

Precedence is `forbidden` > config gate > `required`/`optional` allowlist > phase
plan. A `requiredTool` the config's `includeMeta`/`includeOrchestration` gate has
already hidden stays hidden (config safety wins) and the server logs a stderr
diagnostic. Unknown tool names are ignored with a stderr warning. The policy is
read-only and applies to the project's active spec (the same furthest-along spec
the phase watcher tracks). An absent `mcp` config block never applies a manifest,
so the default surface stays byte-identical.

### Host capability negotiation (`capabilities.specd`)

`initialize` accepts three optional, **non-standard** hints under
`capabilities.specd`; hosts that omit them are entirely unaffected.

```jsonc
{
  "method": "initialize",
  "params": {
    "capabilities": {
      "specd": {
        "maxTools": 8,
        "preferredNamespaces": ["read", "orchestration"],
        "maxContextTokens": 8000
      }
    }
  }
}
```

| Field | Effect |
|---|---|
| `maxTools` | Caps the emitted tool count. Config/manifest-`required` tools are never dropped to satisfy it — if they alone exceed `maxTools`, all required tools are still emitted plus a stderr diagnostic. `≤ 0` is a no-op. |
| `preferredNamespaces` | Orders matching-namespace tools first (in the given order) and prefers them when truncating. Namespaces: `read` (`specd_inspect`/`read`/`query`), `orchestration` (`specd_orchestrate`/`worker` + `brain_*`), `meta`, `core`. A namespace may also be named by a member tool (e.g. `specd_read`). |
| `maxContextTokens` | Caps the `budget` of every context manifest produced during the session (Pinky missions, dispatch packets). Threaded into the engine as the host context window. `≤ 0` / garbage is ignored safely; omitting it yields the derived per-phase budget unchanged. |

The negotiated preferences persist for the session and re-apply on every
`tools/list` (including dynamic re-fetches under `expose:"phase"`). Garbage values
(negative `maxTools`, unknown namespaces) are ignored safely and never tear down
`initialize`. Omitting `capabilities.specd` yields byte-identical output to the
feature-off path.

---

## Composite tools

Composite tools collapse the 1:1 command→tool mapping into a handful of
view-/action-routed verbs. They are **dispatch wrappers**: each validates its
selector against a fixed allowlist, then routes to the same handler the atomic
tool would — identical output, no new authority. They appear whenever an `mcp`
block is present (an absent block keeps the pre-composite surface byte-for-byte).

| Tool | Selector | Routes to |
|------|----------|-----------|
| `specd_inspect` | `view`: `status\|waves\|context\|check\|validate\|replay\|diff` | the matching read command |
| `specd_read` | `view`: `report` (+ `format: md\|html`) | `report` (streaming `serve`/`watch` stay CLI-only over MCP) |
| `specd_query` | `view`: `next\|dispatch` | `next`/`dispatch` |
| `specd_orchestrate` | `action`: `start\|step\|status\|why\|pause\|resume\|cancel` | the matching `brain` sub-action |
| `specd_worker` | `action`: `claim\|heartbeat\|progress\|query\|report\|block\|release\|inbox` | the matching `pinky` sub-action |

An unknown or missing `view`/`action` returns an MCP error naming the valid
values — no dispatch. `specd_orchestrate`/`specd_worker` follow the orchestration
gate (hidden when orchestration is excluded). The atomic `specd_*` and `brain_*`
tools remain under `expose:"all"` for back-compat.

## Resources

The MCP `resources` capability exposes spec artifacts and steering files for
direct host reads, so reading context no longer costs a tool call.

- `resources/list` enumerates every existing artifact (`requirements.md`,
  `design.md`, `tasks.md`, `decisions.md`, `memory.md`, `mid-requirements.md`,
  `state.json`) per spec plus `.specd/steering/*.md`, in deterministic order.
- `resources/read` returns content by URI with the right mime
  (`text/markdown`, `application/json`).
- URI scheme: `specd://specs/<slug>/<artifact>` and `specd://steering/<file>`.
- Read-only and strictly contained: any URI resolving outside `.specd/` is
  rejected before a byte is read, and unknown/traversal URIs return a
  resource-not-found error with no filesystem disclosure.

## Prompts

The MCP `prompts` capability serves specd's phase and role guidance as reusable,
deterministic templates (embedded — no network, no LLM).

- `prompts/list` returns four phase prompts (`phase/requirements`,
  `phase/design`, `phase/tasks`, `phase/execute`) and two role prompts
  (`role/builder`, `role/investigator`) with declared arguments.
- `prompts/get` renders a prompt's messages. Phase prompts accept an optional
  `slug` that injects a one-line spec-context header.
- Identical inputs always render identical messages.

---

## Orchestration through MCP

Brain/Pinky orchestration is exposed through generated tools, not custom MCP business logic.
The MCP request is always one bounded CLI invocation; it never waits for an LLM worker to finish.
Hosts must run their own worker loop and call Pinky tools as work progresses.

### Brain control loop

A host starts or attaches a Brain session, then polls status or steps one decision at a time:

```json
{
  "name": "specd_brain",
  "arguments": {
    "args": ["start", "my-feature"],
    "approval-policy": "manual",
    "max-workers": "2",
    "max-retries": "2",
    "timeout-seconds": "7200",
    "json": true
  }
}
```

```json
{
  "name": "specd_brain",
  "arguments": { "args": ["step", "my-feature"], "session": "<session-id>", "approval-policy": "manual", "max-workers": "2", "max-retries": "2", "timeout-seconds": "7200", "json": true }
}
```

Use `specd_brain` with `args: ["status"]` for a bounded read of persisted session state.
Use `args: ["pause"]`, `["resume"]`, or `["cancel"]` to persist cooperative controls. `cancel`
records intent and causes later Brain steps to emit cancellation directives; specd never kills host
processes.

### Pinky worker lifecycle

When Brain emits a mission event, the host is responsible for starting or selecting a worker,
feeding it the mission JSON, and calling Pinky tools:

1. `specd_pinky` with `args: ["claim"]` and `mission: "<path-or->"` to acquire the ACP lease.
2. `specd_pinky` with `args: ["heartbeat"]` before lease expiry while work continues.
3. `specd_pinky` with `args: ["progress"]` for telemetry, or `args: ["query"]` with `text` for a bounded mid-task clarification.
4. Poll `specd_pinky` with `args: ["inbox"]` for Brain directives; if no bounded answer is possible, use `args: ["block"]` and stop.
5. Run the task's normal `specd verify <slug> <task>` command through the host shell.
6. `specd_pinky` with `args: ["report"]` and a matching `verification-ref` to submit terminal evidence.
7. `specd_pinky` with `args: ["release"]` when abandoning or after terminal handling.

Pinky reports complete a task only when they bind to the matching passing verification record, the
reported changed files match the recorded verify scope, and the task's evidence gate accepts the
proof. Host-reported tokens, cost, duration, summaries, and progress are stored as telemetry; they
are not completion proof by themselves.

### Approval, recovery, and bounded polling

- Manual approval remains the default. `planning` and `session` policies never clear high/critical
  mid-requirement gates.
- MCP `watch` must be called with `--once` unless the transport is a streaming host path; for
  orchestration, prefer repeated bounded `specd_brain status` / `step` calls.
- If a host crashes, restart the MCP server, call `specd_brain status`, then `step`. Brain recovers
  from persisted session files and the ACP event log, reclaims expired leases, and retries within
  the configured retry budget.
- If a worker needs a small clarification, it may send `query`; the host/Brain replies with
  `brain directive` and the worker reads it from `pinky inbox`. If the question cannot be bounded,
  use `block` and stop.
- If a worker sees a cancellation directive, it should stop at a safe point, report cancellation or
  release the lease, and let the next Brain step converge.

---

## Host Configuration

### Automated setup (recommended)

For the managed adapters — **claude-code, codex, cursor, antigravity, vscode** — you do not
need to edit any config by hand. `specd init` detects the host and registers the
server for you, **project-scoped**:

```bash
specd init --agent auto            # detect and configure the unambiguous host
specd init --agent claude-code --yes
specd init --agent all --yes       # every detected host
specd doctor                       # verify registration + MCP handshake
```

Where the host ships a stable CLI, specd uses it (`claude mcp add --scope project`);
for Antigravity it writes `.agents/mcp_config.json` directly with a targeted JSON merge
that preserves your other servers. `.agents/` is intentionally VCS-tracked so the
project host config stays with the repo. See [agent-harness-compat.md](agent-harness-compat.md)
for the per-host install method and verification depth.

#### Trust boundaries

- **Project scope by default.** specd writes only inside your repo. Global/user host
  config is never modified without `--scope global` **and** explicit consent.
- **Fail closed.** Existing host config must parse before any change; on a mutation
  specd writes a timestamped backup first and reports its path.
- **Preservation.** Unrelated MCP servers and settings are never removed or rewritten.
- **No secrets.** Environment secrets are never copied into generated config.
- **Host-native trust stands.** specd never bypasses a host's own trust/approval
  prompt, and never starts or controls the agent. Restart/reload of the host stays
  user-controlled.
- specd records what it created in `.specd/integrations.json` so repair/uninstall only
  touches specd-owned entries.

### Manual snippets (air-gapped / unmanaged hosts)

`specd mcp --config <host>` prints a ready-to-paste config snippet for any supported host and
exits without starting the server. Use it when there is no host CLI, for the
snippet-only host (**claude-desktop**), or when you prefer to merge config yourself.
Combine with `--root` to substitute your project path:

```bash
specd mcp --config cursor
specd mcp --config claude-desktop --root /path/to/your/project
```

Supported values: `antigravity`, `claude-code`, `claude-desktop`, `codex`, `cursor`, `vscode`.

---

### Claude Desktop

Add the following block to your Claude Desktop configuration file:
- **macOS/Linux:** `~/.config/Claude/claude_desktop_config.json`
- **Windows:** `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp", "--root", "/path/to/your/project"]
    }
  }
}
```

Replace `/path/to/your/project` with the absolute path to your specd project root, or run
`specd mcp --config claude-desktop --root /path/to/your/project` to get a pre-filled snippet.

### Cursor

Add the following to Cursor's MCP configuration (`.cursor/mcp.json` in your project, or the
global MCP settings):

```json
{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp", "--root", "/path/to/your/project"]
    }
  }
}
```

Run `specd mcp --config cursor --root /path/to/your/project` to get a pre-filled snippet.

**Note:** Cursor sends newline-delimited JSON framing. `Content-Length` auto-detection means no
specd-side change is needed if Cursor switches framing in a future version.

### VS Code

Add `specd` to the workspace MCP file at `.vscode/mcp.json` under the top-level
`servers` key:

```json
{
  "servers": {
    "specd": {
      "type": "stdio",
      "command": "specd",
      "args": ["mcp", "--root", "/path/to/your/project"]
    }
  }
}
```

Run `specd mcp --config vscode --root /path/to/your/project` to get a pre-filled snippet.

### Antigravity CLI

Merge the following into your Antigravity MCP config file. Antigravity reads from three
locations in priority order:
1. `.agents/mcp_config.json` (workspace-local, project-scoped)
2. `~/.gemini/antigravity-cli/mcp_config.json` (per-CLI global)
3. `~/.gemini/config/mcp_config.json` (shared with Antigravity IDE and 2.0)

```json
{
  "mcpServers": {
    "specd": {
      "command": "specd",
      "args": ["mcp", "--root", "/path/to/your/project"],
      "env": {
        "MCP_MODE": "stdio",
        "DISABLE_CONSOLE_OUTPUT": "true"
      }
    }
  }
}
```

The `MCP_MODE` and `DISABLE_CONSOLE_OUTPUT` env vars prevent Antigravity's debug output from
leaking onto stdout and corrupting the JSON-RPC stream. Replace `/path/to/your/project`
with the absolute path to your specd project root.

Run `specd mcp --config antigravity --root /path/to/your/project` to get a pre-filled snippet.

### OpenAI Codex CLI

Merge the following into your Codex config file:
- **Global:** `~/.codex/config.toml`
- **Project-scoped (trusted):** `.codex/config.toml`

```toml
[mcp_servers.specd]
command = "specd"
args = ["mcp", "--root", "/path/to/your/project"]
```

Replace `/path/to/your/project` with the absolute path to your specd project root.
Codex natively supports stdio MCP servers — no extra environment variables or transport
adapters are needed.

Run `specd mcp --config codex --root /path/to/your/project` to get a pre-filled snippet.

---

## Limitations

| Limitation | Detail |
|---|---|
| **Loopback HTTP only** | `specd mcp --http` binds loopback (`127.0.0.1`) by default and is not an authenticated remote endpoint. Do not expose it on a public interface without an authenticating proxy in front. |
| **Static tool list (default)** | `tools.listChanged` is `false` except under `expose:"phase"`, where the tool list mutates with the active phase and `listChanged` is advertised `true`. In other modes the list is fixed for the process lifetime. |
| **Serial tool calls** | Tool calls are processed one at a time. Concurrent calls from the same host are serialised by the stdio loop. |
| **Host config schema drift** | MCP support in Cursor and VS Code is evolving. Config examples in this guide target the `2024-11-05` protocol revision and may need adjustment for future host versions. |
