# Adapter contract and boundary index (Domain 10)

The durable product of Domain 10 is a **contract**, not a set of integrations.
Core generates request envelopes as data and consumes result envelopes only after
they are validated and pinned; core never imports `internal/adapter`. This file is
the maintained boundary index required by R5.1: every cross-domain integration
item is classified with an owner, so no item ships without a boundary decision.

## Boundary classes

- **core** — deterministic logic that lives inside `internal/core*`/`internal/context`.
- **adapter contract** — a versioned envelope/interface `specd` owns and freezes.
- **reference adapter** — a thin, removable example executable shipped for conformance.
- **external only** — a vendor/provider integration that lives outside this repo.

## Common envelope (`adapter/v1`, frozen)

`internal/adapter/envelope.go` defines one request and one result envelope with a
frozen common core: `schema_version`, `kind`, `request_id`/`correlation_id`,
subject identity (`spec_slug`/`task_id`/`mission_id`/`git_head`/`release_id`/
`environment`), `actor`/`authority_ref`, `input_refs`/`input_digests`,
`capabilities_required`/`capabilities_offered`, `limits`, `status`/`exit_class`/
`retryable`, `output_refs`/`evidence_refs`, `measurements`, `redactions`/data
`class` per ref, `started_at`/`finished_at`, `adapter_name`/`adapter_version`.
Payload-specific fields are additive extensions carried in `payload` and owned by
the consuming domain. The common core was co-designed from the P0 field demands of
Domains 04/05/07/08 and frozen once at `10c`.

## Boundary index

| Integration item | Boundary class | Owner | Notes |
|---|---|---|---|
| Import guard / zero-dependency invariant | core | Domain 10 | static test, no runtime deps |
| Common request/result envelope (`adapter/v1`) | adapter contract | Domain 10 | frozen here |
| Result identity + staleness check | core | Domain 10 | rejects before any gate (R3.2) |
| Data classification taxonomy + export redaction | adapter contract | Domain 10 / 06 | 10 defines, 06 enforces |
| Adapter runner (stdin/stdout JSON) | adapter contract | Domain 10 | opt-in, sandboxed (later wave) |
| `specd adapters --json` inspection | core | Domain 10 | read-only, no secret load (later wave) |
| Eval request/result envelope | adapter contract | Domain 04 | payload extension |
| Eval runner executable | reference adapter | Domain 04 | thin example |
| Mission/worker transport + A2A mapping | adapter contract | Domain 05 | payload extension |
| Model/provider routing integration | external only | Domain 05 | vendor SDK outside repo |
| Sandbox/secret-env contract | adapter contract | Domain 06 | runner policy |
| Telemetry event schema + OTel export | adapter contract | Domain 07 | export via external adapter |
| Provider-attested usage ingestion | external only | Domain 07 | signed envelope from outside |
| Deployment/CI adapter envelope | adapter contract | Domain 08 | release/env identity |
| CI/CD provider integration | external only | Domain 08 | vendor system outside repo |
| Runtime-feedback → maintenance link | adapter contract | Domain 09 | cannot mutate completed history |

Every roadmap integration item maps to one row here or to a domain-owned
extension of the frozen envelope. Adapter schema versions negotiate
independently of the CLI and on-disk state schema (R10.4).

## Adapter schema compatibility and negotiation

Adapter-envelope and payload schemas are independent of CLI and on-disk state schemas.
Before any side effect, caller and adapter must agree on an exact offered version and all required
capabilities. Unknown versions fail closed; there is no implicit downgrade or fallback.

An additive change may retain a version only when every previously valid message keeps its meaning
and old strict decoders remain valid. Optional domain payloads use their own schema version. A
breaking change—removing or renaming a field, changing meaning/defaults, or tightening a previously
valid value—requires a new schema version offered explicitly alongside any still-supported version.
CLI releases and state migrations neither select nor imply adapter compatibility.
