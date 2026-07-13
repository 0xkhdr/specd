# specd — Command Reference

> **Source of truth.** Every verb, flag, exit code, and allowed phase on this page is
> generated to match `internal/core/commands.go` (`var Commands`, `HelpSchemaVersion 1`).
> `docs/CHEATSHEET.md` is a byte-identical copy of this file; `scripts/docs-lint.sh`
> fails CI if they drift. Edit this file, then copy it over the cheatsheet.

`specd <verb> [args] [flags]`. Run `specd help` for the live palette or
`specd help <verb>` for one command. `specd help --json` emits the machine-readable
palette (`schema_version` + `commands[]`) that dispatch, MCP, and role prompts pin against.

## Conventions

**Exit codes** (every verb, unless noted):

| Code | Meaning |
|---|---|
| `0` | success |
| `1` | gate or verify failure |
| `2` | usage error or fail-closed rejection |

Unknown verbs and disallowed flag values **fail closed (exit 2)**. A verb run outside its
allowed lifecycle phase is rejected (exit 2). Deferred verbs print a deferral notice and
exit 0 — they never silently no-op.

**Phase enforcement.** A verb that resolves a spec is checked against that spec's current
phase. `any` = valid in every phase. `post-requirements` = `analyze · plan · execute ·
verify · reflect` (fails closed while a spec is still in the `perceive`/requirements phase).
`post-execution` = `execute · verify · reflect` (terminal verbs need completed work to act on).

---

## Lifecycle

### `init`
```
specd init [--agent=<name>] [--repair|--refresh] [--dry-run]
```
Initialize or re-sync specd project state and managed assets. Scaffolds `.specd/`, writes
`AGENTS.md`, and a commented `project.yml` (with an active `verify.timeout_seconds: 600`
bound) into the project root; an existing `project.yml` is never overwritten. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--agent` | string | Select agent harness. |
| `--repair` | bool | Restore drifted managed regions from the current templates. |
| `--refresh` | bool | Update managed regions to the current binary's template version. |
| `--dry-run` | bool | Print the managed-region changes and write nothing. |

```bash
specd init
specd init --agent=pinky
specd init --repair --dry-run
specd init --refresh
```

### `agents`
```
specd agents [doctor | guide <slug>] [--json]
```
Inspect installed agent artifacts, run read-only diagnostics with `doctor`, or emit deterministic driver actions with `guide`. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit JSON. |

```bash
specd agents
specd agents doctor --json
specd agents guide payments --json
```

### `new`
```
specd new <name> [--agent=<name>]
```
Create a new spec workspace. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--agent` | string | Select agent harness. |

```bash
specd new payments
specd new payments --agent=codex
specd new payments --agent=pinky
```

### `approve`
```
specd approve <spec> <gate>
```
Record human approval for a lifecycle gate. Advances a phase only when the gate registry
passes. The `orchestrated` gate enters orchestrated mode through a state CAS when
`orchestration.enabled: true`; it does not change lifecycle status. **Phases:** any.

`specd approve exception <approve|revoke> <finding> [governed exception fields]` appends an
immutable governed exception lifecycle record. Every field is required; evidence integrity and
worker authority cannot be waived.

```bash
specd approve payments requirements
specd approve payments design
specd approve payments orchestrated
```

### `midreq`
```
specd midreq <spec> --text <change> [--scope <scope>]
```
Capture a scoped mid-stream requirement change. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--text` | string | Change description (required). |
| `--scope` | string | Optional scope label. |

```bash
specd midreq payments --text 'add refund path' --scope requirements
```

### `decision`
```
specd decision <spec> --text <rationale> [--scope <scope>]
```
Record an explicit human decision. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--text` | string | Decision rationale (required). |
| `--scope` | string | Optional scope label. |

```bash
specd decision payments --text 'defer webhooks' --scope design
```

### `spike`
```
specd spike <spec> --question <q> --scope <s> --expiry <RFC3339> [--output <ref>]
```
Record a bounded exploratory spike (learning without a completion or approval bypass). **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--question` | string | Bounded question the spike explores (required). |
| `--scope` | string | Bounded scope of the exploration (required). |
| `--expiry` | string | RFC3339 instant after which the spike is stale (required, must be in the future). |
| `--output` | string | Optional reference to the spike's output (attaches to a decision; never satisfies task evidence). |

A spike attaches learning to a spec without authorizing anything: it never completes a task
(that still requires a passing verify record) and never approves architecture (that still
requires a human design approval).

```bash
specd spike payments --question 'is webhook retry idempotent?' --scope 'payments/webhook' --expiry 2026-07-19T00:00:00Z
```

---

## Execution

### `next`
```
specd next <slug> [--json | --waves | --dispatch]
```
Select the next eligible task or wave. **Phases:** post-requirements.

| Flag | Value | Description |
|---|---|---|
| `--waves` | bool | Show all wave groups as JSON. |
| `--dispatch` | bool | Emit the context manifest for the first frontier task. |
| `--json` | bool | Emit machine-readable frontier list. |

```bash
specd next payments
specd next payments --json
```

### `task`
```
specd task <id> [--override --reason <text>] | specd task complete <spec> <id>
```
Show task details, clear an escalated task with a human override, or mark a task complete
(requires passing evidence). **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable task row. |
| `--override` | bool | Clear an escalated task (resets the verify-failure ratchet; does not complete it). Requires `--reason`. |
| `--reason` | string | Human justification for `--override` (required, non-empty). |
| `--tokens` | string | Optional worker-reported token count, stored verbatim (`task complete`). |
| `--cost` | string | Optional worker-reported cost as a decimal string, stored verbatim (`task complete`). |
| `--duration-ms` | string | Optional worker-reported wall-clock milliseconds, stored verbatim (`task complete`). |
| `--input-tokens` | string | Optional provider-neutral input token count. |
| `--output-tokens` | string | Optional provider-neutral output token count. |
| `--cached-tokens` | string | Optional provider-neutral cached token count. |
| `--provider` | string | Optional bounded provider identifier; never a metric label. |
| `--model` | string | Optional bounded model identifier; never a metric label. |
| `--currency` | string | Currency unit required with canonical cost. |
| `--pricing-ref` | string | Pricing reference required with canonical cost. |
| `--telemetry-source` | `worker`\|`provider_adapter`\|`operator` | Telemetry provenance. |
| `--attestation-ref` | string | Optional external attestation reference. |

```bash
specd task T3 --json
specd task T3 --override --reason 'flaky infra, verified manually'
specd task complete payments T3
```

### `check`
```
specd check <spec> [--security] [--json]
```
Run the validation gate registry against a spec. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--security` | bool | Run opt-in security gates. These scan selected tracked files; they do not yet scan `.specd/` or enforce role capability, declared-file diff scope, or mandatory sandboxing. |
| `--schema` | bool | Validate `state.json` schema. |
| `--schema-only` | bool | Validate only `state.json` schema. |
| `--json` | bool | Emit machine-readable findings. |

```bash
specd check payments
specd check payments --security --json
```

### `verify`
```
specd verify <slug> <task-id> [--revert-on-fail] [--sandbox] [--sandbox-binary=<path>]
specd verify <slug> --criterion <r>.<n> --status pass|fail --evidence <text>
```
Run and record task verification (task mode), or record a per-acceptance-criterion evidence
record (`--criterion` mode). A task completes **only** against a passing verify record
(exit 0 pinned to a resolvable git HEAD). **Phases:** post-requirements.

| Flag | Value | Description |
|---|---|---|
| `--revert-on-fail` | bool | Restore working tree on verify failure. |
| `--sandbox` | bool | Run the verify line inside a bwrap sandbox (fail-closed if the binary is absent). |
| `--sandbox-binary` | string | Path to sandbox binary (overrides auto-detect). |
| `--criterion` | string | Record evidence for acceptance criterion `<r>.<n>` instead of running a task verify. |
| `--status` | `pass`\|`fail` | Criterion verdict (with `--criterion`). |
| `--evidence` | string | Evidence text or path backing the criterion verdict (with `--criterion`). |
| `--tokens` | string | Optional worker-reported token count, stored verbatim. |
| `--cost` | string | Optional worker-reported cost as a decimal string, stored verbatim. |
| `--duration-ms` | string | Optional worker-reported wall-clock milliseconds, stored verbatim. |
| `--input-tokens` | string | Optional provider-neutral input token count. |
| `--output-tokens` | string | Optional provider-neutral output token count. |
| `--cached-tokens` | string | Optional provider-neutral cached token count. |
| `--provider` | string | Optional bounded provider identifier; never a metric label. |
| `--model` | string | Optional bounded model identifier; never a metric label. |
| `--currency` | string | Currency unit required with canonical cost. |
| `--pricing-ref` | string | Pricing reference required with canonical cost. |
| `--telemetry-source` | `worker`\|`provider_adapter`\|`operator` | Telemetry provenance. |
| `--attestation-ref` | string | Optional external attestation reference. |

```bash
specd verify payments T3
specd verify payments T3 --revert-on-fail
specd verify payments --criterion 1.2 --status pass --evidence 'covered by T3 integration test'
```

### `context`
```
specd context <slug> <task-id> [--json|--hud]
```
Build the bounded context manifest for a task. **Phases:** post-requirements.

Manifest schema **v1** remains compatibility output; hosts opt into typed **v2** additively. V2
requires requirements/design/role/source lanes, canonical ordering, source digests, selected-task
identity, and driver route/capability identity. Unknown schema/item/trust values, missing lanes,
route mismatch, stale receipt, or required-budget overflow fail closed. Optional omissions carry a
reason. Receipts contain digests/totals/provenance only; skills and memory remain untrusted
advisory data and cannot widen authority.

When task quality declarations exist, context also exposes compact class/check IDs, verify,
artifact refs/digests, subject revision, freshness, and dataset/rubric/output/trace digests.
Packet contains metadata only; raw datasets, outputs, and traces remain external.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable context. |
| `--hud` | bool | Render the operator HUD (files, bytes, tokens, mode). |

```bash
specd context payments T3
specd context payments T3 --hud
```

### `review`
```
specd review <spec> [--force]
```
Scaffold the review report the auditor fills before completion. **Phases:** post-execution.

| Flag | Value | Description |
|---|---|---|
| `--force` | bool | Overwrite an existing report for the current git HEAD. |

```bash
specd review payments
specd review payments --force
```

### `submit`
```
specd submit <spec> [--resubmit]
```
Run every gate, then stream the PR summary to the operator-configured submit command.
**Phases:** post-execution.

| Flag | Value | Description |
|---|---|---|
| `--resubmit` | bool | Allow resubmitting a spec already submitted at the current git HEAD. |

```bash
specd submit payments
specd submit payments --resubmit
```

---

## Delivery

Release and deployment ledgers are an additive, offline domain. They never build,
upload, or drive an external system, and they never change task evidence or
`complete` — the evidence gate passes or fails identically whether these ledgers
are present or absent. Both verbs are kept out of the general MCP palette.

### `release`
```
specd release candidate <spec> --artifact-digest <d> --sbom-ref <r> --provenance-ref <r>
```
Freeze an immutable, reproducible release-candidate identity — spec revision, git
HEAD, evidence-set digest, artifact digest, SBOM/provenance refs, and bootstrap
digest — into `.specd/specs/<spec>/releases.jsonl`. The candidate id is a content
address of those inputs, so identical inputs re-freeze idempotently. Builds and
uploads nothing. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--artifact-digest` | string | Content digest of the already-built artifact (a reference; release never builds). |
| `--sbom-ref` | string | Reference to the artifact's SBOM. |
| `--provenance-ref` | string | Reference to the artifact's provenance attestation. |

```bash
specd release candidate payments --artifact-digest sha256:abc --sbom-ref sbom://payments --provenance-ref prov://payments
```

### `deploy`
```
specd deploy <spec> --release <id> --environment <env> --adapter <a> --authority <auth> [--strategy <s>] [--population <p>] [--window <w>] [--idempotency-key <k>]
```
Append a monotonic deployment attempt to `.specd/specs/<spec>/deployments.jsonl`
under the spec lock. The attempt binds the frozen candidate's git HEAD and
artifact digest, so it can never claim an artifact the release did not freeze.
Retries of the same release into the same environment share a deployment id and
accrue monotonic attempts; a crash yields the prior complete record or one
complete new record, never a partial or duplicate attempt. Drives no external
system. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--release` | string | Frozen release-candidate id to deploy. |
| `--environment` | string | Target environment (development\|staging\|production). |
| `--adapter` | string | Deployment adapter name (a reference; core drives nothing). |
| `--authority` | string | Authority under which the attempt is recorded. |
| `--strategy` | string | Rollout strategy label. |
| `--population` | string | Target population label. |
| `--window` | string | Observation window label. |
| `--idempotency-key` | string | Caller-supplied idempotency key for the attempt. |

```bash
specd deploy payments --release a1b2c3 --environment staging --adapter shell --authority ci
```

---

## Inspection

### `adapters`
```
specd adapters [--json]
```
Inspect configured interoperability adapters read-only, distinguishing configured, missing, incompatible, and disabled without loading secrets or running anything. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable JSON. |

```bash
specd adapters
specd adapters --json
```

### `eval`
```
specd eval <import|status> <spec> [artifact]
```
Import validated local adapter evidence or inspect stored eval evidence. Import never runs an
adapter or contacts a provider. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable JSON for status. |
| `--task` | string | Expected task identity for import. |
| `--check` | string | Expected check identity for import. |

```bash
specd eval import payments adapter.jsonl --task T1 --check rubric-v1
specd eval status payments --json
```

### `help`
```
specd help [command] [--json]
```
Show command help. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable help. |

```bash
specd help
specd help --json
```

### `version`
```
specd version [--json]
```
Print build version metadata. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable JSON. |

```bash
specd version
specd version --json
```

### `status`
```
specd status [spec] [--json] | specd status <spec> --guide [--json] | specd status --program
```
Report current spec and task state, machine driving guidance, or the cross-spec program view. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable status. |
| `--guide` | bool | Emit machine driving guidance: phase, required artifact, legal commands, human-only actions, and blockers. |
| `--program` | bool | Show the cross-spec program view: specs, links, phases, and frontier. |

```bash
specd status payments
specd status payments --json
specd status payments --guide --json
specd status --program
```

### `report`
```
specd report <spec> [--pr|--metrics|--efficiency|--rollup|--delivery|--json|--history|--trace|--proof|--format prometheus|event|otel]
```
Render evidence-backed status, PR, history, trace, proof, and metrics reports. Deterministic —
generated from `state.json` + task artifacts, never from an LLM. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--pr` | bool | Emit PR-oriented report. |
| `--metrics` | bool | Emit metrics summary. |
| `--efficiency` | bool | Emit deterministic context-efficiency data with explicit `unknown` measurements. |
| `--rollup` | bool | Emit exact cross-spec economic totals, preserving missing telemetry separately from measured zero. |
| `--delivery` | bool | Emit byte-stable deployment status with adapter and trust source labeled separately. |
| `--json` | bool | Emit machine-readable report (JSON Lines with `--history`). |
| `--history` | bool | Replay the spec's audit trail from existing records in timestamp order. |
| `--trace` | bool | Export the metadata-only run trace as stable JSON Lines. |
| `--proof` | bool | Emit the lifecycle proof: coverage, stale records, amendments, escaped-defect links. |
| `--format` | `prometheus`, `event`, `otel` | Alternate output format; event emits neutral local JSONL, prometheus emits metrics, otel emits adapter-mapped spans. |

```bash
specd report payments --pr
specd report payments --metrics
specd report payments --history
specd report payments --trace
specd report payments --proof
specd report payments --efficiency
specd report payments --rollup
specd report payments --delivery
specd report payments --format event
specd report payments --format prometheus
specd report payments --format otel
```

---

## Integration

### `memory`
```
specd memory <slug> <add|promote> [flags]
```
Append or promote steering-memory patterns (learning flywheel). **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--key` | string | Pattern key (H2 heading). |
| `--pattern` | string | One-line pattern statement (`add`). |
| `--body` | string | Detail of the pattern (`add`). |
| `--source` | string | Where the pattern came from (`add`). |
| `--criticality` | `minor`\|`important`\|`critical` | Criticality (`add`). |
| `--related` | string | Comma-separated related keys → wikilinks (`add`). |
| `--force` | bool | Promote past the threshold (`promote`). |

```bash
specd memory payments add --key 'atomic writes' --pattern 'use AtomicWrite'
```

### `mcp`
```
specd mcp | specd mcp --config <host> [--root <path>] [--spec <slug>]
```
Serve the MCP integration surface over stdio, or print a host config snippet. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--config` | string | Print a paste-ready MCP config snippet for a host (e.g. `claude-code`). |
| `--root` | string | Pin the server working directory in the snippet. |
| `--spec` | string | Pin the active spec in the snippet. |

```bash
specd mcp
specd mcp --config claude-code --spec demo
```

MCP hosts declare driver capabilities during `initialize` with `driver_capabilities`:
`context_loading`, `sandbox`, `telemetry`, `eval`, and `a2a`. The response reports every
capability as `supported`, `downgraded`, or `refused`; missing declarations never disappear.
Missing sandbox refuses mutable execution and names read-only recovery. Other missing features
downgrade to deterministic local behavior.

### `handshake`
```
specd handshake bootstrap [<spec>] [--json] [--expect-<identity> <value>]
```
Emit a complete, drift-safe bootstrap identity packet. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable handshake. |
| `--expect-palette-digest` | string | Fail (exit 1) if the command-palette digest differs. |
| `--expect-config-digest` | string | Fail (exit 1) if the effective-config digest differs. |
| `--expect-managed-digest` | string | Fail if managed guidance differs. |
| `--expect-binary-version` | string | Fail if binary version differs. |
| `--expect-binary-commit` | string | Fail if binary commit differs. |
| `--expect-state-schema` | string | Fail if state schema differs. |
| `--expect-context-schema` | string | Fail if context schema differs. |
| `--expect-template-schema` | string | Fail if template schema differs. |
| `--expect-root` | string | Fail if workspace root differs. |
| `--expect-spec` | string | Fail if active spec differs. |
| `--expect-revision` | string | Fail if state revision differs. |

```bash
specd handshake bootstrap
specd handshake bootstrap --json
```

### `link`
```
specd link <from-slug> <to-slug>
```
Record that one spec depends on another (cross-spec ordering). **Phases:** any.

```bash
specd link api auth
```

### `unlink`
```
specd unlink <from-slug> <to-slug>
```
Remove a cross-spec dependency link. **Phases:** any.

```bash
specd unlink api auth
```

---

## Orchestration

### `brain`
```
specd brain <start|step|run|status|cancel|resume|claim|heartbeat|report> <spec> [args] [--authority]
```
Run the opt-in deterministic orchestration controller. No LLM sits in its decision path.
`run` records a pending mission/dispatch for every currently-ready, unleased task (one wave) and
returns. It does **not** launch a worker, agent, model, or adapter. Workers explicitly `claim` a
pending mission, renew its typed lease with `heartbeat`, then `report` passing current evidence.
Report validates mission/lease/worker/role/HEAD, derives the local diff and scope verdict, and calls
normal task completion. Pending dispatch remains no proof of delivery or work.

External delivery uses versioned A2A JSON envelopes for `mission`, `claim`, `heartbeat`, `cancel`,
and `report`. Envelopes preserve required identity/digest pins, reject unknown versions, kinds, and
fields, and keep adapter/message metadata outside semantic payloads. Export refuses secret markers,
raw prompts/source, hidden reasoning, and unbounded tool output. A2A is a mapping contract only:
core performs no network call, and imported transport data grants no authority.
**Phases:** post-requirements.

| Flag | Value | Description |
|---|---|---|
| `--authority` | bool | Grant dispatch authority (fail-closed by default). |

```bash
specd brain start payments --authority
specd brain claim payments payments.s1.T1 worker-1 craftsman
specd brain heartbeat payments <lease-id> worker-1
specd brain report payments <lease-id> worker-1
specd brain status payments
specd brain resume payments
specd brain cancel payments
```

---

## Deferred

### `triage`
```
specd triage <spec>
```
Run the opt-in extended-loop triage tier. **Deferred:** registered but not wired — prints a
deferral notice and exits 0. **Phases:** any.

```bash
specd triage payments
```

---

## Security release proof

Production sandbox declarations use `sandbox-adapter/v1` and platform class `linux`, `darwin`, or
`ci`. Production requires `credentials.hidden`, `network.isolated`, `resources.bounded`,
`home.synthetic`, and `filesystem.write-bounded`; incomplete or unknown claims fail before process
execution. Promoted incidents use deterministic `security-regression/v1` fixtures with redacted
provenance and policy-digest-pinned attestations. See `docs/troubleshooting.md` for recovery.
