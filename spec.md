# specd Init and Agent Discovery Specification

Status: proposed  
Scope: post-install project initialization, coding-agent discovery, MCP registration, onboarding verification  
Primary command: `specd init`

## 1. Executive summary

`specd` already has strong enforcement and MCP foundations:

- deterministic Go CLI with no runtime dependencies;
- embedded steering, role, skill, and `AGENTS.md` templates;
- idempotent marker-based `AGENTS.md` merge;
- native MCP server over stdio and HTTP/SSE;
- generated MCP tools backed by CLI command metadata;
- config snippets for Claude Desktop, Cursor, VS Code, Antigravity, and Codex;
- atomic file writes and broad lifecycle tests.

Current first-run flow does not convert those capabilities into smooth adoption.
Installation ends with “run `specd init`.” `init` then copies files and prints a
file list. User must discover MCP support, identify their coding agent, find its
configuration location, merge a snippet, restart or reload the host, confirm tool
discovery, read a steering skill, inspect the repository, and choose the next
command.

Recommended product direction:

> Make `specd init` a safe, deterministic onboarding orchestrator that installs
> project assets, discovers compatible local coding-agent hosts, offers or performs
> project-scoped MCP registration, verifies the integration, and returns one exact
> next action. Keep repository interpretation and prose authoring as agent work.

This preserves the Foundational Split. `specd` detects facts and configures
interfaces; the coding agent reasons about the repository and authors steering.

## 2. Goals

1. Reduce installation-to-first-agent-use to one project command.
2. Make `specd` discoverable as an MCP tool without manual config editing where a
   supported host exposes a stable project-scoped configuration mechanism.
3. Preserve user-owned host configuration and repository files.
4. Make every mutation previewable, idempotent, atomic where practical, and
   recoverable.
5. Produce deterministic human and JSON output suitable for shell scripts,
   installers, agents, and IDE integrations.
6. Verify that configured hosts can start `specd mcp`, negotiate MCP, and list
   `specd` tools.
7. Give the user or agent one clear next action after initialization.
8. Keep zero runtime dependencies and zero LLM calls.
9. Build host integration as a data-driven adapter system, not host-specific logic
   embedded in `RunInit`.
10. Establish measurable onboarding quality suitable for a production CLI product.

## 3. Non-goals

- `specd init` will not infer product intent or author `product.md`, `tech.md`, or
  `structure.md`.
- `specd init` will not silently modify global user configuration.
- `specd init` will not launch or control a coding agent.
- `specd init` will not install coding-agent products.
- `specd init` will not expose unauthenticated MCP HTTP on a non-loopback address.
- `specd init` will not replace host-native trust or approval prompts.
- Initial release will not promise universal host support. Unsupported hosts retain
  `AGENTS.md` plus CLI-shell compatibility.

## 4. Current architecture

### 4.1 Init path

`internal/cmd/init.go`:

- uses current working directory as project root;
- writes six steering files, four role files, six skills, and `config.json`;
- merges embedded instructions into root `AGENTS.md`;
- skips existing files unless `--force`;
- supports separate `--pack`, `--list-packs`, and remote pack checksum flow;
- prints written, merged, and skipped paths.

`internal/core/agents.go`:

- owns marker-based merge;
- preserves content outside managed markers;
- supports destructive `force` reset.

`internal/core/io.go`:

- provides atomic temp-file, fsync, chmod, rename writes.

### 4.2 MCP path

`internal/cmd/mcp.go`:

- starts stdio MCP by default;
- can bind HTTP/SSE;
- can print host config snippets with `--config <host>`;
- can scope all tool calls to `--root <path>`.

`internal/mcp/hosts.go`:

- embeds five host snippets;
- substitutes project roots;
- returns sorted host names.

`internal/mcp/server.go` and `internal/mcp/tools.go`:

- expose CLI commands as MCP tools;
- force structured command output;
- attach read-only and destructive annotations;
- support `initialize`, `tools/list`, `tools/call`, and ping;
- currently advertise protocol revision `2024-11-05`;
- currently omit server-wide MCP `instructions`.

### 4.3 Existing strengths

1. Small trusted core and no dependency supply chain.
2. CLI and MCP share dispatch logic, preventing behavior drift.
3. Embedded templates make installation portable.
4. Marker merge protects user-authored `AGENTS.md` sections.
5. Existing host compatibility tests create a good base for adapter conformance.
6. Project-root pinning through `specd mcp --root` prevents ambiguous lookup.
7. MCP tool annotations already communicate mutation risk.
8. Documentation already explains manual MCP setup.

## 5. Gap analysis

### P0 — reliability and trust gaps

#### G1. Init can report success after scaffold write failures

The local `place` closure prints an error and returns, but `RunInit` continues and
finally returns exit code `0`. A partially initialized project can therefore look
successful.

Required fix:

- collect all errors;
- stop before later phases when a required scaffold write fails;
- return exit code `1`;
- JSON receipt must report `status: "failed"` and failed paths;
- never claim readiness after partial failure.

#### G2. Default scaffold is not transactional as a unit

Each file write is atomic, but the full initialization operation is not. Failure
after several writes leaves a mixed tree.

Required fix:

- introduce an initialization plan;
- preflight every target and template before writes;
- stage new `.specd/` content in a sibling temporary directory for first init;
- rename staged tree into place when possible;
- treat `AGENTS.md` and existing-tree upgrades as journaled mutations with backups
  or explicit partial-success reporting;
- never delete user content during rollback.

#### G3. `--force` is too broad and destructive

`--force` overwrites all managed scaffold files and resets `AGENTS.md`, including
user content outside markers. This is unsafe for a routine repair or upgrade.

Required fix:

- split behaviors:
  - `--repair`: recreate missing managed files, preserve existing files;
  - `--refresh`: update only specd-managed/frozen assets and marker sections;
  - `--force`: retained as explicit destructive compatibility mode with warning;
- require clear human output listing destructive effects;
- add `--dry-run` preview.

#### G4. Host configuration is copy/paste, not installation

`specd mcp --config <host>` prints snippets only. User must merge them manually.
This is main onboarding break.

Required fix:

- create host adapters with detection, plan, install, verify, and uninstall/repair
  metadata;
- prefer host-native CLI registration commands when stable;
- otherwise perform structured project-file merge;
- never overwrite complete config files with snippets.

### P1 — adoption and product-value gaps

#### G5. No coding-agent host auto-detection

`init` does not inspect installed CLIs or project configuration. It cannot suggest
the shortest supported integration path.

Required fix:

- detect executable presence with `exec.LookPath`;
- detect project config markers;
- report confidence and reason;
- separate detection from installation;
- support `--agent auto|codex|claude-code|cursor|vscode|gemini|none`;
- if multiple hosts are found in non-interactive mode, configure none unless user
  explicitly selects `--agent all` or a host.

#### G6. Supported-host registry is snippet-centric

Registry stores only destination text and embedded content. It lacks config scope,
installation method, detection probes, verification method, and merge semantics.

Required fix:

- replace with typed adapter metadata and methods;
- keep templates embedded;
- make adapter order deterministic;
- test every adapter against shared conformance suite.

#### G7. No onboarding receipt or next action

Human output is a long file list. MCP and `SPECD_JSON=1` callers do not receive a
stable init result schema. `RunInit` does not emit JSON despite MCP forcing JSON
mode.

Required fix:

- define versioned `InitResult`;
- include root, mode, files, detected hosts, configured hosts, verification,
  warnings, backups, and `nextAction`;
- human output should lead with outcome and next command;
- detailed file list moves below concise summary or behind `--verbose`.

#### G8. No integration health check

Current tests prove internal server behavior, but user cannot run one command to
confirm their project setup.

Required fix:

- add `specd doctor` or `specd init --check`;
- verify binary resolution, project root, scaffold integrity, MCP handshake,
  `tools/list`, expected core tools, host config presence, and config parseability;
- do not require launching full GUI hosts for baseline success;
- output remediation commands.

#### G9. MCP server omits workflow instructions

Current MCP initialization response does not provide `instructions`. Modern MCP
clients can use server instructions as cross-tool guidance. Codex documentation
explicitly states that it reads this field.

Required fix:

- return concise server instructions:
  - call `specd_status` or `specd_context` first;
  - do not edit `state.json` or task checkboxes;
  - use `specd_check` before approval;
  - use verify evidence before completion;
- keep first 512 characters self-contained;
- keep `AGENTS.md` as durable fallback because not all hosts consume instructions.

#### G10. MCP protocol negotiation is stale and fixed

Server ignores client-requested protocol version and always returns `2024-11-05`.
Current MCP lifecycle requires version negotiation. Latest public specification at
analysis time is `2025-11-25`.

Required fix:

- parse initialize parameters;
- support an explicit ordered set of protocol revisions;
- echo requested version when supported;
- otherwise return latest supported version;
- preserve compatibility tests for older clients;
- test unsupported-version behavior.

#### G11. Host portfolio misses important CLI-first paths

Current registry has Claude Desktop but not Claude Code, and Antigravity but not a
first-class Gemini CLI adapter. Coding-agent harness market requires CLI-focused
integration first.

Required initial portfolio:

1. Codex CLI/project config.
2. Claude Code/project `.mcp.json`.
3. Gemini CLI/project `.gemini/settings.json`.
4. Cursor/project `.cursor/mcp.json`.
5. VS Code/workspace `.vscode/mcp.json` or currently supported workspace format.
6. Generic/manual snippet fallback.

Host schemas change quickly. Each adapter must cite a tested schema version and be
isolated from core init logic.

### P2 — polish, lifecycle, and market-readiness gaps

#### G12. Install script ends before integration

Installer only points to `specd init`; it does not verify that new shell can resolve
the binary or explain agent auto-setup.

Required fix:

- print exact binary path;
- run installed binary `version`;
- explain `cd <project> && specd init --agent auto`;
- avoid initializing arbitrary current directories from a curl installer;
- optionally provide `--project <path>` only if explicitly requested later.

#### G13. No managed integration manifest

There is no record of which host config entries `specd` created. Safe repair and
uninstall need ownership data.

Required fix:

- write `.specd/integrations.json`;
- record schema version, host, scope, target path, server name, root, install
  method, content fingerprint, and timestamp;
- never store secrets;
- use manifest to distinguish specd-owned entry from user-owned config.

#### G14. No schema-aware config merge

Blind snippet replacement would destroy neighboring MCP servers and settings.
JSON and TOML need targeted merge behavior.

Required fix:

- JSON adapters parse object, mutate only owned server key, preserve unrelated
  semantic content, and write deterministic formatting;
- TOML should prefer host-native CLI because stdlib-only constraint makes complete,
  comment-preserving TOML editing expensive and risky;
- if safe merge cannot be guaranteed, print command/snippet and mark setup manual;
- create timestamped backup before changing existing host config.

#### G15. No onboarding telemetry or measurable funnel

No metrics exist for init success, host detection, integration success, or time to
first spec. Product decisions cannot be evidence-driven.

Required fix:

- keep default fully offline and private;
- emit local deterministic lifecycle events or counters only if they fit existing
  telemetry policy;
- any remote analytics must be explicit opt-in and separately specified;
- establish benchmark scripts for time-to-init and doctor latency.

#### G16. Documentation starts from commands, not outcomes

Quickstart asks user to understand internals before seeing an agent use the tool.

Required fix:

- new golden path:
  1. install;
  2. `cd project`;
  3. `specd init --agent auto`;
  4. restart/reload host if needed;
  5. ask agent: “Use specd to plan <feature>”;
- retain manual and air-gapped flows;
- document trust boundaries for project MCP configuration.

## 6. Target user experience

### 6.1 Interactive terminal

```text
$ specd init

Initialized specd in /work/acme-api
  Project assets: 18 ready, 0 failed
  Coding agents detected: Codex CLI, Claude Code

Configure project-scoped MCP integration? [Codex CLI / Claude Code / both / skip]

  ✓ Codex: .codex/config.toml entry installed through `codex mcp add`
  ✓ Claude Code: .mcp.json entry installed through `claude mcp add --scope project`
  ✓ MCP handshake passed; 24 specd tools discovered

Next: ask your coding agent:
  "Read specd context and help me create a spec for <feature>."
```

Prompting must occur only when stdin and stdout are terminals and `--yes` or
`--non-interactive` is absent.

### 6.2 Deterministic automation

```sh
specd init --agent codex --yes --json
specd init --agent all --dry-run --json
specd init --agent none --non-interactive
specd init --check --json
```

Rules:

- `--agent auto` detects and configures one unambiguous host in interactive mode;
- `--agent <name>` is explicit and works non-interactively;
- `--agent all` configures all detected supported hosts;
- `--agent none` scaffolds only;
- `--yes` accepts non-destructive project-scoped changes;
- global scope always requires explicit `--scope global` and confirmation;
- CI/non-TTY default performs scaffold only and returns suggested host actions.

### 6.3 Re-run and repair

```text
$ specd init
specd already initialized.
  ✓ managed scaffold current
  ✓ Codex integration healthy
  ! Claude Code config missing

Next: specd init --repair --agent claude-code
```

Re-running healthy init must make zero file changes.

## 7. Proposed CLI contract

### 7.1 `specd init`

```text
specd init
  [--agent <auto|all|none|codex|claude-code|gemini|cursor|vscode>]
  [--scope <project|global>]
  [--yes]
  [--non-interactive]
  [--dry-run]
  [--repair]
  [--refresh]
  [--check]
  [--json]
  [--verbose]
  [--force]
```

Compatibility:

- existing `--pack`, `--list-packs`, and `--sha256` remain;
- pack application must be composed into the init plan rather than bypassing
  standard scaffold unless current pack semantics intentionally replace it;
- conflicting modes return exit code `2` with actionable usage.

### 7.2 `specd doctor`

Recommended separate command:

```text
specd doctor [--agent <name|all>] [--json] [--fix]
```

`--fix` may perform only safe, project-scoped, specd-owned repairs. Global changes
still require explicit scope and confirmation.

### 7.3 Exit codes

- `0`: requested initialization/configuration complete and checks pass;
- `1`: write, config, handshake, or health failure;
- `2`: invalid flags, conflicting modes, unavailable requested host CLI;
- `3`: only where an operation expects an existing initialized root and none exists.

Partial configuration is exit `1`, even if scaffold succeeded.

## 8. Architecture design

### 8.1 Init planner

Add `internal/core/initplan.go`.

Core types:

```go
type InitOptions struct {
    Root           string
    Force          bool
    Repair         bool
    Refresh        bool
    DryRun         bool
    Interactive    bool
    AgentSelection []string
    Scope          string
}

type InitAction struct {
    Kind        string
    Target      string
    Description string
    Destructive bool
}

type InitPlan struct {
    Root     string
    Actions  []InitAction
    Warnings []string
}
```

Planner performs no writes. Executor applies deterministic action order and
returns structured results.

### 8.2 Scaffold manifest

Add an embedded manifest rather than maintaining independent arrays in
`internal/cmd/init.go`.

```go
type ScaffoldAsset struct {
    Template string
    Target   string
    Policy   string // create, managed-refresh, marker-merge
    Required bool
}
```

Benefits:

- one source for init, repair, tests, and doctor;
- detects missing embedded templates;
- makes policy explicit per file;
- simplifies future migrations.

### 8.3 Host adapter registry

Add `internal/integration` or extend `internal/mcp` with strict separation from
transport server code.

```go
type HostAdapter interface {
    Name() string
    Detect(root string) Detection
    Plan(root string, scope Scope) (HostPlan, error)
    Install(plan HostPlan) (HostResult, error)
    Inspect(root string, scope Scope) (HostState, error)
    Verify(root string) Verification
}
```

Detection contains:

- executable path;
- project marker/config path;
- supported scopes;
- install method;
- confidence;
- reason.

Adapters should prefer host-native commands:

- Codex: `codex mcp add ... -- specd mcp --root <root>`;
- Claude Code: `claude mcp add ... --scope project -- specd mcp --root <root>`;
- Gemini CLI: `gemini mcp add --scope project ...`;
- file merge only where official project config is stable and native CLI is absent.

Exact commands must be validated against current official host versions during
implementation. Adapter tests use fake executables and fixtures, never developer
machine config.

### 8.4 Integration manifest

Add `.specd/integrations.json`:

```json
{
  "version": 1,
  "entries": [
    {
      "host": "codex",
      "scope": "project",
      "serverName": "specd",
      "root": "/work/acme-api",
      "method": "native-cli",
      "target": ".codex/config.toml",
      "fingerprint": "sha256:..."
    }
  ]
}
```

Paths in committed project manifests require portability review. Prefer relative
root tokens when host supports them. If absolute roots are required, add manifest
to `.gitignore` or split portable declaration from local state.

### 8.5 Verification probe

Implement in-process MCP protocol probe:

1. construct initialize request with newest supported revision;
2. call server through pipes or direct connection abstraction;
3. validate initialize response;
4. send initialized notification;
5. call `tools/list`;
6. assert baseline tools: `specd_init`, `specd_status`, `specd_context`,
   `specd_check`, `specd_next`, `specd_verify`, `specd_task`;
7. report protocol revision, tool count, and latency.

Host verification has two layers:

- server health: always deterministic and host-independent;
- host registration health: inspect host config or native host listing command.

Do not claim GUI host runtime success unless host provides a stable machine-readable
check.

### 8.6 MCP lifecycle upgrade

Add typed initialize params and supported versions.

Initial response should include:

```json
{
  "protocolVersion": "<negotiated>",
  "capabilities": {"tools": {"listChanged": false}},
  "serverInfo": {
    "name": "specd",
    "title": "specd",
    "version": "<build version>",
    "description": "Deterministic spec-driven coding harness"
  },
  "instructions": "Call specd_status or specd_context first. Never edit .specd state.json or tasks.md checkboxes directly. Use specd_check before approval and specd_verify before completing tasks."
}
```

### 8.7 Stable init result schema

```json
{
  "schemaVersion": 1,
  "status": "ready",
  "root": "/work/acme-api",
  "mode": "init",
  "files": {
    "written": [],
    "updated": [],
    "skipped": [],
    "failed": []
  },
  "agents": {
    "detected": [],
    "configured": [],
    "manual": []
  },
  "verification": {
    "mcp": "pass",
    "protocolVersion": "2025-11-25",
    "toolCount": 24
  },
  "warnings": [],
  "nextAction": {
    "kind": "agent-prompt",
    "text": "Read specd context and help me create a spec for <feature>."
  }
}
```

All arrays must be non-null and deterministically sorted.

## 9. Safety and security requirements

1. Project scope is default. Global host config needs explicit `--scope global`.
2. No host config mutation without explicit host choice or interactive consent.
3. `--yes` authorizes only documented non-destructive project changes.
4. Existing config must parse before mutation. Invalid config fails closed.
5. Preserve unrelated servers and settings.
6. Backup existing config before mutation; report backup path.
7. Never copy environment secrets into generated config.
8. Resolve `specd` executable to an absolute trusted path when host PATH behavior is
   uncertain; show path in dry-run.
9. Reject project roots containing NUL; quote/encode arguments through structured
   process APIs, never shell concatenation.
10. Do not execute host commands through `sh -c`.
11. Do not bind MCP HTTP beyond loopback during init.
12. Integration removal only removes entry whose ownership/fingerprint matches the
   manifest.
13. Symlink handling must be explicit: reject config targets escaping expected scope
   unless user opts in.
14. Every failure returns non-zero and leaves remediation text.

## 10. Functional requirements

### R1 — reliable scaffold

- R1.1: WHEN any required scaffold write fails, `specd init` SHALL return exit `1`.
- R1.2: WHEN preflight fails, `specd init` SHALL write no project files.
- R1.3: WHEN init is rerun on a healthy project, it SHALL make no byte changes.
- R1.4: WHEN `--repair` is used, missing managed assets SHALL be restored without
  overwriting user-authored files.
- R1.5: WHEN `--refresh` is used, only explicitly managed assets and marker sections
  SHALL update.

### R2 — host discovery

- R2.1: `specd init` SHALL detect supported host executables and project configs.
- R2.2: Detection results SHALL include evidence and confidence.
- R2.3: Non-interactive ambiguous detection SHALL not mutate host configs.
- R2.4: Explicit unsupported host selection SHALL return actionable exit `2`.

### R3 — MCP registration

- R3.1: Project-scoped integration SHALL be default.
- R3.2: Existing host config SHALL preserve unrelated content.
- R3.3: Host-native registration CLI SHALL be preferred where stable.
- R3.4: Every installed integration SHALL be recorded without secrets.
- R3.5: Repeated registration SHALL be idempotent.

### R4 — verification and discovery

- R4.1: Init SHALL perform MCP initialize plus `tools/list` health probe.
- R4.2: Probe SHALL verify baseline workflow tools.
- R4.3: MCP initialize SHALL negotiate supported protocol versions.
- R4.4: MCP initialize SHALL return concise workflow instructions.
- R4.5: `doctor` SHALL distinguish scaffold, server, and host-registration failures.

### R5 — output and automation

- R5.1: `SPECD_JSON=1 specd init` and `specd init --json` SHALL emit one valid JSON
  document and no ANSI.
- R5.2: Init JSON SHALL use versioned stable schema with non-null arrays.
- R5.3: Human output SHALL lead with readiness state and one next action.
- R5.4: `--dry-run` SHALL list exact proposed mutations and commands without writes.

### R6 — portability

- R6.1: Implementation SHALL retain Go stdlib-only runtime.
- R6.2: Linux, macOS, and Windows path/argument behavior SHALL be tested.
- R6.3: Unsupported hosts SHALL still work through `AGENTS.md` and direct CLI use.
- R6.4: Host adapters SHALL be independently versioned/tested so schema drift does
  not break core init.

## 11. Testing strategy

### Unit tests

- init planner action order and conflicts;
- preflight failure produces zero writes;
- required-write failure returns exit `1`;
- scaffold manifest/template parity;
- repair/refresh/force policy;
- JSON schema and non-null arrays;
- host detection with fake PATH;
- adapter command argument construction;
- JSON config merge preservation;
- manifest ownership/fingerprint checks;
- MCP version negotiation and instructions;
- doctor diagnostics and remediation.

### Integration tests

- fresh init → detected host → project registration → MCP probe;
- rerun byte-stability;
- existing multi-server host config preservation;
- invalid host config fail-closed;
- host command failure after scaffold success returns exit `1`;
- spaces and Unicode in project path;
- symlink and permission failures;
- CLI and MCP invocation parity for `specd_init`;
- every supported adapter passes shared conformance suite.

### End-to-end fixtures

Use fake `codex`, `claude`, and `gemini` executables that:

- record argv;
- emulate add/list/remove;
- return controlled failures;
- never touch real user configuration.

### Quality gates

- `make build`;
- `make test`;
- `make ci`;
- no external Go module;
- no writes outside temp/project scope in tests;
- init output deterministic under `-count=2`;
- race detector clean.

## 12. Rollout plan

### Release A — correctness foundation

- fix false-success writes;
- introduce planner/result schema;
- add dry-run, repair, refresh;
- add MCP instructions and protocol negotiation;
- add doctor server probe.

### Release B — CLI agent auto-setup

- Codex, Claude Code, Gemini CLI adapters;
- project-scoped registration;
- integration manifest;
- interactive and non-interactive selection.

### Release C — IDE host setup

- Cursor and VS Code workspace adapters;
- schema-drift fixtures and compatibility matrix;
- polished restart/reload guidance.

### Release D — ecosystem scale

- public host-adapter contribution contract;
- optional remote/streamable-HTTP recipes with authentication guidance;
- onboarding benchmarks and opt-in product telemetry;
- signed integration metadata if marketplace distribution is added.

## 13. Success metrics

Production acceptance targets:

- fresh project scaffold success: 100% across supported test platforms;
- false-success rate: 0%;
- healthy rerun byte changes: 0;
- supported CLI host project registration: one command, no manual file edit;
- MCP health probe: under 500 ms locally at p95;
- first useful next action visible within first 12 output lines;
- JSON output parse success: 100%;
- unrelated host config preservation: 100% fixture coverage;
- time from install to agent seeing `specd_status`: under two minutes for supported
  hosts.

## 14. Product positioning

Init quality is market leverage, not setup polish. Competing coding workflows often
depend on one editor, one model vendor, or prose conventions the model may ignore.
`specd` can own a stronger position:

> Install once, initialize any repository, connect any supported coding agent, and
> enforce one evidence-backed engineering workflow locally.

Durable differentiation comes from:

- host neutrality through CLI plus MCP;
- deterministic enforcement rather than prompt-only compliance;
- project-scoped portable onboarding;
- inspectable local files and no LLM dependency;
- safe configuration ownership and repair;
- measurable compatibility instead of broad unsupported claims.

## 15. Source references

Repository evidence:

- `internal/cmd/init.go`
- `internal/core/agents.go`
- `internal/core/io.go`
- `internal/cmd/mcp.go`
- `internal/mcp/hosts.go`
- `internal/mcp/server.go`
- `internal/mcp/tools.go`
- `internal/core/embed_templates/`
- `docs/mcp-guide.md`
- `docs/agent-harness-compat.md`
- `docs/agent-harness-gap-analysis.md`
- `scripts/install.sh`

Current primary external references reviewed:

- MCP lifecycle and version negotiation:
  https://modelcontextprotocol.io/specification/2025-11-25/basic/lifecycle
- OpenAI Codex MCP configuration and `codex mcp`:
  https://developers.openai.com/codex/mcp
- Anthropic Claude Code MCP installation and project scope:
  https://code.claude.com/docs/en/mcp
- VS Code MCP server management:
  https://code.visualstudio.com/docs/agent-customization/mcp-servers
- Gemini CLI MCP configuration and `gemini mcp add`:
  https://google-gemini.github.io/gemini-cli/docs/tools/mcp-server.html

