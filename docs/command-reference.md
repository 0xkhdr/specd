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
| `--json` | bool | Emit JSON. |

**Examples:**

```bash
specd agents
specd agents doctor --json
specd agents guide payments --json
```

### `approve` — human only

```
specd approve <spec>
```

Advance a spec exactly one lifecycle step after human approval and passing readiness gates.

**Phases:** any.

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
specd brain <start|step|run|status|cancel|resume|claim|heartbeat|report> <spec> [args] [--authority]
```

Run the opt-in deterministic orchestration controller. Mission ids (the `claim` argument) are minted by brain dispatch and listed by `specd brain status` — never invented by a worker.

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
```

### `check`

```
specd check <spec> [--security] [--json]
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
| `--output-tokens` | string | Optional provider-neutral output token count. |
| `--pricing-ref` | string | Pricing reference required with canonical cost. |
| `--provider` | string | Optional bounded provider identifier. |
| `--telemetry-source` | string | Telemetry provenance. |
| `--tokens` | string | Optional worker-reported token count, stored verbatim. |

**Examples:**

```bash
specd complete-task payments T3
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

### `decision` — human only

```
specd decision <spec> --text <rationale> [--scope <scope>]
```

Record an explicit human decision.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--scope` | string | Optional scope label. |
| `--text` | string | Decision rationale (required). |

**Examples:**

```bash
specd decision payments --text 'defer webhooks' --scope design
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

### `exception` — human only

```
specd exception <approve|revoke> <finding> [governed exception fields]
```

Record or revoke a governed human security exception without changing lifecycle status.

**Phases:** any.

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

Emit a complete, drift-safe bootstrap identity packet.

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
| `--kind` | string | Link kind (default: follows). |
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
| `--criticality` | string | minor|important|critical (add). |
| `--force` | bool | Promote past the threshold (promote). |
| `--key` | string | Pattern key (H2 heading). |
| `--pattern` | string | One-line pattern statement (add). |
| `--related` | string | Comma-separated related keys → wikilinks (add). |
| `--source` | string | Where the pattern came from (add). |

**Examples:**

```bash
specd memory payments add --key 'atomic writes' --pattern 'use AtomicWrite'
```

### `midreq` — human only

```
specd midreq <spec> --text <change> [--scope <scope>]
```

Capture a scoped mid-stream requirement change.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--scope` | string | Optional scope label. |
| `--text` | string | Change description (required). |

**Examples:**

```bash
specd midreq payments --text 'add refund path' --scope requirements
```

### `mode` — human only

```
specd mode <spec> orchestrated
```

Record human approval for the separate opt-in orchestration mode transition.

**Phases:** any.

**Examples:**

```bash
specd mode payments orchestrated
```

### `new`

```
specd new <name> [--agent=<name>]
```

Create a new spec workspace.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--agent` | string | Select agent harness. |

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

### `report`

```
specd report <spec> [--pr|--metrics|--efficiency|--rollup|--delivery|--outcome-review|--json|--history|--trace|--format prometheus|event|otel] | specd report --portfolio
```

Render evidence-backed status, PR, history, trace, and metrics reports.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--delivery` | bool | Emit deterministic deployment status with adapter and trust source labeled separately. |
| `--efficiency` | bool | Emit deterministic context-efficiency report with explicit unknown values. |
| `--format` | string | Alternate output format; event emits neutral local JSONL, prometheus emits metrics, otel emits adapter-mapped spans. |
| `--history` | bool | Replay the spec's audit trail from existing records in timestamp order. |
| `--json` | bool | Emit machine-readable report (JSON Lines with --history). |
| `--metrics` | bool | Emit metrics summary. |
| `--outcome-review` | bool | Join local change evidence to release and incident references, preserving missing outcomes as unknown. |
| `--portfolio` | bool | Emit deterministic cross-spec release/environment status and blockers from local ledgers. |
| `--pr` | bool | Emit PR-oriented report. |
| `--rollup` | bool | Emit exact cross-spec economic roll-up with explicit missing telemetry. |
| `--trace` | bool | Export the metadata-only run trace as stable JSON Lines. |

**Examples:**

```bash
specd report payments --pr
specd report payments --metrics
specd report payments --history
specd report payments --trace
specd report payments --format prometheus
specd report payments --format otel
```

### `review`

```
specd review <spec> [--force]
```

Scaffold the review report the auditor fills before completion.

**Phases:** execute · verify · reflect.

| Flag | Value | Description |
|---|---|---|
| `--force` | bool | Overwrite an existing report for the current git HEAD. |

**Examples:**

```bash
specd review payments
specd review payments --force
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

Report current spec and task state, machine driving guidance, or the cross-spec program view.

**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--guide` | bool | Emit machine driving guidance: phase, required artifact, legal commands, human-only actions, and blockers. |
| `--json` | bool | Emit machine-readable status. |
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
| `--model` | string | Optional bounded model identifier. |
| `--output-tokens` | string | Optional provider-neutral output token count. |
| `--pricing-ref` | string | Pricing reference required with canonical cost. |
| `--provider` | string | Optional bounded provider identifier. |
| `--revert-on-fail` | bool | Restore working tree on verify failure. |
| `--sandbox` | bool | Run the verify line inside a bwrap sandbox (fail-closed if the binary is absent). |
| `--sandbox-binary` | string | Path to sandbox binary (overrides auto-detect). |
| `--status` | pass|fail | Criterion verdict (with --criterion): pass|fail. |
| `--telemetry-source` | string | Telemetry provenance. |
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
