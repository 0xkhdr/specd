# W0 T01 — Boundary inventory (Domain 10)

Scout, read-only. Maps requirements R1–R10 to current code surfaces and to the boundary /
consuming domain that must sit on each. Source: `requirements.md`,
`docs/google-sdlc-alignment/10-scope-boundaries-and-interoperability.md`. All refs are
`path:line` at branch `sdlc-specs`.

## Current-state facts (verified)

- `go.mod` has **no `require` block** (module + `go 1.26` only) — R1.2 already true *by
  convention*, defended by **no static test** yet.
- Packages importing `internal/core` (non-test): `internal/cli`, `internal/cmd`,
  `internal/context`, `internal/core/gates`, `internal/core/gates/security`, `internal/mcp`,
  `internal/orchestration`. **No `internal/adapter` exists yet** (T02 creates it).
- `os/exec` appears only in `internal/core/verify/exec.go`, `internal/core/gates/security/gate.go`,
  `internal/cmd/registry.go`. **No `net`/`net/http` anywhere** in `internal/`.
- `internal/orchestration/acp.go:15` already carries a `// spec 10 R3` marker — worker-rigor
  identity fields were pre-seeded for this domain.

## R1–R10 mapping

| Requirement | Current code surface | Boundary / consuming domain | Gap |
|---|---|---|---|
| **R1.1** prohibited import in trusted core fails | trusted-core dirs today: `internal/core/*.go`, `internal/core/gates/`, `internal/context/`, report path `internal/core/report.go`, DAG `internal/core/dag.go`+`frontier.go`. No importer references any model/eval/deploy/telemetry SDK. | New `internal/adapter/import_guard_test.go` (T02) parses import graph of these dirs. Boundary sits **between `internal/core*`/`internal/context` and `internal/adapter`**: adapter may import core envelope types, core must never import adapter. | No static architecture test exists. **P0.** |
| **R1.2** zero runtime deps | `go.mod` (no require) | `internal/adapter/import_guard_test.go` `TestZeroDependency` + `go mod verify` (T03) | No test asserting empty require set. **P0.** |
| **R1.3** adapter isolated from gate/DAG/report | gates `internal/core/gates/`, DAG `internal/core/dag.go`/`frontier.go`, report `internal/core/report.go` | Guard test must also forbid these three package sets from importing `internal/adapter` (transitive). | Not enforced. **P0.** |
| **R2.1** versioned request/result envelope | Closest existing typed envelopes: `internal/orchestration/acp.go:23` `ACPEvent`, report envelope fields; `internal/core/evidence.go:15` `EvidenceRecord`; `internal/core/submit.go:20` `SubmissionRecord`. All ad-hoc, no `schema_version`/`kind`. | New `internal/adapter/envelope.go` (T04). Envelope is the **keystone** consumed by 03/04/05/07/08/09 adapter waves. | No unified versioned envelope; stdlib JSON only. **P0.** |
| **R2.2** unknown version/field/malformed fail closed | none | `internal/adapter/envelope.go` reject path (T05); golden fixtures in `internal/adapter/testdata` | Missing. **P0 (RED fixture).** |
| **R2.3** byte-semantic stable encoding | Precedent: byte-stable tasks parser `internal/core/tasksparser.go`; canonical digest `internal/core/submit.go:35` `SummaryHash` | `internal/adapter/envelope.go` canonical ordering + digests, golden round-trip (T04) | Missing for envelopes. |
| **R2.4** stable status/exit classes + `retryable` | Ad-hoc exit handling: `internal/core/verify/exec.go:32` `TimeoutExitCode=124` | `internal/adapter/envelope.go` exit-class enum (T05) | No shared rejected/failed/timedout/unavailable/succeeded taxonomy. |
| **R3.1** result validated vs request identity | Partial fields exist scattered: `internal/orchestration/acp.go` `MissionID`/`GitHead`/`VerifyRef`/`Attempt` (~l.32–43); `internal/orchestration/lease.go:7` `WorkerID`; `internal/core/evidence.go:19` `git_head` + `HeadPinned` (l.115) | New `internal/adapter/identity.go` (T06). Consumers: Domain 05 (mission id), Domain 08 (release/env id), Domain 01 (spec revision). | No single common identity check across request↔result. **P0.** |
| **R3.2** mismatch rejected before any gate | Evidence gate today only checks exit 0 + `HeadPinned` (`evidence.go:115`) | `internal/adapter/identity.go` must reject **before** status can satisfy gate/completion/deploy/eval | Gate accepts a zero-exit adapter blob without provenance check. **P0.** |
| **R3.3** stale result marked historical | none (no digest-vs-current comparison) | `internal/adapter/identity.go` input-digest drift check (T06) | Missing. |
| **R4.1** classification taxonomy | Redaction precedent only: security gate `internal/core/gates/security/gate.go` allowlist model | New `internal/adapter/classify.go` + `docs/data-classification.md` (T07). **Domain 06 owns enforcement**, Domain 10 owns taxonomy. | No taxonomy for spec/source/prompt/tool-output/secret/telemetry/feedback. **P0.** |
| **R4.2** restricted classes absent/redacted at boundary | none — no export filter proving absence | `classify.go` export tests; consuming **Domain 07** (telemetry privacy), **Domain 06** (secret redaction) | No default-redacted export proof. **P0.** |
| **R4.3** content refs+digests default, inline opt-in | `input_refs`/`input_digests` concept exists only in the ownership doc; `internal/context/manifest.go` already uses digests/refs for context | `envelope.go`/`classify.go` inline opt-in + size bound | Adapter envelopes don't yet enforce refs-by-default. |
| **R5.1** every roadmap item classified core/adapter/reference/external | Roadmap: `specs/*/README.md`, `docs/google-sdlc-alignment/README.md` | Maintained index in `docs/google-sdlc-alignment/README.md` + `docs/adapter-contract.md` (T08) | No boundary-classification column on integration items. **P0.** |
| **R6.1** opt-in adapter runner (stdin/stdout JSON, timeout, cap, env allowlist, sandbox) | **Generalizes** `internal/core/submit.go` (`SubmitConfig` command+timeout, `config_loader.go:70`) and `internal/core/verify/exec.go:40` `Run` (timeout at l.44, env scrub at `scrubbedEnv` l.114); sandbox policy precedent `gates/security/gate.go:129` | New `internal/adapter/runner.go` (T09) reusing submit/verify exec pattern; **Domain 06** owns sandbox contract | Submit/verify are single-purpose; no general JSON-in/JSON-out runner with env allowlist. |
| **R6.2** typed failing record for missing/timeout/oversized/malformed/non-zero | `verify/exec.go` stamps `TimeoutExitCode`; no output cap / malformed handling | `runner.go` typed failure classes (T09) | No oversized/malformed-output handling; no output cap. |
| **R6.3** secrets via env allowlist only, never `.specd/`/model context | env scrub `verify/exec.go:114`; config selects command not secrets `config_loader.go` | `runner.go` env allowlist | Not generalized to adapters. |
| **R7.1** capability negotiation before side effect | none (`capabilities_required`/`offered` absent) | `envelope.go` + `internal/cmd/adapters.go` (T10) | Missing. |
| **R7.2** `specd adapters --json` read-only, no secret load | Verbs declared in `internal/core/commands.go`; dispatch `internal/cmd/registry.go`; doctor precedent = report projections `internal/core/report.go` | New `internal/cmd/adapters.go` + commands.go verb + registry + docs pair (T10) | Verb does not exist. |
| **R8.1** all-adapters-absent core stays green | Core lifecycle already runs offline (no `net` imports); e2e `internal/cmd/e2e_test.go` | `internal/adapter/offline_test.go` proof (T11) | Adapters must be opt-in so removal keeps tests green (structurally true today; needs proof once adapters land). |
| **R8.2** outage → blocked with exact cause, never implicit pass | Blocker plumbing exists: `internal/core/submit.go:114` `SubmitBlockers`; escalation `internal/core/escalation.go` | `offline_test.go` / runner provider-outage path (T11) | No adapter-outage→blocked mapping yet. |
| **R9.1** conformance suite (JSON fixtures + shell, no `internal/` import) | Regression precedent: `scripts/regress-domains.sh`, `scripts/regress-all.sh` | New `scripts/adapter-conformance.sh` + `internal/adapter/testdata` (T12) | No public certification suite. |
| **R9.2** covers all validation scenarios | Scenario table in alignment doc §"Production validation scenarios" | conformance fixtures (T12) | RED fixtures not authored. |
| **R10.1** A2A/MCP mapping preserves authority/role/scope/identity/evidence | MCP surface `internal/mcp/server.go`, `internal/mcp/tools_core.go`, `internal/mcp/tools_brain.go`; ACP mission transport `internal/orchestration/acp.go` | New `internal/adapter/a2a.go` (T13); **Domain 05** owns mission | No A2A transport; MCP maps tools only, not missions. |
| **R10.2** OTel export, correlation preserved, raw source/prompt absent | `internal/core/telemetry.go:18` `Annotations`/`TaskTelemetry`; `internal/core/prometheus.go:35` `RenderPrometheus` (Prometheus text export exists) | New `internal/adapter/otel_export.go` + `internal/cmd/report.go` (T14); **Domain 07** owns export | Prometheus exists; no OTLP-compatible record mapping; SDK must stay out of core (R1). |
| **R10.3** feedback links maintenance, cannot mutate completed history | Program/successor plumbing `internal/core/program.go`; append-only ledgers `acp.go`, `submit.go` | New `internal/adapter/feedback.go` (T15); **Domain 08** release, **Domain 09** successor | No runtime-feedback intake contract. |
| **R10.4** adapter-schema versioning separate from CLI/state schema | `schema_version` concept new; state schema in `internal/core/state.go` | `docs/adapter-contract.md` + `internal/adapter/version_test.go` (T16) | No published adapter versioning/negotiation policy. |

## Consuming domains 03–09 (adapter-wave prerequisites on this envelope)

Per `design.md` "Position in the program", these waves list "Domain 10 adapter/boundary contract"
as a prerequisite. Local spec dirs present: **03, 04, 06, 07** (05, 08, 09 not yet scaffolded here).

| Domain | Adapter waves | Needs from Domain 10 |
|---|---|---|
| 03 agent-tool-driving | `03f`/`03g` | envelope + capability negotiation for tool/host adapters |
| 04 verification-evals | `04f`/`04g` | envelope + identity (eval request/result, rubric/dataset digests) — **P0 field source** |
| 05 orchestration (worker transport) | `05f` | mission envelope + identity + A2A mapping — **P0 field source** |
| 06 security | `06i` | classification taxonomy + runner sandbox/secret-env contract |
| 07 observability | `07i`/`07j` | envelope measurements + OTel export + telemetry classification — **P0 field source** |
| 08 deployment | `08i`/`08k`/`08l` | release/env identity + deploy result envelope — **P0 field source** |
| 09 maintenance/successor | `09l` | runtime-feedback contract (R10.3) |

Cross-wave rule (`tasks.md` l.57): the `10c` common envelope must be frozen **only after** the
P0 field demands of Domains 04/05/07/08 are recorded here. Those four are the field sources; their
concrete fields are not yet enumerated in their local specs and must be surfaced before T04 freezes.

## P0 gaps (must have RED fixture / failing test before green — from alignment doc P0 plan)

1. **R1.1/R1.3** — static import-architecture test forbidding model/eval/deploy/telemetry/runtime
   SDK + network imports in `internal/core*`, `internal/context`, gate/DAG/report paths, and
   forbidding those paths from importing `internal/adapter`. (T02)
2. **R1.2** — zero-runtime-dependency assertion test (`go.mod` require set empty + `go mod verify`). (T03)
3. **R2.1/R2.2/R2.3/R2.4** — versioned stdlib-JSON request/result envelope with `schema_version`/
   `kind`, stable error/exit classes, byte-semantic golden round-trip, fail-closed on unknown
   version/field/malformed. (T04/T05)
4. **R3.1/R3.2** — common identity check (request/spec/task/mission/HEAD/release/env/adapter/digest/
   timestamp) that rejects a mismatched result **before** any gate/completion/deploy/eval accepts it.
   Today the evidence gate accepts a zero-exit blob on `HeadPinned` alone. (T06)
5. **R4.1/R4.2** — data-classification taxonomy + default-redacted export proving secrets/raw
   source/prompts/CoT absent by default at every boundary. (T07)
6. **R5.1** — boundary-classification index (core/adapter contract/reference adapter/external only)
   with an owner on every integration roadmap item. (T08)

Note: R1.2 (zero deps) and R8.1 (offline core) are already **true structurally** today — no
`require` block, no `net` imports — but neither is defended by a test, so both are P0 to lock in.
