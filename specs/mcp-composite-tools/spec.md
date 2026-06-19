# Spec: MCP Composite Tools

> Plan items: **A2** (merge read-only inspection tools), **A4** (clean unified
> orchestration tools). Wave 2 — depends on [mcp-config-tool-filtering](../mcp-config-tool-filtering/spec.md).

## 1. Overview

The current 1:1 command→tool mapping makes inspection alone 10 tools and
orchestration a confusing mix (`specd_brain` raw + `brain_*` intents +
`specd_pinky`). This spec collapses inspection into 3 composite tools and the
orchestration surface into 2 clean verbs, driven by a `--view`/`--action` enum
parameter so one schema replaces many.

Composites are **dispatch wrappers** that route to existing handlers — no new
core authority, identical output to the atomic predecessors (plan §7 round-trip).

## 2. Goals / Non-goals

**Goals**
- `specd_inspect` (view-routed read) replaces 7 read tools.
- `specd_read` (report/serve/watch) and `specd_query` (next/dispatch) consolidate the rest.
- `specd_orchestrate` (action-routed) + `specd_worker` replace `brain`/`pinky`/`brain_*`.
- Atomic tools remain available under `expose:"all"` for power users / backward compat.

**Non-goals**
- Removing the atomic CLI commands (CLI surface is unchanged).
- Resource/prompt channels.

## 3. Foundational facts (verified)
- Inspection commands (category `inspection`, commands.go): `status, check, context, validate, schema, report, serve, replay, diff, waves, program`. Read-only set (tools.go:16): `status, waves, context, check, next, dispatch, report, serve, watch, validate, replay, diff`.
- `brain` sub-actions: `start|run|status|step|why|directive|pause|resume|cancel` (commands.go).
- `pinky` sub-actions: `claim|brief|heartbeat|progress|query|report|block|release|inbox`.
- Intent tools translate args→argv and route through the same `Dispatcher` (intent.go); composites follow this exact pattern.
- `serve`/`watch` long-running; over MCP `watch` is bounded to `--once` (`enforceBoundedToolCall`, server.go:230). `serve` should be excluded from a bounded composite or `--once`-equivalent only.

## 4. Requirements (EARS)

- **R1** THE SYSTEM SHALL expose `specd_inspect` with a required `view` enum ∈ `{status, waves, context, check, validate, replay, diff}` that routes to the matching atomic handler and returns its output unchanged.
- **R2** THE SYSTEM SHALL expose `specd_read` with `view` ∈ `{report}` plus `format` (`md|html`) — `serve`/`watch` streaming transports stay CLI-only over MCP.
- **R3** THE SYSTEM SHALL expose `specd_query` with `view` ∈ `{next, dispatch}` plus `all`/`frontier` flags.
- **R4** THE SYSTEM SHALL expose `specd_orchestrate` with `action` ∈ `{start, step, status, why, pause, resume, cancel}` translating to the matching `brain` sub-action with policy defaults.
- **R5** THE SYSTEM SHALL expose `specd_worker` with `action` ∈ `{claim, heartbeat, progress, query, report, block, release, inbox}` translating to the matching `pinky` sub-action.
- **R6** WHEN an unknown `view`/`action` is supplied, THE SYSTEM SHALL return an MCP error naming the valid enum values (no dispatch).
- **R7** WHEN orchestration is excluded by config, THE SYSTEM SHALL NOT expose `specd_orchestrate`/`specd_worker`.
- **R8** THE SYSTEM SHALL produce output for each composite+view byte-identical to invoking the atomic command with equivalent flags (round-trip parity).
- **R9** Composite read tools SHALL carry `readOnlyHint:true`; `specd_orchestrate`/`specd_worker` SHALL carry `readOnlyHint:false` except status/why/inbox/query views.

## 5. Design

### 5.1 Registration model
Composites are intent-style tools (named args, no positional `args` array). Add a
`compositeTools []intentTool` set (or extend `intentTools`) with translators:
- `specd_inspect{view, slug, ...}` → `(view, [slug])` routed to atomic command.
- `specd_orchestrate{action, spec/session, policy...}` → `("brain", [action, ...])`.
- `specd_worker{action, ...}` → `("pinky", [action, ...])`.

Reuse `argString`/`argBool` and the `intentTool.def()` schema emitter (intent.go).
Add an `enum` field to `schemaProp` (currently `Type/Description/Items`) so the
`view`/`action` property advertises allowed values to the host.

### 5.2 Enum validation
Translators validate `view`/`action` against a fixed allowlist before building
argv (R6). Invalid ⇒ `fmt.Errorf` surfaced as `errInvalidParams`.

### 5.3 Interaction with filtering (Wave 1)
Exposure plan gains a notion of "tool family": when `expose:"essential"`, prefer
composites; when `expose:"all"`, emit composites **and** atomics (back-compat).
The existing `brain_*` intent tools are superseded by `specd_orchestrate` but
retained under `all` (deprecate in docs, remove in a later major). Document the
overlap so [mcp-dynamic-tool-list](../mcp-dynamic-tool-list/spec.md) can prefer
composites in phase mode.

### 5.4 Bounded streaming
`specd_read` excludes `serve` (long-running HTTP) and `watch` (stream); these stay
CLI-only over MCP, consistent with `enforceBoundedToolCall`. Document the reason.

## 6. Acceptance criteria
- **AC1** `specd_inspect view=status slug=X` output == `specd_status X --json`.
- **AC2** `specd_inspect view=diff` requires `from`; missing ⇒ enum/flag error, no dispatch.
- **AC3** `specd_orchestrate action=status session=S` == `specd_brain status --session S`.
- **AC4** `specd_worker action=claim` == `specd_pinky claim`.
- **AC5** unknown `view`/`action` ⇒ error listing valid values.
- **AC6** orchestration disabled ⇒ neither orchestrate nor worker present.
- **AC7** `expose:"essential"` tool count ≤ 12 with composites covering inspect/read/query.

## 7. Testing
- Round-trip tests: composite output == atomic output for every view/action (R8, plan §7.3).
- Schema test: enum advertised in `inputSchema` for `view`/`action`.
- Filter interaction test: essential vs all surface sets.

## 8. Risks
- **Behavioural drift** if a view forgets a flag → covered by round-trip tests.
- **Double surface under `all`** (atomic + composite) increases count short-term → acceptable; `essential`/phase modes are the reduction path.
