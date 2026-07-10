# Design — scope boundaries and interoperability

## Decision

Add a new `internal/adapter/` package holding stdlib-JSON envelope types, identity validation,
classification/redaction, and the runner. Trusted core (`internal/core`, `internal/core/gates`,
`internal/context`, report/DAG paths) never imports `internal/adapter`; instead the CLI layer wires
adapter results into core as already-validated, pinned records. The zero-dependency invariant is
enforced by a static import test, not convention. Reference adapters are separate executables driven
over stdin/stdout JSON; production vendor adapters requiring dependencies live outside this repo.

## Contracts

| Contract | Source | Parsed identity | Gate use |
|---|---|---|---|
| request envelope | core-generated JSON | `schema_version`,`kind`,`correlation_id`,pinned subject | adapter dispatch input |
| result envelope | adapter stdout JSON | request/subject/adapter/digest identity | validated before satisfying any gate |
| classification | policy config + taxonomy | class per field/ref | export redaction gate |
| runner record | `state.json`/ledger record | typed exit class, correlation | failure/blocked evidence |
| capability | adapter manifest/doctor | required/offered set | pre-side-effect negotiation |
| conformance case | JSON fixture + shell contract | scenario id, expected class | certification suite |

## Common envelope fields

Per `docs/google-sdlc-alignment/10-...md`: `schema_version`/`kind`, `request_id`/`correlation_id`,
`spec_slug`/`task_id`/`mission_id`, `git_head`/`release_id`/`environment`, `actor`/`authority_ref`,
`input_refs`/`input_digests`, `capabilities_required`/`capabilities_offered`, `limits`,
`status`/`exit_class`/`retryable`, `output_refs`/`evidence_refs`, `measurements`,
`redactions`/`data_classification`, `started_at`/`finished_at`, `adapter_name`/`adapter_version`.
Payload-specific fields (mission, eval, deploy, telemetry) are additive extensions owned by the
consuming domain; the common core is frozen once at `10c` from those domains' P0 demands.

## Boundary invariant

A static test (`internal/adapter/import_guard_test.go` plus a `go list`-based check) enumerates
trusted core directories and fails if any imports a prohibited package class (provider/model SDK,
eval service, deployment client, telemetry backend, protocol stack, or `net/http`-style network in
core). A `go.mod` `require` outside test tooling fails the zero-dependency assertion. This is the
deterministic enforcement of the repository's existing convention.

## Fail-closed acceptance

```text
core request(pinned subject, capabilities, limits, classification)
        ↓ runner (timeout, output cap, env allowlist, sandbox)
adapter result(JSON)
        ↓ identity check (R3) → mismatch = reject before any gate
        ↓ capability check (R7) → unmet = reject
        ↓ classification check (R4) → restricted class present = reject/redact
validated pinned record → consuming domain gate
```

Missing binary, timeout, oversized/malformed output, and non-zero exit each map to a distinct
`exit_class` and a durable typed failing record. No path yields implicit success or unsafe local
fallback.

## Data classification

Taxonomy applied at export: `public-metadata`, `spec-text`, `source-path`, `source-content`,
`prompt`, `tool-output`, `secret`, `telemetry`, `production-feedback`. Default policy exports
references/digests + `public-metadata` only; `source-content`/`prompt`/`secret`/chain-of-thought
are absent unless a project policy explicitly opts in with size bounds. Redaction records what was
removed (`redactions` field) so audits see that a transfer policy applied. Domain 06 enforces
redaction; this domain supplies the taxonomy and default-deny export.

## Offline continuity

Adapters are opt-in and independently versioned. Core paths never call an adapter to complete a
lifecycle phase, gate, local verify, or report. A provider-dependent task with no configured or
reachable adapter becomes `blocked` with an exact external cause; it never times out into success.
Removing every adapter config leaves the full suite green — a conformance test asserts this.

## Verification layers

- Unit/golden: envelope round-trip, unknown version/field rejection, identity mismatch, exit-class
  mapping, classification redaction, import-guard enumeration.
- Command black-box: `specd adapters --json` states; runner timeout/oversize/malformed/non-zero.
- Fresh repo: full lifecycle with zero adapters green; provider-outage blocked-with-cause.
- Conformance: JSON+shell suite runs a fake adapter and a deliberately-broken adapter; a
  third-party adapter certifies without importing `internal/`.
- Ecosystem: A2A/MCP/OTel/release-feedback canonical round trips preserve required semantics.

## Risks

- Freezing the wrong envelope too early → `10c` is co-designed from Domains 04/05/07/08 P0 demands;
  additive extension fields keep the common core stable while payloads evolve.
- Shell/process adapters increase attack surface → strict executable allowlist, sandbox, timeout,
  output cap, provenance; Domain 06 owns enforcement.
- Feature accumulation ("compatibility" → platform monolith) → reference adapters stay thin and
  removable; vendor adapters live in separate repos; conformance is the product.
- Import-guard false negatives → enumerate directories explicitly and test the enumeration itself.
