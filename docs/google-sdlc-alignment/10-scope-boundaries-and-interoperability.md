# Domain 10 — Scope Boundaries and Interoperability

> **Status:** Historical assessment; proposals are non-normative.
> **As of commit:** `f62f16f44f92de5fa59a9304b8b10b0721564eaa` (2026-07-10).
> **Superseded by:** [`specs/11-workflow-coherence`](../../specs/11-workflow-coherence/README.md) and current normative docs.

## Purpose

Define what `specd` must own to align with the paper, what it should describe through stable contracts, and what must remain in external model hosts, CI/CD systems, eval services, production runtimes, and organizational platforms.

Full adherence to the paper's principles does not require one binary to become the entire AI software stack. For `specd`, compatibility means preserving intent, policy, context, authority, evidence, and lifecycle identity across boundaries while keeping the deterministic local core small and trustworthy.

## Paper position

The comparison document notes that the paper spans two related scopes:

1. using agents to build and maintain software; and
2. building autonomous agents that serve users in production.

It also describes MCP/A2A interoperability, portable Agent Skills, model routing, evals, observability, deployment, governance, and organizational operation. `specd` strongly targets the first scope as a coding harness. The comparison explicitly recommends that it integrate with deployed-agent runtimes rather than transform itself into one.

That recommendation follows the paper's harness logic: the surrounding system must be engineered, but components still need crisp responsibility boundaries.

## Current `specd` handling

### Core responsibilities already well placed

- Versioned on-disk requirements, design, task DAG, state, evidence, and role/steering artifacts under `.specd/`.
- Pure deterministic gates and report projections in `internal/core` and `internal/core/gates`.
- Atomic writes, state CAS, per-spec locks, byte-stable task parsing, and HEAD-pinned evidence.
- A single stdlib-only static CLI with no network calls in core workflows.
- A canonical command palette and MCP server for tool discovery/calls.
- Optional operator-command boundaries for submission, sandbox process execution for verify, and file-based integration artifacts for agent hosts.
- Local append-only orchestration and evidence ledgers that can anchor external actions.

### Boundary weaknesses

- External adapter contracts are mostly command strings or implied host behavior rather than versioned request/result envelopes.
- MCP provides tool interoperability but not cross-agent mission transport or a general adapter lifecycle.
- Provider/model, eval runner, trace exporter, deployment target, and production-runtime identities do not share a common provenance contract.
- `submit.command` is a useful operator-controlled seam, but comparable seams for eval, model host, deployment, rollback, and runtime feedback are absent.
- Capability negotiation and failure semantics are not standardized across integrations.
- No explicit data-classification/redaction policy says which `.specd` fields may cross process, network, A2A, CI artifact, or production telemetry boundaries.
- The repository's zero-runtime-dependency invariant can be pressured by proposals to embed provider SDKs, OpenTelemetry libraries, deployment clients, or protocol stacks directly.
- The paper-alignment roadmap needs a clear “compatible through adapter” classification so broad production-agent requirements are not mistaken for core CLI features.

## Domain ownership map

| Concern | `specd` core owns | Adapter/external system owns | Boundary artifact |
|---|---|---|---|
| Intent and lifecycle | Artifact schema, phase/state, approvals, links | Product decisions and authorship | Versioned spec/state records |
| Task selection | DAG/frontier, role/scope eligibility | Worker scheduling capacity | Mission envelope |
| Model inference | Capability/routing policy and recorded identity | Provider authentication, API call, rate limits | Route request/result |
| Context | Selection policy, manifest, budget, digests | Content loading and model-window assembly | Context manifest |
| Tools | Palette, allowed phase, authority metadata | Host execution UI/runtime | CLI/MCP schemas |
| Verification | Command contract, execution evidence, completion gate | Project test binaries/services | Verify record |
| Non-deterministic eval | Required rubric/dataset/threshold and accepted scored artifact | Eval runner or judge invocation | Eval request/result + provenance |
| Orchestration | Mission identity, leases, ledger, report validation | Worker process launch and transport | Claim/heartbeat/report envelopes |
| Observability | Local events, correlations, deterministic export | Collection, storage, visualization, alerting | JSONL/OTLP-compatible export |
| Deployment | Required evidence policy, release/environment identity | CI/CD platform and credentials | Deploy/observe/rollback result |
| Production agent runtime | Build/release requirements and feedback intake | Serving, user sessions, runtime permissions, safety | Release manifest and feedback record |
| Organization | Policy templates and auditable approvals | Staffing, accountability, SLAs, on-call | Ownership/policy metadata |

## Common adapter contract and fields

| Field | Requirement |
|---|---|
| `schema_version` / `kind` | Identify envelope and reject incompatible versions. |
| `request_id` / `correlation_id` | Idempotent call identity and cross-ledger trace correlation. |
| `spec_slug` / `task_id` / `mission_id` | Preserve the exact unit of intent/work. |
| `git_head` / `release_id` / `environment` | Pin the software and operational target. |
| `actor` / `authority_ref` | Identify human, harness, worker, or service and its delegated capability. |
| `input_refs` / `input_digests` | Make context, rubric, dataset, config, and policy inputs reproducible without copying them. |
| `capabilities_required` / `capabilities_offered` | Deterministic negotiation before side effects. |
| `limits` | Timeout, retries, token/cost/data limits, and sandbox/network policy. |
| `status` / `exit_class` / `retryable` | Stable result semantics; distinguish rejected, failed, timed out, unavailable, and succeeded. |
| `output_refs` / `evidence_refs` | Content-addressed artifacts rather than unbounded inline logs. |
| `measurements` | Tokens, cost, duration, scores, and health values with units/source. |
| `redactions` / `data_classification` | Record what was removed and which transfer policy applied. |
| `started_at` / `finished_at` | Operational timing; deterministic reports may normalize/order separately. |
| `adapter_name` / `adapter_version` | Provenance and compatibility. |

## Gaps and failure modes

- Embedding provider SDKs introduces network behavior, credentials, dependency churn, and non-determinism into trusted core paths.
- A shell adapter exits zero but returns malformed or stale evidence; the harness records success without validating identity/provenance.
- An eval result references a rubric or dataset that changed after execution.
- A deployment adapter points to a different commit/environment than the spec completion evidence.
- A remote worker receives source, prompts, secrets, or customer data because no classification policy constrains envelope export.
- Different adapters interpret timeout, retry, cancel, and “success” differently, causing accidental retries or false completion.
- An optional integration outage blocks local planning because core and adapter availability are coupled.
- “Paper compatibility” becomes feature accumulation: a runtime server, dashboard, marketplace, model gateway, and scheduler enter the binary and erode subtractive bias.

## Target best-practice architecture

1. **Local deterministic core:** parses and validates artifacts, computes gates/frontiers/policy, records append-only results, and renders reports without network or model calls.
2. **Versioned envelopes:** every external action begins with a core-generated request and ends with a schema-validated result tied to input digests and software identity.
3. **Thin adapters:** executable plugins/processes translate envelopes to a provider, CI/CD platform, runtime, or protocol. They are opt-in and independently versioned.
4. **Fail-closed acceptance:** missing adapter, incompatible capability, malformed result, identity mismatch, timeout, or stale digest is a failure/blocked record, never implicit success or fallback to unsafe execution.
5. **Policy-owned selection:** project configuration selects allowed adapters/capabilities; environment supplies secrets. Neither secrets nor provider SDK state enters project artifacts.
6. **Offline continuity:** requirements, design, tasks, context planning, gates, local verify, status, and reports continue to work when all optional integrations are absent.
7. **Auditable export:** adapters receive only classified fields and references needed for the action; results are redacted before durable storage.

## Recommended action plan

### P0 — Freeze responsibility and envelope rules

1. Add an architecture decision and contributor invariant stating that model, eval-service, deployment, telemetry-backend, and production-runtime calls cannot enter gate/DAG/report paths or add core runtime dependencies. **Acceptance:** static architecture tests reject prohibited imports/packages in trusted core directories.
2. Define a small versioned adapter envelope package using Go stdlib JSON types and stable error classes. **Acceptance:** golden fixtures round-trip byte-semantically; unknown required fields/version and malformed evidence fail closed.
3. Classify every roadmap item as `core`, `adapter contract`, `reference adapter`, or `external only` in this directory's index. **Acceptance:** no action item lacks an owner/boundary classification.
4. Define data classification for spec text, source paths/content, prompts, tool output, secrets, telemetry, and production feedback. **Acceptance:** export tests prove restricted classes are absent/redacted by default.
5. Add identity checks common to every result: request id, spec/task/mission, HEAD/release/environment, adapter version, input digests, and timestamps. **Acceptance:** a mismatched fixture is rejected before its status can satisfy a gate.

### P1 — Build reference seams, not a platform monolith

1. Generalize the safe operator-command pattern into an optional adapter runner with stdin/stdout JSON, timeout, output cap, environment allowlist, and sandbox policy. **Acceptance:** missing binary, timeout, oversized/malformed output, and non-zero exit produce typed failing records.
2. Use the runner for reference adapters or fixtures for eval, trace export, deployment evidence, and worker transport. Keep production vendor adapters in separate repositories/packages when dependencies are required. **Acceptance:** removing all adapters leaves core tests and workflows green.
3. Add `specd adapters --json`/doctor-style capability inspection without loading secrets. **Acceptance:** configured, missing, incompatible, and disabled states are distinguishable and read-only.
4. Add conformance suites published as JSON fixtures and shell test contracts. **Acceptance:** third-party adapters can be certified without importing internal Go packages.

### P2 — Interoperate across the production ecosystem

1. Map mission envelopes to A2A and tool schemas to MCP without losing `specd` authority/evidence fields. **Acceptance:** canonical round trips preserve all required semantics.
2. Map local event exports to OpenTelemetry-compatible records via an external adapter. **Acceptance:** correlation survives export while raw source/prompt data remains absent by default.
3. Define release and runtime-feedback contracts usable by common CI/CD and deployed-agent platforms. **Acceptance:** feedback can create/link maintenance work without granting runtime systems authority to mutate completed history.
4. Publish compatibility/versioning policy for adapter schemas separately from the CLI and on-disk state schema. **Acceptance:** additive and breaking changes follow declared negotiation behavior.

## Production validation scenarios

| Scenario | Expected result |
|---|---|
| No adapters installed | Core lifecycle, local verify, gates, and reports remain fully usable. |
| Adapter missing or wrong version | Preflight fails before side effects with a typed capability/version finding. |
| Malformed or oversized adapter output | Result is rejected and bounded; no completion/deploy/eval success is recorded. |
| Adapter times out or crashes | Failure record is durable, retry policy is bounded, and core state stays consistent. |
| Stale eval/deploy result | HEAD/release/input digest mismatch prevents the result satisfying a gate. |
| Restricted data export | Conformance test shows secrets/raw source/prompts are omitted or explicitly approved/redacted. |
| Provider outage | Local work continues; provider-dependent task becomes blocked with exact external cause. |
| A2A/MCP conversion | Version, authority, role, scope, identity, and evidence semantics survive round trip. |
| Third-party adapter | Passes public fixture suite without importing `internal/` packages. |

## Context-safety considerations

- Adapter requests carry content references and digests by default; inline content is opt-in, size-bounded, classified, and purpose-specific.
- Production feedback is untrusted data, not instruction. Keep it typed and outside role/steering prompts until a human-approved requirement or memory workflow promotes it.
- Tool and protocol schemas should be loaded on demand by capability; do not inject every adapter manual into task context.
- Export ledgers should retain enough provenance to audit decisions without storing chain-of-thought, hidden prompts, or full source snapshots.
- Secrets are supplied through the adapter execution environment and allowlists, never `.specd` Markdown/JSON or model context.

## Non-goals and risks

- `specd` is not a model gateway, production serving runtime, trace backend, deployment controller, marketplace, or organizational system of record.
- Reference adapters must not be mistaken for universal production integrations; conformance contracts are the durable product.
- Shell/process adapters increase attack surface. Strict executable allowlists, sandboxing, timeouts, output limits, and provenance are required.
- Interoperability standards evolve. Keep protocol translation outside core state and version adapters independently.
- Over-abstraction too early can freeze the wrong contract. Start with the P0 fields demanded by eval, worker, trace, and deployment domains and evolve through golden fixtures.
