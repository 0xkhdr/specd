# specd — Command Reference

> **Status:** Normative documentation for current `specd` behavior.

> **Source of truth.** Every verb, flag, exit code, and allowed phase on this page is
> generated to match `internal/core/commands.go` (`var Commands`, `HelpSchemaVersion 1`).
> `docs/CHEATSHEET.md` is a byte-identical copy of this file; `scripts/docs-lint.sh`
> fails CI if they drift. Edit this file, then copy it over the cheatsheet.

Adapter compatibility is negotiated before execution against an exact offered adapter-envelope
and payload schema version. It is independent of CLI/state versions; unknown versions fail closed
with no implicit downgrade. See `docs/adapter-contract.md`.

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
bound) into the project root; an existing `project.yml` is never overwritten. It also installs
inspectable, schema/version-stamped maintenance templates under
`.specd/templates/maintenance/` for incident follow-up, dependency/deprecation work, migration,
and recurring invariants. Each template traces source → requirements → tasks → evidence →
learning and includes readiness-valid typed intake and successor-link examples. Content added
outside its `specd:managed` markers survives `--repair` and `--refresh`. **Phases:** any.

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
specd agents [inspect | doctor | guide <slug>] [--json]
```
Inspect installed agent artifacts, run read-only diagnostics with `doctor`, or emit deterministic driver actions with `guide`. `agents inspect` is an alias of bare `agents`. With orchestration enabled, `doctor` verifies that the active handshake harness has all required Pinky worker definitions (`.claude/agents/pinky-*.md` or `.codex/agents/pinky-*.toml`) and that Codex registration is consistent; failures name `specd init --repair`. Running this multi-operation verb with `--help` prints its operation palette and exits 0. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit JSON. |

```bash
specd agents
specd agents inspect --json
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

### `incident`
```
specd incident seed <new-spec> --source-spec <spec> --release <id> --deployment <id> --criterion <id> --evidence-ref <ref[,ref]>
```
Seed a new spec from bounded delivery observation references. References must be absolute
scheme-based identifiers without query strings, fragments, credentials, or embedded raw payloads;
source release/deployment ledgers remain unchanged. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--source-spec` | string | Source spec owning the immutable delivery ledger. |
| `--release` | string | Source release identity. |
| `--deployment` | string | Source deployment identity. |
| `--criterion` | string | Failed or observed health criterion. |
| `--evidence-ref` | string | Comma-separated bounded external references. |

```bash
specd incident seed checkout-recovery --source-spec checkout --release rel-7 --deployment dep-4 --criterion availability --evidence-ref obs://health/42
```

### `archive`
```
specd archive <spec> --successor <spec> --owner <owner> --evidence <ref>
```
Retire a spec from active discovery while preserving every file hash and an audit manifest.
Archive requires an active successor and records a typed `supersedes` link; it never deletes or
rewrites audit content. Replaying the same request is idempotent. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--successor` | string | Active successor spec that receives the `supersedes` link. |
| `--owner` | string | Accountable archive owner. |
| `--evidence` | string | Audit evidence reference authorizing retirement. |

```bash
specd archive payments-v1 --successor payments-v2 --owner platform --evidence release:rel-7
```

### `approve`
```
specd approve <spec>
```
Record human approval and advance exactly from the current lifecycle status to its immediate
successor when readiness gates pass. Same, skipped, backward, unknown, and terminal transitions
fail before gate evaluation or mutation. The command takes the spec slug only; an explicit
target argument is rejected. Tasks-phase approval validates every non-empty task `evidence` cell
and reports requirement/criterion coverage gaps against the tasks.md `refs` column as warnings;
executing-phase approval blocks on the same gaps and names both fixes: add IDs to `refs`, or mark
the task `kind: deferred`. **Phases:** any. **Human only.**

```bash
specd approve payments
```

### `mode`
```
specd mode <spec> orchestrated
```
Record the separate human-approved transition into orchestrated mode. Requires
`orchestration.enabled: true`, changes mode through state CAS, and never impersonates lifecycle
approval or changes lifecycle status. **Phases:** any. **Human only.**

```bash
specd mode payments orchestrated
```

### `exception`
```
specd exception <approve|revoke> <finding> [governed exception fields]
```
Append an immutable governed security-exception lifecycle record. Every governed field is
required; evidence integrity and worker authority cannot be waived. This operation never changes
lifecycle status. **Phases:** any. **Human only.**

```bash
specd exception approve scanner-false-positive --reason 'reviewed false positive'
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

### `drift`
```
specd drift <spec> [--json]
```
Project versioned declarations from `.specd/specs/<spec>/drift.json` against append-only local
verify evidence. Output distinguishes `holds`, `drifted`, `not-evaluable`, and `none`; each declared
finding includes source, workspace-relative path, severity, last passing HEAD when known, and a
suggested successor command. Read-only and offline: it never runs evidence commands or creates a
spec. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit stable JSON Lines. |

```bash
specd drift payments --json
```

### `recurring`
```
specd recurring record <spec> --check <id> --head <sha> --release <id> --config <id> --verdict pass|fail --observed-at <RFC3339>
```
Append a validated result envelope produced by external CI or a scheduler. Results pin check,
HEAD, release, configuration, verdict, and explicit observation time under the spec lock; later
results never replace earlier passing records. `specd` validates and records only—it does not run,
schedule, or poll checks. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--check` | string | Recurring check identity. |
| `--head` | string | Tested git HEAD. |
| `--release` | string | Tested release identity. |
| `--config` | string | Tested configuration identity. |
| `--verdict` | `pass` or `fail` | Check verdict. |
| `--observed-at` | RFC3339 | Explicit observation time supplied by runner. |

```bash
specd recurring record payments --check api-health --head 0123456789012345678901234567890123456789 --release rel-7 --config prod-v3 --verdict pass --observed-at 2026-01-01T00:00:00Z
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
specd task <id> [--override --reason <text>]
```
Show task details or clear an escalated task with a human override. **Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable task row. |
| `--override` | bool | Clear an escalated task (resets the verify-failure ratchet; does not complete it). Requires `--reason`. |
| `--reason` | string | Human justification for `--override` (required, non-empty). |

```bash
specd task T3 --json
specd task T3 --override --reason 'flaky infra, verified manually'
```

### `complete-task`
```
specd complete-task <spec> <id>
```
Complete one task by consuming current passing evidence through the gated completion transaction.
Verify records evidence only; it never changes task status. Completion requires evidence pinned to
current `HEAD`, declared fresh quality evidence, production authority/scope/security controls, and
the locked state CAS. No bypass or human override is available. **Phases:** post-requirements.
Missing declared evidence is reported as `EVIDENCE_MISSING` with the exact `class/check-id`.
For `test/*`, re-run `specd verify`; for `output_eval`, `trajectory_eval`, or `review`, import the
external envelope with `specd eval import <slug> <file> --task <id> --check <check-id>`, or remove
the declaration. Plain verify records carry no evidence class for those non-test contracts.
With top-level `profile: production`, raw task
operations are refused; an MCP caller supplies the claimed mission's digest-pinned `AuthorityV1`
packet, and dispatch derives changed-path scope from that mission baseline.

Completion accepts optional telemetry flags: `--tokens`, `--cost`, `--duration-ms`,
`--input-tokens`, `--output-tokens`, `--cached-tokens`, `--provider`, `--model`, `--currency`,
`--pricing-ref`, `--telemetry-source`, and `--attestation-ref`.

```bash
specd complete-task payments T3
```

### `check`
```
specd check <spec> [--security] [--json]
```
Run the validation gate registry against a spec. Every non-empty task `evidence` cell is parsed
early as `class/check-id`; valid classes are `test`, `output_eval`, `trajectory_eval`, and
`review` (example: `test/unit`). Malformed declarations are blockers. Verify lines using
interactive job control (`kill %N`) or ending in `&` without capturing `$!` are deterministic
warnings. Coverage matching uses only the tasks.md `refs` column. **Phases:** any.

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
(exit 0 pinned to a resolvable git HEAD). A passing task verify stamps an `EvidenceEnvelopeV1`
for each declared `test/<check-id>` at the same HEAD with producer `specd-verify`; non-test
classes remain external and are never stamped. If non-test evidence remains outstanding, success
names its contract and exact `eval import` command instead of suggesting `complete-task`.
**Phases:** post-requirements.

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

The human-readable manifest remains the default output; hosts opt into the typed machine
manifest (`--json`) additively. The machine manifest
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
adapter or contacts a provider. Declared evidence uses `class/check-id`, where class is one of
`test`, `output_eval`, `trajectory_eval`, or `review`; `--check` is the check-id from that
declaration. Running `eval --help` (or bare `eval`) prints its operation palette and exits 0.
**Phases:** any.

| Flag | Value | Description |
|---|---|---|
| `--json` | bool | Emit machine-readable JSON for status. |
| `--task` | string | Expected task identity for import. |
| `--check` | string | Check-id from the task's declared `class/check-id` evidence cell. |

```bash
specd eval import payments adapter.jsonl --task T1 --check rubric-v1
specd eval status payments --json
```

### `help`
```
specd help [command] [--json]
```
Show command help. `help --json` includes each flag's enum plus optional `values` shape or
provenance (for example `pass|fail`, evidence classes, and where brain mission IDs come from).
Multi-operation verbs `brain`, `eval`, `exception`, and `agents` also accept `--help`, render
their palette usage/flags/examples, and exit 0. **Phases:** any.

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
specd report <spec> [--pr|--metrics|--efficiency|--rollup|--delivery|--outcome-review|--json|--history|--trace|--proof|--format prometheus|event|otel]
specd report --portfolio
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
| `--portfolio` | bool | Emit deterministic cross-spec release/environment status and blockers from local ledgers. |
| `--outcome-review` | bool | Join local evidence to feedback references; missing outcomes remain `unknown`. |
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
specd report --portfolio
specd report payments --outcome-review
specd report payments --format event
specd report payments --format prometheus
specd report payments --format otel
```

---

## Integration

Portfolio status/export is bounded to 10,000 compact spec records. Routine projections never load
spec prose or full context. Archive manifests bound active-context growth while retaining immutable
history under `.specd/archive/specs/<slug>/` for explicit audit retrieval.

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
Tool-call flags are JSON properties, not positional `args`: any `args` element beginning with
`--` is rejected and the error names both the offending element and the working property spelling
(for example, pass `guide: true`, not `"--guide"` inside `args`).

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
specd link <from-slug> <to-slug> [--kind <kind>] [--reason <text>]
```
Record a typed, traceable cross-spec dependency link. Every kind preserves dependency ordering;
`kind` defaults to `follows`. **Phases:** any.

| Flag | Type | Description |
|---|---|---|
| `--kind` | string | Link kind: `follows`, `regresses`, `maintains`, or `supersedes` (default: `follows`). |
| `--reason` | string | Optional human-authored reason stored with link. |

```bash
specd link api auth
specd link api-v2 api --kind supersedes --reason 'replace obsolete contract'
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
Mission IDs are minted by brain dispatch and listed by `specd brain status`; workers must not
invent them. Dispatch authority is absent by default and is granted per run with `--authority`.
Wait output distinguishes: absent authority (`specd brain run <slug> --authority`), empty frontier
(`specd status <slug> --guide`), and missing active-harness worker definitions
(`specd init --repair`). Running `brain --help` (or bare `brain`) prints all operations and exits 0.

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

## Security release proof

Production sandbox declarations use `sandbox-adapter/v1` and platform class `linux`, `darwin`, or
`ci`. Production requires `credentials.hidden`, `network.isolated`, `resources.bounded`,
`home.synthetic`, and `filesystem.write-bounded`; incomplete or unknown claims fail before process
execution. Promoted incidents use deterministic `security-regression/v1` fixtures with redacted
provenance and policy-digest-pinned attestations. See `docs/troubleshooting.md` for recovery.
