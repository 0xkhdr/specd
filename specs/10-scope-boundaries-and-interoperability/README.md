# Domain 10 — Scope boundaries and interoperability

## Goal

Keep the deterministic local core small and trustworthy while letting `specd` preserve intent,
policy, context, authority, evidence, and lifecycle identity across every external boundary —
model hosts, eval services, CI/CD, trace backends, production runtimes, and organizational
systems. Compatibility is achieved through **versioned envelopes and thin, opt-in adapters**,
never by importing provider/OTel/deployment SDKs into gate, DAG, or report paths. `specd`
generates a core request and validates a schema-checked result tied to input digests and software
identity; adapters translate that request to an external system and return bounded, attested,
redacted evidence. No LLM, no network, and no new runtime dependency ever enters the trusted core.

## Source and intent

Derived from `docs/google-sdlc-alignment/README.md` and
`docs/google-sdlc-alignment/10-scope-boundaries-and-interoperability.md`.
Paper position: full adherence does not require one binary to become the entire AI software
stack; the surrounding system must be engineered, but components need crisp responsibility
boundaries. `specd` targets the coding-harness scope and integrates with deployed-agent runtimes
rather than becoming one. This domain freezes **who owns what** and the **common envelope** so the
adapter waves in Domains 03–09 all speak one contract instead of ad-hoc command strings.

Current state: `submit.command` is a useful operator-controlled seam; MCP provides tool
interoperability; append-only orchestration/evidence ledgers can anchor external actions.
Gaps: external adapter contracts are command strings or implied host behavior, not versioned
request/result envelopes; provider/model, eval runner, trace exporter, deployment target, and
runtime identities share no common provenance contract; capability negotiation and failure
semantics are not standardized; no data-classification policy says which `.specd/` fields may cross
a process/network/A2A/CI/telemetry boundary; the zero-runtime-dependency invariant is defended only
by convention, not by a static test; and "paper compatibility" risks becoming feature accumulation.

## Position in the program

This domain is numbered last but its **P0 envelope is a keystone foundation**. Every "adapter"
wave in the roadmap consumes it: `03f`/`03g`, `04f`/`04g`, `05f`, `06i`, `07i`/`07j`,
`08i`/`08k`/`08l`, and `09l` all list "Domain 10 adapter/boundary contract" as a prerequisite.
Therefore `10a`–`10e` (envelope, identity, classification, boundary invariant, index) must be
authored and **frozen early** — co-designed from the P0 field demands of Domains 04/05/07/08 — so
consuming domains adopt a stable schema. Reference adapters (`10f`–`10i`) and ecosystem mappings
(`10j`–`10m`) follow later without blocking the deterministic core. See `../progress.md` for
the program-wide build order.

## Ownership

| Area | Domain 10 owns | Other domain owns |
|---|---|---|
| Core boundary | static import invariant forbidding model/eval/deploy/telemetry/runtime SDKs and network in trusted core; zero new runtime deps | every domain's own deterministic gate/DAG/report logic |
| Envelope | versioned request/result package, stable error/exit classes, common identity fields | each domain's payload-specific fields (mission, eval, deploy, telemetry) |
| Identity | common request/correlation/spec/task/mission/HEAD/release/env/adapter/digest/timestamp checks | Domain 01 spec revision; Domain 05 mission id; Domain 08 release/env id |
| Data classification | class taxonomy + default redaction policy for cross-boundary transfer | Domain 06 secret/redaction enforcement; Domain 07 telemetry privacy |
| Adapter runner | opt-in stdin/stdout JSON runner, timeout, output cap, env allowlist, sandbox policy | Domain 06 sandbox contract; hosts execute their own binaries |
| Capability negotiation | `capabilities_required`/`offered`, `specd adapters` doctor, fail-closed acceptance | external systems declare their real capabilities |
| Conformance | JSON fixtures + shell contract suite certifying third-party adapters | adapter authors implement to the contract |
| Ecosystem mapping | A2A/MCP/OTel/CI/runtime-feedback translations that preserve authority + evidence | external protocols/platforms define their own schemas |

## External prerequisites

Cross-domain links remain program dependencies tracked in `../progress.md`, not local DAG
`T<n>` IDs. Do not encode foreign task IDs in `depends-on`.

- Domains 04/05/07/08 must surface the concrete fields their adapter waves need (eval, mission,
  telemetry, deploy) so `10c` freezes the common envelope from real demand, not speculation.
- Domain 06 owns redaction/secret enforcement; this domain defines the classification taxonomy the
  redaction policy applies.
- Domain 05 owns worker transport; this domain provides the envelope it maps onto.

## Deliverable specs

| Wave | Slug | Result | Requires |
|---|---|---|---|
| W0 | `10a-boundary-contract-baseline` | roadmap boundary inventory, corrected wording, RED fixtures for every P0 gap and each validation scenario | — |
| W0 | `10b-core-import-invariant` | static architecture test rejecting prohibited imports/deps in trusted core directories | 10a |
| W1 | `10c-versioned-adapter-envelope` | stdlib-JSON request/result package, stable error/exit classes, byte-semantic golden round-trip, unknown version/field fail-closed | 10a, Domains 04/05/07/08 field demands |
| W1 | `10d-common-result-identity` | request/spec/task/mission/HEAD/release/env/adapter/digest/timestamp checks; mismatch rejected before any gate is satisfied | 10c |
| W1 | `10e-data-classification-and-redaction` | class taxonomy + default-redacted export; restricted classes provably absent by default | 10c, Domain 06 redaction |
| W2 | `10f-adapter-runner-seam` | opt-in stdin/stdout JSON runner; missing binary/timeout/oversized/malformed/non-zero → typed failing record | 10c,10d, Domain 06 sandbox |
| W2 | `10g-capability-doctor` | `specd adapters --json` read-only inspection; configured/missing/incompatible/disabled distinguishable; no secret load | 10c |
| W3 | `10h-offline-continuity-proof` | removing all adapters leaves core lifecycle/verify/gates/reports green; provider outage → blocked with exact external cause | 10f,10g |
| W3 | `10i-conformance-suite` | JSON fixtures + shell contract certifying a third-party adapter without importing `internal/` packages | 10f,10h |
| W4 | `10j-a2a-and-mcp-mapping` | mission/tool round trip preserving authority/role/scope/evidence semantics | 10d,10i, Domain 05 mission |
| W4 | `10k-otel-and-trace-export` | OTel-compatible export via adapter; correlation survives, raw source/prompt absent by default | 10e,10i, Domain 07 export |
| W5 | `10l-release-and-feedback-contract` | release + runtime-feedback envelopes; feedback links maintenance work without authority to mutate completed history | 10i, Domain 08 release, Domain 09 successor |
| W5 | `10m-versioning-policy-and-release-proof` | adapter-schema compatibility/negotiation policy separate from CLI/state schema; full conformance release proof | 10i,10j,10k,10l |

## DAG

```text
10a ─┬─> 10b
     └─> 10c ─┬─> 10d ─┬─> 10f ─┬─> 10h ─> 10i ─┬─> 10j ─┐
              │        │        └───────────────┼─> 10k ─┼─> 10m
              ├─> 10e ─┴────────> 10k           ├─> 10l ─┘
              └─> 10g ──────────> 10h

Domains 04/05/07/08 field demands ─> 10c
Domain 06 redaction ─> 10e,10f
Domain 05 mission ─> 10j
Domain 07 export ─> 10k
Domain 08 release + Domain 09 successor ─> 10l
```

## Program rules

1. No LLM, network call, or provider/OTel/deployment SDK in any gate, DAG, or report path. The
   `10b` static import test is the enforcement, not convention. Zero new runtime dependencies.
2. Every external action is a core-generated request and a schema-validated result. A JSON envelope
   that exits zero is **data, not proof**: identity, input digests, and software target are
   verified before any status can satisfy a gate.
3. Fail-closed acceptance. Missing adapter, incompatible capability, malformed/oversized result,
   identity mismatch, timeout, or stale digest is a failure/blocked record — never implicit
   success and never a fallback to unsafe local execution.
4. Offline continuity is inviolable. Requirements, design, tasks, context planning, gates, local
   verify, status, and reports work with all optional integrations absent. Adapter availability is
   never coupled to core availability.
5. Secrets and provider SDK state never enter `.specd/` Markdown/JSON or model context. Secrets are
   supplied by the adapter execution environment and allowlists; project config selects allowed
   adapters/capabilities.
6. Adapters receive only classified fields and references needed for the action; results are
   redacted before durable storage. Production feedback is untrusted data, not instruction.
7. Reference adapters are not universal production integrations — the **conformance contract is the
   durable product**. Vendor adapters requiring dependencies live in separate repos/packages.
8. Adapter schema versions negotiate independently of the CLI and on-disk state schema. Additive
   and breaking changes follow declared negotiation behavior. Stdlib-only core; no `reference/`
   edits; verbs declared once in `internal/core/commands.go`, mirrored in both docs.

## Completion claim

The domain is complete when: (1) a static test rejects any model/eval/deploy/telemetry/runtime SDK
or network import in trusted core directories and the binary keeps zero runtime dependencies;
(2) a versioned stdlib-JSON envelope round-trips byte-semantically and rejects unknown
version/field and malformed evidence; (3) a result whose request id, spec/task/mission,
HEAD/release/environment, adapter version, or input digest mismatches is rejected before it can
satisfy any gate; (4) a data-classification taxonomy proves secrets/raw source/prompts are absent
or redacted by default in every export; (5) an opt-in adapter runner turns missing-binary/timeout/
oversized/malformed/non-zero-exit into typed failing records; (6) `specd adapters` distinguishes
configured/missing/incompatible/disabled read-only without loading secrets; (7) removing all
adapters leaves the full core lifecycle, verify, gates, and reports green, and a provider outage
blocks the dependent task with an exact external cause while local work proceeds; (8) a third-party
adapter passes the public JSON+shell conformance suite without importing `internal/`; (9) A2A/MCP,
OTel, and release/runtime-feedback round trips preserve authority, role, scope, identity, and
evidence semantics and cannot mutate completed history; and every validation scenario in
`10-scope-boundaries-and-interoperability.md` has a deterministic offline fixture.
