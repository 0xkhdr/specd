# specd — Command Reference

<!-- GENERATED FILE — do not edit by hand.
     Source of truth: internal/core/commands.go (the `specd help --json` palette, schema version 1).
     Regenerate with: go run ./tools/gendocs -->

> **Status:** Normative documentation for current `specd` behavior, generated from the
> command palette (`specd help --json`, schema version 1).

## Conventions

`specd <verb> [args] [flags]`. Run `specd help` for the live palette, or `specd help <verb>`
for one command. `specd help --json` emits the machine-readable palette this page is generated from.

Unknown verbs and disallowed flag values fail closed (exit 2). Deferred verbs print a notice and exit 0.

**Exit codes** (the standard convention; per-verb deviations are noted on the verb):

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | gate or verify failure |
| `2` | usage error or fail-closed rejection |

## Commands

### `adapters`

```
specd adapters [--json]
```

Inspect configured interoperability adapters read-only, distinguishing configured, missing, incompatible, and disabled without loading secrets or running anything.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable JSON. |

**Examples:**

```bash
specd adapters
specd adapters --json
```

### `agents`

```
specd agents [doctor | guide <slug>] [--json]
```

Inspect agent artifacts, diagnose prerequisites, or emit deterministic driver guidance without writing.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--compat` | bool | In `agents doctor`, report backward-compatibility diagnostics. |
| `--json` | bool | Emit JSON. |

**Examples:**

```bash
specd agents
specd agents doctor --json
specd agents guide payments --json
```

### `approve`

```
specd approve <spec>
```

Advance a spec exactly one lifecycle step after human approval and passing readiness gates.

**Phases:** any. **Human only.**

**Examples:**

```bash
specd approve payments
```

### `archive`

```
specd archive <spec> --successor <spec> --owner <owner> --evidence <ref>
```

Retire a spec from active context while preserving content hashes and successor provenance.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--evidence` | string | Audit evidence reference authorizing retirement. |
| `--owner` | string | Accountable archive owner. |
| `--successor` | string | Active successor spec receiving a supersedes link. |

**Examples:**

```bash
specd archive payments-v1 --successor payments-v2 --owner platform --evidence release:rel-7
```

### `brain`

```
specd brain <start|step|run|status|cancel|resume|claim|heartbeat|report|release> <spec> [args] [--authority]
```

Run the opt-in deterministic orchestration controller. Mission ids (the `claim` argument) are minted by brain dispatch and listed by `specd brain status` — never invented by a worker. A run that halts before dispatch exits 2; a run with no ready work exits 0.

**Phases:** analyze · plan · execute · verify · reflect.

| Flag | Value | Description |
|---|---|---|
| `--authority` | bool | Grant dispatch authority (fail-closed by default). |

**Examples:**

```bash
specd brain start payments --authority
specd brain claim payments payments.s1.T1 worker-1 craftsman
specd brain heartbeat payments <lease-id> worker-1
specd brain report payments <lease-id> worker-1
specd brain release payments payments.s1.T1
```

### `check`

```
specd check <spec> [--security] [--schema] [--schema-only] [--json]
```

Run the validation gate registry against a spec.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable findings. |
| `--schema` | bool | Validate state.json schema. |
| `--schema-only` | bool | Validate only state.json schema. |
| `--security` | bool | Run opt-in security gates. |

**Examples:**

```bash
specd check payments
specd check payments --security --json
```

### `clarification`

```
specd clarification <open|answer|withdraw|expire> <spec> [id] [flags]
```

Record an immutable clarification transition. Only a blocking task-scoped question affects that task's readiness.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--answer` | string | Answer text (required for answer). |
| `--blocking` | bool | Block the affected task's readiness until the question is resolved. |
| `--entity` | string | Entity the question is about as <spec|task|artifact>:<id>. Defaults to the spec. |
| `--json` | bool | Emit the appended record as JSON. |
| `--question` | string | Question to record (required for open). |
| `--reason` | string | Reason for a withdrawal or expiry. |

**Examples:**

```bash
specd clarification open payments --question 'which currency rounds up?' --entity task:T3 --blocking
specd clarification answer payments C1 --answer 'round half up'
```

### `complete-task`

```
specd complete-task <spec> <id>
```

Complete one task by consuming current passing evidence through the gated completion transaction.

**Phases:** analyze · plan · execute · verify · reflect.

| Flag | Value | Description |
|---|---|---|
| `--attestation-ref` | string | Optional external attestation reference. |
| `--cached-tokens` | string | Optional provider-neutral cached token count. |
| `--cost` | string | Optional worker-reported cost as a decimal string, stored verbatim. |
| `--currency` | string | Currency unit required with canonical cost. |
| `--duration-ms` | string | Optional worker-reported wall-clock milliseconds, stored verbatim. |
| `--input-tokens` | string | Optional provider-neutral input token count. |
| `--model` | string | Optional bounded model identifier. |
| `--nonce` | minted by `specd session action` | Single-use operation nonce. Spent on use; a replay is refused. |
| `--output-tokens` | string | Optional provider-neutral output token count. |
| `--pricing-ref` | string | Pricing reference required with canonical cost. |
| `--provider` | string | Optional bounded provider identifier. |
| `--session` | id minted by `specd session open` | Driver session id. Required while a session is open; mint the accompanying nonce with `specd session action`. |
| `--telemetry-source` | worker|provider_adapter|operator | Telemetry provenance. |
| `--tokens` | string | Optional worker-reported token count, stored verbatim. |

**Examples:**

```bash
specd complete-task payments T3
```

### `config`

```
specd config <show|validate|migrate> [--dry-run] [--source <project.yml|project.yaml>]
```

Inspect, validate, or explicitly migrate project configuration.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--dry-run` | bool | Preview migration operations without writing files. |
| `--source` | project.yml|project.yaml | Select one legacy spelling when both exist. |

**Examples:**

```bash
specd config show
specd config validate
specd config migrate --dry-run
specd config migrate --source project.yml
```

### `context`

```
specd context <slug> <task-id> [--json|--hud]
```

Build the bounded context manifest for a task.

**Phases:** analyze · plan · execute · verify · reflect.

| Flag | Value | Description |
|---|---|---|
| `--hud` | bool | Render the operator HUD (files, bytes, tokens, mode). |
| `--json` | bool | Emit machine-readable context. |

**Examples:**

```bash
specd context payments T3
specd context payments T3 --hud
```

### `decision`

```
specd decision <spec> --text <rationale> [--scope <scope>]
```

Record an explicit human decision.

**Phases:** any. **Human only.**

| Flag | Value | Description |
|---|---|---|
| `--scope` | string | Optional scope label. |
| `--text` | string | Decision rationale (required). |

**Examples:**

```bash
specd decision payments --text 'defer webhooks' --scope design
```

### `delegate`

```
specd delegate issue <spec> --grant <id> --transitions <t,...> | specd delegate revoke <grant> | specd delegate approve <spec> --grant <id> --token <t>
```

Operator-scoped delegation of approval authority: issue, revoke, or use a bounded grant. Delegated approval runs the same readiness gates as interactive approval and weakens none of them.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--expires-in` | string | Grant lifetime as a Go duration (e.g. 12h). |
| `--grant` | string | Grant identity to issue or use. |
| `--production` | bool | Permit production-profile transitions. Off unless asked for explicitly. |
| `--reason` | string | Why the delegation was used or revoked. |
| `--reason-required` | bool | Refuse a use of this grant that carries no reason. |
| `--token` | the bearer value printed once by `delegate issue` | Bearer token for the grant. Never stored in the repository. |
| `--transitions` | approve.<gate>[,approve.<gate>] | Exact transitions the grant may approve. No patterns. |
| `--uses` | string | Maximum number of approvals the grant authorizes. |

**Examples:**

```bash
specd delegate issue payments --grant nightly --transitions approve.design --uses 2 --expires-in 12h
specd delegate approve payments --grant nightly --token $SPECD_GRANT_TOKEN --reason "nightly unattended run"
specd delegate revoke nightly --reason "run finished"
```

### `deploy`

```
specd deploy <spec> --release <id> --environment <env> --adapter <a> --authority <auth> [--strategy <s>] [--population <p>] [--window <w>] [--idempotency-key <k>]
```

Append a monotonic deployment attempt to deployments.jsonl under the spec lock. Drives no external system.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--adapter` | string | Deployment adapter name (a reference; core drives nothing). |
| `--authority` | string | Authority under which the attempt is recorded. |
| `--environment` | string | Target environment (development|staging|production). |
| `--idempotency-key` | string | Caller-supplied idempotency key for the attempt. |
| `--population` | string | Target population label. |
| `--release` | string | Frozen release-candidate id to deploy. |
| `--strategy` | string | Rollout strategy label. |
| `--window` | string | Observation window label. |

**Examples:**

```bash
specd deploy payments --release a1b2c3 --environment staging --adapter shell --authority ci
```

### `drift`

```
specd drift <spec> [--json]
```

Project declared invariants and active decisions against local verify evidence without writing.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit stable JSON Lines. |

**Examples:**

```bash
specd drift payments
specd drift payments --json
```

### `drive`

```
specd drive <spec> [--json] [--sandbox]
```

Emit the single next-action envelope: session, revision, assurance, permitted actor, actor-tagged operations, handoffs, route blockers, selected task, authority, context digest, blockers, and the exact next command. A projection over the granular commands, which keep working unchanged.

**Phases:** analyze · plan · execute · verify · reflect.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit the machine-readable drive envelope. |
| `--sandbox` | bool | Declare that the invoking host isolates execution. Raises the reported assurance ceiling; absent, the session is advisory. |

**Examples:**

```bash
specd drive payments --json
specd drive payments --sandbox --json
```

### `eval`

```
specd eval <import|status> <spec> [artifact]
```

Import validated local eval evidence or inspect stored eval evidence.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--check` | check-id from the task's declared evidence cell (class/check-id; classes: test|output_eval|trajectory_eval|review) | Expected check identity for import. |
| `--json` | bool | Emit machine-readable JSON. |
| `--task` | task id from the spec's tasks.md `id` column | Expected task identity for import. |

**Examples:**

```bash
specd eval import payments adapter.jsonl --task T1
specd eval status payments --json
```

### `exception`

```
specd exception <approve|revoke> <finding> [governed exception fields]
```

Record or revoke a governed human security exception without changing lifecycle status.

**Phases:** any. **Human only.**

| Flag | Value | Description |
|---|---|---|
| `--action` | string | Governed exception action. |
| `--approver` | string | Governed exception approver. |
| `--control` | string | Governed exception control. |
| `--environment` | string | Governed exception environment. |
| `--expires-at` | string | Governed exception expires-at. |
| `--issued-at` | string | Governed exception issued-at. |
| `--owner` | string | Governed exception owner. |
| `--reason` | string | Governed exception reason. |
| `--revision` | string | Governed exception revision. |
| `--scope` | string | Governed exception scope. |
| `--ticket` | string | Governed exception ticket. |

**Examples:**

```bash
specd exception approve scanner-false-positive --reason 'reviewed false positive'
```

### `handshake`

```
specd handshake bootstrap [<spec>] [--json] [--expect-<identity> <value>]
```

Emit a complete, drift-safe bootstrap identity packet with executable next commands and separate handoffs.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--expect-binary-commit` | string | Fail if binary commit differs. |
| `--expect-binary-version` | string | Fail if binary version differs. |
| `--expect-config-digest` | string | Fail (exit 1) if the effective-config digest differs. |
| `--expect-context-schema` | string | Fail if context schema differs. |
| `--expect-managed-digest` | string | Fail if managed guidance differs. |
| `--expect-palette-digest` | string | Fail (exit 1) if the command-palette digest differs. |
| `--expect-revision` | string | Fail if state revision differs. |
| `--expect-root` | string | Fail if workspace root differs. |
| `--expect-spec` | string | Fail if active spec differs. |
| `--expect-state-schema` | string | Fail if state schema differs. |
| `--expect-template-schema` | string | Fail if template schema differs. |
| `--json` | bool | Emit machine-readable handshake. |

**Examples:**

```bash
specd handshake bootstrap
specd handshake bootstrap --json
```

### `help`

```
specd help [command] [--json]
```

Show command help.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable help. |

**Examples:**

```bash
specd help
specd help --json
```

### `incident`

```
specd incident seed <new-spec> --source-spec <spec> --release <id> --deployment <id> --criterion <id> --evidence-ref <ref[,ref]>
```

Seed a new spec from bounded delivery observation references without loading raw payloads.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--criterion` | string | Failed or observed health criterion. |
| `--deployment` | string | Source deployment identity. |
| `--evidence-ref` | string | Comma-separated bounded external references; queries, fragments, and raw payloads are refused. |
| `--release` | string | Source release identity. |
| `--source-spec` | string | Source spec owning the immutable delivery ledger. |

**Examples:**

```bash
specd incident seed checkout-recovery --source-spec checkout --release rel-7 --deployment dep-4 --criterion availability --evidence-ref obs://health/42
```

### `init`

```
specd init [--agent=<name>] [--repair|--refresh] [--dry-run]
```

Initialize or re-sync specd project state and managed assets.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--agent` | string | Select agent harness. |
| `--dry-run` | bool | Print the managed-region changes and write nothing. |
| `--refresh` | bool | Update managed regions to the current binary's template version. |
| `--repair` | bool | Restore drifted managed regions from the current templates. |

**Examples:**

```bash
specd init
specd init --agent=pinky
specd init --repair --dry-run
specd init --refresh
```

### `link`

```
specd link <from-slug> <to-slug> [--kind <kind>] [--reason <text>]
```

Record a typed, traceable cross-spec dependency link.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--kind` | follows|regresses|maintains|supersedes | Link kind (default: follows). |
| `--reason` | string | Optional human-authored reason stored with the link. |

**Examples:**

```bash
specd link api auth
specd link api-v2 api --kind supersedes --reason 'replace obsolete contract'
```

### `mcp`

```
specd mcp | specd mcp --config <host> [--root <path>] [--spec <slug>]
```

Serve the MCP integration surface over stdio, or print a host config snippet.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--config` | string | Print a paste-ready MCP config snippet for a host (e.g. claude-code). |
| `--root` | string | Pin the server working directory in the snippet. |
| `--spec` | string | Pin the active spec in the snippet. |

**Examples:**

```bash
specd mcp
specd mcp --config claude-code --spec demo
```

### `memory`

```
specd memory <slug> <add|promote> [flags]
```

Append or promote steering-memory patterns (learning flywheel).

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--body` | string | Detail of the pattern (add). |
| `--criticality` | minor|important|critical | minor|important|critical (add). |
| `--force` | bool | Promote past the threshold (promote). |
| `--key` | string | Pattern key (H2 heading). |
| `--pattern` | string | One-line pattern statement (add). |
| `--related` | string | Comma-separated related keys → wikilinks (add). |
| `--source` | string | Where the pattern came from (add). |

**Examples:**

```bash
specd memory payments add --key 'atomic writes' --pattern 'use AtomicWrite'
```

### `midreq`

```
specd midreq <spec> --text <change> [--scope <scope>]
```

Capture a scoped mid-stream requirement change.

**Phases:** any. **Human only.**

| Flag | Value | Description |
|---|---|---|
| `--scope` | string | Optional scope label. |
| `--text` | string | Change description (required). |

**Examples:**

```bash
specd midreq payments --text 'add refund path' --scope requirements
```

### `mode`

```
specd mode <spec> [orchestrated]
```

Read the current mode, or record human approval for the opt-in orchestration mode transition.

**Phases:** any. **Human only.**

**Examples:**

```bash
specd mode payments
specd mode payments orchestrated
```

### `new`

```
specd new <name> [--title <title>] [--agent=<name>]
```

Create a new spec workspace.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--agent` | string | Select agent harness. |
| `--title` | string | Optional human-readable spec title. |

**Examples:**

```bash
specd new payments
specd new payments --agent=codex
specd new payments --agent=pinky
```

### `next`

```
specd next <slug> [--json | --waves | --dispatch]
```

Select the next eligible task or wave.

**Phases:** analyze · plan · execute · verify · reflect.

| Flag | Value | Description |
|---|---|---|
| `--dispatch` | bool | Emit the context manifest for the first frontier task. |
| `--json` | bool | Emit machine-readable frontier list. |
| `--waves` | bool | Show all wave groups as JSON. |

**Examples:**

```bash
specd next payments
specd next payments --json
```

### `recurring`

```
specd recurring record <spec> --check <id> --head <sha> --release <id> --config <id> --verdict pass|fail --observed-at <RFC3339>
```

Validate and append an externally executed recurring-check result.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--check` | string | Recurring check identity. |
| `--config` | string | Tested configuration identity. |
| `--head` | string | Tested git HEAD. |
| `--observed-at` | string | Explicit RFC3339 observation time. |
| `--release` | string | Tested release identity. |
| `--verdict` | pass|fail | Check verdict. |

**Examples:**

```bash
specd recurring record payments --check api-health --head 0123456789012345678901234567890123456789 --release rel-7 --config prod-v3 --verdict pass --observed-at 2026-01-01T00:00:00Z
```

### `release`

```
specd release candidate <spec> --artifact-digest <d> --sbom-ref <r> --provenance-ref <r>
```

Freeze an immutable, reproducible release candidate identity into releases.jsonl. Builds and uploads nothing.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--artifact-digest` | string | Content digest of the already-built artifact (a reference; release never builds). |
| `--provenance-ref` | string | Reference to the artifact's provenance attestation. |
| `--sbom-ref` | string | Reference to the artifact's SBOM. |

**Examples:**

```bash
specd release candidate payments --artifact-digest sha256:abc --sbom-ref sbom://payments --provenance-ref prov://payments
```

### `reopen`

```
specd reopen <spec> task <id> --reason <text> --expect-revision <n> [--scope <paths>] [--revoke-lease <id>] | specd reopen <spec> artifact <requirements|design|tasks> --reason <text> --expect-revision <n> | specd reopen <spec> spec --reason <text> --expect-revision <n> | specd reopen <spec> descendant <id> <revalidate|retain|supersede|cancel> --reason <text> --expect-revision <n>
```

Open the next attempt of a completed, failed, or cancelled task, the next draft version of an unreleased artifact, or the next lifecycle cycle of an unreleased spec; prior-attempt evidence stops completing a reopened task and prior bytes are preserved as a content-addressed revision.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--expect-revision` | string | State revision the reopen was previewed against; a moved revision refuses and requires a fresh preview. |
| `--reason` | string | Required audit reason recorded on the attempt event. |
| `--revoke-lease` | string | Lease id the operator authorizes revoking inside this transaction; a live lease otherwise refuses the reopen. |
| `--scope` | string | Comma-separated bounded scope amendment approved inside this transaction, for repair that spans the task's declared files. |

**Examples:**

```bash
specd reopen payments task T7 --reason 'rounding defect found in review' --expect-revision 12
```

### `report`

```
specd report <spec> [--pr|--metrics|--efficiency|--rollup|--delivery|--outcome-review|--json|--history|--trace|--format prometheus|event] | specd report --portfolio
```

Render evidence-backed status, PR, history, trace, and metrics reports.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--compat-removal` | bool | Emit the read-only compatibility-removal readiness report. |
| `--delivery` | bool | Emit deterministic deployment status with adapter and trust source labeled separately. |
| `--efficiency` | bool | Emit deterministic context-efficiency report with explicit unknown values. |
| `--format` | prometheus|event | Alternate output format; event emits neutral local JSONL, prometheus emits metrics. |
| `--history` | bool | Replay the spec's audit trail from existing records in timestamp order. |
| `--json` | bool | Emit machine-readable report (JSON Lines with --history). |
| `--metrics` | bool | Emit metrics summary. |
| `--outcome-review` | bool | Join local change evidence to release and incident references, preserving missing outcomes as unknown. |
| `--portfolio` | bool | Emit deterministic cross-spec release/environment status and blockers from local ledgers. |
| `--pr` | bool | Emit PR-oriented report. |
| `--proof` | bool | Emit the deterministic evidence-proof report. |
| `--rollup` | bool | Emit exact cross-spec economic roll-up with explicit missing telemetry. |
| `--trace` | bool | Export the metadata-only run trace as stable JSON Lines. |
| `--workflow-metrics` | bool | Emit deterministic workflow-friction metrics from local ledgers. |

**Examples:**

```bash
specd report payments --pr
specd report payments --metrics
specd report payments --history
specd report payments --trace
specd report payments --format prometheus
specd report payments --format event
```

### `request-decision`

```
specd request-decision <spec> --text <deviation> [--scope <scope>]
```

Record an agent's request for a human decision. Records the request only; it advances no phase and writes no evidence.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--scope` | string | Optional scope label. |
| `--text` | string | Deviation the agent needs decided (required). |

**Examples:**

```bash
specd request-decision payments --text 'webhook retry needs a backoff not in the design' --scope design
```

### `review`

```
specd review <spec> [--force] [--restamp]
```

Scaffold the review report the auditor fills before completion. Existing reports refuse unless --restamp preserves their findings or --force preserves their exact bytes in review_report.md.bak before replacement.

**Phases:** execute · verify · reflect.

| Flag | Value | Description |
|---|---|---|
| `--force` | bool | Replace an existing report only after preserving its exact bytes in review_report.md.bak. |
| `--restamp` | bool | Update an existing report to a new git HEAD while preserving human findings. |

**Examples:**

```bash
specd review payments
specd review payments --force
specd review payments --restamp
```

### `session`

```
specd session <open|show|action|ack|close> <spec> [<task>] [--driver <host>] [--tokens <n>] [--json]
```

Manage the driver session that binds a host to one spec's mutable work. `action` mints the single-use nonce and bindings a mutable operation must carry; `ack` records the host's context receipt, without which mutable authority stays withheld.

**Phases:** analyze · plan · execute · verify · reflect.

| Flag | Value | Description |
|---|---|---|
| `--driver` | string | Host identity opening the session (required by `open`). |
| `--json` | bool | Emit machine-readable session packet. |
| `--partial` | bool | Acknowledge no required context lane, proving the withholding path (`ack`). |
| `--tokens` | string | Host-reported context token count recorded by `ack`. Recorded, never trusted as the harness estimate. |

**Examples:**

```bash
specd session open payments --driver claude-code --json
specd session ack payments T1 --tokens 4200
specd session action payments --json
specd session close payments
```

### `spike`

```
specd spike <spec> --question <q> --scope <s> --expiry <RFC3339> [--output <ref>]
```

Record a bounded exploratory spike (learning without a completion or approval bypass).

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--expiry` | string | RFC3339 instant after which the spike is stale (required, must be in the future). |
| `--output` | string | Optional reference to the spike's output (attaches to a decision; never satisfies task evidence). |
| `--question` | string | Bounded question the spike explores (required). |
| `--scope` | string | Bounded scope of the exploration (required). |

**Examples:**

```bash
specd spike payments --question 'is webhook retry idempotent?' --scope 'payments/webhook' --expiry 2026-07-19T00:00:00Z
```

### `status`

```
specd status [spec] [--json] | specd status <spec> --guide [--json] | specd status --program
```

Report current spec and task state, route-complete machine guidance with separate handoffs, or the cross-spec program view. JSON status includes parsed review verdict, note, reviewer, and HEAD when a report exists.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--guide` | bool | Emit machine driving guidance: phase, required artifact, legal commands, human-only actions, and blockers. |
| `--json` | bool | Emit machine-readable status, including parsed review metadata when present. |
| `--program` | bool | Show the cross-spec program view: specs, links, phases, and frontier. |

**Examples:**

```bash
specd status payments
specd status payments --json
specd status payments --guide --json
specd status --program
```

### `submit`

```
specd submit <spec> [--resubmit]
```

Run every gate, then stream the PR summary to the operator-configured submit command.

**Phases:** execute · verify · reflect.

| Flag | Value | Description |
|---|---|---|
| `--resubmit` | bool | Allow resubmitting a spec already submitted at the current git HEAD. |

**Examples:**

```bash
specd submit payments
specd submit payments --resubmit
```

### `task`

```
specd task <id> [--override --reason <text>]
```

Show task details or clear an escalated task with a human override.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable task row. |
| `--override` | bool | Clear an escalated task (resets the verify-failure ratchet; does not complete it). Requires --reason. |
| `--reason` | string | Human justification for --override (required, non-empty). |

**Examples:**

```bash
specd task T3 --json
specd task T3 --override --reason 'flaky infra, verified manually'
```

### `undo`

```
specd undo <spec> --reason <text> --expect-revision <n>
```

Compensate the latest unconsumed reversible workflow event by appending a compensation event; history is never deleted.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--expect-revision` | string | State revision the undo was previewed against; a moved revision refuses and requires a fresh preview. |
| `--reason` | string | Required audit reason recorded on the compensation event. |

**Examples:**

```bash
specd undo payments --reason 'wrong stage approved' --expect-revision 7
```

### `unlink`

```
specd unlink <from-slug> <to-slug>
```

Remove a cross-spec dependency link.

**Phases:** any.

**Examples:**

```bash
specd unlink api auth
```

### `verify`

```
specd verify <slug> <task-id> [--revert-on-fail] [--sandbox] [--sandbox-binary=<path>] | specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence <text>
```

Run and record task verification (task mode), or record a per-acceptance-criterion evidence record (--criterion mode).

**Phases:** analyze · plan · execute · verify · reflect.

| Flag | Value | Description |
|---|---|---|
| `--attestation-ref` | string | Optional external attestation reference. |
| `--cached-tokens` | string | Optional provider-neutral cached token count. |
| `--cost` | string | Optional worker-reported cost as a decimal string, stored verbatim. |
| `--criterion` | string | Record evidence for acceptance criterion <r>.<n> instead of running a task verify. |
| `--currency` | string | Currency unit required with canonical cost. |
| `--duration-ms` | string | Optional worker-reported wall-clock milliseconds, stored verbatim. |
| `--evidence` | string | Evidence text or path backing the criterion verdict (with --criterion). |
| `--input-tokens` | string | Optional provider-neutral input token count. |
| `--json` | bool | Emit machine-readable verification output. |
| `--model` | string | Optional bounded model identifier. |
| `--nonce` | minted by `specd session action` | Single-use operation nonce. Spent on use; a replay is refused. |
| `--output-tokens` | string | Optional provider-neutral output token count. |
| `--pricing-ref` | string | Pricing reference required with canonical cost. |
| `--provider` | string | Optional bounded provider identifier. |
| `--revert-on-fail` | bool | Restore working tree on verify failure. |
| `--sandbox` | bool | Run the verify line inside a bwrap sandbox (fail-closed if the binary is absent). |
| `--sandbox-binary` | string | Path to sandbox binary (overrides auto-detect). |
| `--session` | id minted by `specd session open` | Driver session id. Required while a session is open; mint the accompanying nonce with `specd session action`. |
| `--status` | pass|fail | Criterion verdict (with --criterion): pass|fail. |
| `--telemetry-source` | worker|provider_adapter|operator | Telemetry provenance. |
| `--tokens` | string | Optional worker-reported token count, stored verbatim. |

**Examples:**

```bash
specd verify payments T3
specd verify payments T3 --revert-on-fail
specd verify payments --criterion 1.2 --status pass --evidence 'covered by T3 integration test'
```

### `version`

```
specd version [--json]
```

Print build version metadata.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable JSON. |

**Examples:**

```bash
specd version
specd version --json
```
