# CLI Command Reference

> **One binary, 19 registered verbs** (18 active + 1 deferred stub). Every verb maps to
> exactly one harness component and one governing principle — see `docs/charter.md` for
> the full mapping. Exit codes: `0 = ok · 1 = error · 2 = usage/unknown command · 3 = not found`.

---

## Global usage

```
specd <command> [args] [--flag value | --flag=value]
```

No subcommand → prints the verb surface and exits 0.

---

## Lifecycle verbs

### `init`

**Component:** tools · **Principle:** P8 (Steering as Constitution)

```
specd init [--agent=<name>]
```

Scaffolds `.specd/` in the current directory:
- `.specd/roles/{scout,craftsman,validator,auditor}.md` — exactly four role-prompt files.
- `.specd/steering/{reasoning,workflow,product,tech,structure,memory}.md` — the steering constitution.
- Marker-merges `AGENTS.md` (replaces only the `<!-- specd:agents begin/end -->` section,
  preserving all user content outside the markers).

> **Note:** `--agent` is accepted but currently deferred (host-adapter wiring is Wave P5).

---

### `new`

**Component:** tools · **Principle:** P2 (Specs as Source of Truth)

```
specd new <name>
```

Creates a new spec workspace under `.specd/specs/<name>/`:

| File | Contents |
|---|---|
| `requirements.md` | EARS stub (edit before approving requirements) |
| `design.md` | Design stub (module boundaries / invariants) |
| `tasks.md` | Task table stub with one placeholder row |
| `memory.md` | Per-spec steering memory stub |
| `state.json` | Initial state — `status: requirements, revision: 0` |

Slug must match `^[a-z0-9][a-z0-9-]*$`. Refuses if spec already exists.

**Example:**
```bash
specd new payment-service
# → created spec payment-service at .specd/specs/payment-service
```

---

### `approve`

**Component:** guardrails · **Principle:** P6 (Human Gates at Phase Boundaries)

```
specd approve <spec> <gate>
```

Records a human approval at a lifecycle gate. Valid gates:
`requirements | design | tasks | executing | verifying | complete`.

**What it does:**
1. Runs the full gate registry against the spec (equivalent to `specd check <spec>`).
2. If any gate emits an `error` finding → refuses the approval, prints failures, exits 1.
3. On green: advances the spec's `status` + `phase` (forward-only ratchet) under lock+CAS,
   appends `approval:<gate>` record with timestamp, git HEAD, and actor.

**Example:**
```bash
specd approve payment-service requirements
# → approved payment-service → requirements
```

> **Agents cannot call `approve` over MCP.** This is a ForbiddenTool by policy.

---

### `next`

**Component:** orchestration · **Principle:** P4 (Waves, Not Lines)

```
specd next <slug> [--json | --waves | --dispatch]
```

Queries the task DAG for the runnable frontier — tasks whose dependencies are all complete
and which are not themselves complete or blocked.

| Flag | Behavior |
|---|---|
| *(default)* | Prints task IDs, one per line |
| `--json` | Emits the frontier as a JSON array of task rows |
| `--waves` | Emits the full wave-grouped task table as JSON (does not require approval gate) |
| `--dispatch` | Emits the context manifest for the first frontier task (for orchestration use) |

Requires requirements **and** design to be approved (reads `approval:requirements` +
`approval:design` from `state.records`) unless `--waves` is used. Exits 1 if those
approvals are missing.

**Example:**
```bash
specd next payment-service --json
```

---

### `verify`

**Component:** guardrails · **Principle:** P3 (Evidence Gates Every State Change)

```
specd verify <slug> <task-id> [--revert-on-fail] [--sandbox] [--sandbox-binary=<path>]
```

Runs the task's `verify:` command via `sh -c` in a **scrubbed environment**
(allowlist: `PATH, HOME, LANG, LC_ALL, TMPDIR, SPECD_*`). Appends an evidence record
to `evidence.jsonl` regardless of exit code — even failures are recorded.

| Flag | Behavior |
|---|---|
| `--revert-on-fail` | Snapshots `git diff --binary` before run; restores working tree on failure |
| `--sandbox` | Run inside bwrap/container sandbox (`config.verify.sandbox`) |
| `--sandbox-binary=<path>` | Path to the sandbox binary (overrides auto-detect) |

**Evidence record shape:**
```json
{ "task_id": "T3", "command": "go test ./...", "exit_code": 0, "git_head": "abc1234…" }
```

> **Warning printed if git HEAD cannot be resolved.** Evidence with `git_head: "unknown"`
> cannot count toward `task complete`.

**Example:**
```bash
specd verify payment-service T3
specd verify payment-service T3 --revert-on-fail
```

---

### `task`

**Component:** observability · **Principle:** P2

```
specd task <id> [--json]
specd task complete <spec> <id>
```

**`specd task <id>`** — prints the parsed task row for `<id>` (searches all specs in `.specd/specs/`).
With `--json` emits a machine-readable task object.

**`specd task complete <spec> <id>`** — marks a task complete:
1. Requires a **passing** evidence record in `evidence.jsonl` pinned to a real git HEAD.
2. Writes `✅` marker to `tasks.md` (byte-stable single-line rewrite).
3. Updates `state.json` task status map under lock+CAS.
4. Both writes are atomic — they cannot drift.

**Example:**
```bash
specd task T3
specd task T3 --json
specd task complete payment-service T3
```

---

### `check`

**Component:** guardrails · **Principle:** P1 (Foundational Split)

```
specd check <slug> [--security] [--json]
```

Runs the pluggable gate registry against the spec. Core gates always run; the security
gate is opt-in.

Output (default): `<severity> <gate>: <message>` per finding.
With `--json`: JSON array of findings.
Exit code 1 if any finding has severity `error`.

**Example:**
```bash
specd check payment-service
specd check payment-service --security --json
```

---

## Decision & change-capture verbs

### `decision`

**Component:** guardrails · **Principle:** P6

```
specd decision <spec> --text "<rationale>" [--scope <scope>]
```

Appends a human decision record to `state.records`. `--text` is **required** — a blank
decision captures nothing. Stamped with timestamp, git HEAD, and actor.

**Example:**
```bash
specd decision payment-service --text "Chose PostgreSQL over SQLite for multi-writer support." --scope "data-layer"
```

---

### `midreq`

**Component:** guardrails · **Principle:** P6

```
specd midreq <spec> --text "<change>" [--scope <scope>]
```

Records a scoped mid-stream requirement change without restarting the lifecycle.
`--text` is **required**. High/critical midreq gates are never auto-cleared; they require
explicit human review.

**Example:**
```bash
specd midreq payment-service --text "Add idempotency key support to POST /payments." --scope "api"
```

---

## Observability verbs

### `status`

**Component:** observability · **Principle:** P7 (Deterministic Reporting)

```
specd status <slug> [--json]
```

Deterministic projection of `state.json` — never LLM-generated. Prints phase,
status, task completion summary, and open decisions/midreqs.

With `--json`: emits the full `ReportModel` plus `records` (raw, never re-synthesized).

---

### `report`

**Component:** observability · **Principle:** P7

```
specd report <slug> [--pr | --metrics | --json]
```

| Flag | Output |
|---|---|
| *(default)* | Same as `status` |
| `--pr` | PR-oriented Markdown summary (suitable for GitHub PR description) |
| `--metrics` | Prometheus textfile exposition |
| `--json` | Full `ReportModel` as JSON |

All projections are computed from `state.json` + `evidence.jsonl`. No network, no LLM.

---

## Context & memory verbs

### `context`

**Component:** context · **Principle:** P2

```
specd context <slug> <task-id> [--json | --hud]
```

Builds the bounded context manifest for a task — the minimum information the assigned
role needs to execute the task, assembled from on-disk files via the four disclosure modes.

| Flag | Output |
|---|---|
| *(default)* | File paths, one per line |
| `--json` | Full manifest with items, modes, and token estimates |
| `--hud` | Operator HUD: files, bytes, tokens, mode summary |

Token budget: `SPECD_CONTEXT_MAX_TOKENS` / `context.max_tokens` (default: 12000).

> `--json` and `--hud` are mutually exclusive; using both returns an error.

---

### `memory`

**Component:** context · **Principle:** P8 (Steering as Constitution)

```
specd memory <slug> add   --key <key> --pattern <text> --body <detail> --source <from> --criticality <minor|important|critical> [--related <keys>]
specd memory <slug> promote --key <key> [--force]
```

Appends or promotes steering-memory patterns to the per-spec `memory.md`. Patterns that
cross the promotion threshold are elevated to the global steering `memory.md`.
The `--force` flag overrides the threshold.

---

## Integration verbs

### `handshake`

**Component:** instructions · **Principle:** P5 (Agent-Agnostic by Design)

```
specd handshake bootstrap [--json]
```

Emits bootstrap/policy material for agent onboarding: schema version, the effective tool
palette, and the policy digest. This is the universal on-ramp for any host.

---

### `mcp`

**Component:** tools · **Principle:** P5

```
specd mcp
```

Starts a stdio JSON-RPC 2.0 MCP server. The tool set is data-driven from `Commands[]` so
tool registration and help cannot drift. Per-spec tool policy is read from `.specd/manifest.json`
(`required/optional/forbidden`); malformed manifest degrades to **empty policy, not open**.

---

## Orchestration verbs (opt-in)

### `brain`

**Component:** orchestration · **Principle:** P4

```
specd brain start  <slug>
specd brain step   <slug>
specd brain run    <slug>
specd brain status <slug>
specd brain approve <slug>
specd brain cancel  <slug>
```

The deterministic wave controller. `Decide(Snapshot) → Decision` is a pure function —
zero IO, zero randomness, zero LLM calls. Actions: `dispatch | wait | await-approval |
escalate | policy-violation | complete`.

**Fail-closed:** refuses to start unless `orchestration.enabled: true` in config
**and** the spec's mode is `orchestrated`. When disabled, the entire orchestration tier
is inert — CLI output and `check` output are byte-identical to the disabled state.

| Flag | Behavior |
|---|---|
| `--authority` | Grant dispatch authority (fail-closed by default) |

---

## Deferred verbs

### `triage` *(deferred)*

```
specd triage <spec>
```

Registered stub — prints `specd triage: deferred — not yet wired` and exits 0.
Planned for the extended-loop triage tier (review-specs W5 will resolve the count
by subtraction, not by wiring the stub). See [open finding F7](../PROJECT.md).

---

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success |
| `1` | Error (gate failure, missing evidence, lock timeout, etc.) |
| `2` | Usage error / unknown command |
| `3` | Spec root not found (`.specd/` not in directory or any parent) |
