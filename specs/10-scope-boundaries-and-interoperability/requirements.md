# Requirements — scope boundaries and interoperability

## Scope

Stable IDs. Domain 10 defines the boundary invariant, common adapter envelope, identity checks,
data classification, adapter runner, capability negotiation, offline continuity, conformance, and
ecosystem mappings that let `specd` interoperate without importing SDKs, adding runtime
dependencies, or putting model/network calls in trusted core paths.

### R1 — Core boundary invariant

- R1.1: When code in a trusted core directory (`internal/core`, `internal/core/gates`,
  `internal/context`, report/DAG paths) imports a provider/model, eval-service, deployment,
  telemetry-backend, protocol, or network package, a static architecture test shall fail.
- R1.2: When the module graph is evaluated, the binary shall retain zero runtime dependencies;
  a new `require` in `go.mod` outside test tooling shall fail the invariant.
- R1.3: Adapter execution code shall be isolated from gate/DAG/report packages so a gate cannot
  reach an adapter or network transitively.

### R2 — Versioned adapter envelope

- R2.1: The system shall define one versioned request envelope and one versioned result envelope
  using stdlib JSON types, carrying `schema_version`, `kind`, correlation/identity, capability,
  limit, status, output/evidence-ref, measurement, redaction, and provenance fields.
- R2.2: When an envelope declares an unknown required `schema_version`, `kind`, field value, or is
  otherwise malformed, the system shall fail closed with a stable error class and finding.
- R2.3: When an identical envelope is encoded repeatedly, output shall be byte-semantically
  stable (canonical field ordering and digests); golden fixtures shall round-trip.
- R2.4: Result status shall use stable classes that distinguish rejected, failed, timed-out,
  unavailable, and succeeded; a `retryable` flag shall bound retry behavior deterministically.

### R3 — Common result identity

- R3.1: Every result shall be validated against its request for `request_id`/`correlation_id`,
  `spec_slug`/`task_id`/`mission_id`, `git_head`/`release_id`/`environment`,
  `adapter_name`/`adapter_version`, `input_digests`, and timestamps.
- R3.2: When any identity field mismatches the pinned request or references an unreachable
  `git_head`/release/environment, the result shall be rejected before its status can satisfy any
  gate, completion, deploy, or eval decision.
- R3.3: A stale result whose input digest differs from the current pinned subject shall be marked
  historical, never current by accident.

### R4 — Data classification and redaction

- R4.1: The system shall define a classification taxonomy for spec text, source paths/content,
  prompts, tool output, secrets, telemetry, and production feedback.
- R4.2: When an envelope crosses a process/network/A2A/CI/telemetry boundary, restricted classes
  (secrets, raw source, prompts, chain-of-thought) shall be absent or redacted by default; export
  tests shall prove their absence.
- R4.3: Adapter requests shall carry content references and digests by default; inline content
  shall be opt-in, size-bounded, classified, and purpose-specific.

### R5 — Roadmap boundary classification

- R5.1: Every cross-domain integration item shall be classified `core`, `adapter contract`,
  `reference adapter`, or `external only` in a maintained index; no item shall lack an
  owner/boundary classification.

### R6 — Adapter runner seam

- R6.1: The system shall generalize the operator-command pattern into an opt-in adapter runner
  invoking a project-selected executable with a JSON request on stdin and a JSON result on stdout,
  bounded by timeout, output cap, environment allowlist, and sandbox policy.
- R6.2: Missing binary, timeout, oversized output, malformed result, or non-zero exit shall each
  produce a typed failing record; none shall be recorded as success or fall back to unsafe local
  execution.
- R6.3: Secrets shall be supplied only through the adapter execution environment and allowlist,
  never through `.specd/` artifacts or model context.

### R7 — Capability negotiation and inspection

- R7.1: A request shall declare `capabilities_required`; acceptance shall occur only when the
  adapter's `capabilities_offered` satisfies them before any side effect.
- R7.2: `specd adapters --json` shall be a read-only projection distinguishing configured, missing,
  incompatible, and disabled adapters without loading secrets.

### R8 — Offline continuity

- R8.1: When all optional adapters are absent, core lifecycle, local verify, gates, status, and
  reports shall remain fully usable; removing every adapter shall leave core tests green.
- R8.2: When a provider/adapter is unavailable, local planning shall continue and the dependent
  task shall become blocked with an exact external cause, never an implicit success or timeout-pass.

### R9 — Conformance

- R9.1: The system shall publish a conformance suite as JSON fixtures and shell test contracts so a
  third-party adapter can be certified without importing internal Go packages.
- R9.2: The suite shall cover the validation scenarios: no adapters, wrong version, malformed/
  oversized output, timeout/crash, stale result, restricted-data export, provider outage,
  A2A/MCP conversion, and third-party adapter.

### R10 — Ecosystem interoperability

- R10.1: Mission and tool schemas shall map to A2A and MCP without losing `specd`
  authority/role/scope/identity/evidence fields; canonical round trips shall preserve semantics.
- R10.2: Local event exports shall map to OpenTelemetry-compatible records via an external adapter
  with correlation preserved and raw source/prompt data absent by default.
- R10.3: Release and runtime-feedback contracts shall let feedback create/link maintenance work
  without granting runtime systems authority to mutate completed history.
- R10.4: Adapter schema compatibility/versioning policy shall be published separately from the CLI
  and on-disk state schema; additive and breaking changes shall follow declared negotiation.

## Non-goals

- `specd` is not a model gateway, serving runtime, trace backend, deployment controller,
  marketplace, or organizational system of record.
- Reference adapters are not universal production integrations; the conformance contract is the
  durable product.
- No protocol translation lives in core state; adapters version independently.
