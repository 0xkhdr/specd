# Design — Deterministic deployment and production assurance

## Decision

Build production delivery as a parallel, additive ledger domain that never touches the six-phase
completion authority. The lifecycle still ends at `submit`; a passing git-pinned evidence set plus
configured gates make a change *eligible* for a release candidate. `specd release candidate` freezes
an immutable candidate identity (spec revision, HEAD, evidence-set digest, artifact digest,
SBOM/provenance refs, bootstrap digest) into `releases.jsonl` — it builds and uploads nothing.
External CI/CD and runtime adapters perform the deploy and return a versioned, bounded evidence
envelope over stdin/file; core validates it against environment policy and appends a deployment
attempt to `deployments.jsonl` under the spec lock. A closed deployment state machine
(`requested → started → observing → healthy|failed → rolling_back → rolled_back`) fails closed on
every transition. Canary promotion and rollback are pure verdicts over fresh, allowlisted,
per-artifact/environment observation — never agent prose, never a timeout default. In parallel, the
agent bootstrap is made sufficient (one packet binds every identity), `orchestrated` mode is made
reachable through CAS/approval with a production-wired brake, and the repo's own install/upgrade
path is hardened with a staged atomic swap and rollback-on-smoke.

```text
lifecycle (unchanged) → submit → passing evidence-set digest ──────────────┐
                                                                           │
bootstrap packet {binary, state/context/template schema, root/spec/rev,    │
   palette/config/managed digests, allowed tools, next cmd} ── mismatch → EXIT
                                                                           ↓
specd release candidate → releases.jsonl (immutable: HEAD, evidence digest,
   artifact digest, sbom/provenance, bootstrap digest) ────────────────────┐
                                                                           │
environment policy (closed name → strategy/approver/criteria/window/       │
   freshness/rollback target) ──────────┐                                  │
                                        ↓                                   ↓
[external adapter] deploy → stdin/file envelope (no creds, idempotency key,
   trust label) → validate + append deployments.jsonl (spec lock) ─────────┐
                                        ↓                                   │
delivery state machine: requested→started→observing→healthy|failed→        │
   rolling_back→rolled_back (every transition fails closed) ───────────────┼─> report --delivery
                                        ↓                                   │
canary verdict: all required criteria fresh, allowlisted, exact            │
   artifact/env, full window → promote (records baseline + evidence refs)   │
   else → rollback (target + reason + post-rollback health) ───────────────┘
                                        ↓
bounded observation ref → incident/refinement spec (raw logs stay external)
```

Missing/stale/mismatched observation never turns failure into success and never blocks a valid
source completion. No deployment record satisfies a task evidence gate. Production policy is never
relaxable by task text, prompt, or adapter response.

## Delivery envelopes and state machine

New `internal/core/delivery.go` defines the versioned envelopes and the closed state machine; new
`internal/core/gates/delivery.go` holds the pure verdict functions:

```text
ReleaseCandidateV1
  release_id (immutable, unique; artifact digest + human version), spec_id,
  spec_revision, git_head, task_evidence_set_digest, artifact_digest,
  sbom_ref, provenance_ref, bootstrap_digest, state_schema, created_at

DeploymentV1
  deployment_id, attempt (monotonic per release/env), release_id, environment,
  status ∈ {requested,started,observing,healthy,failed,rolling_back,rolled_back},
  strategy, population, window, adapter, authority, actor, idempotency_key,
  started_at, finished_at, telemetry_source, evidence_ref, attestation_ref

HealthObservationV1
  deployment_id, criterion_id, health_check, threshold, observation,
  freshness (observed_at + max_age), release/artifact/env identity, source

RollbackV1
  deployment_id, rollback_target (release_id), reason, adapter, action_result,
  post_rollback_health, capability_class, human_required
```

`release_id` is derived from artifact digest + version, never from an environment name. Transitions
are a table, not free code: an unknown status, a jump (e.g. `requested → healthy`), a mismatched
HEAD/artifact, a stale/missing observation, or a `failed` without a rollback target is rejected
before any append. The evidence gate passes or fails identically with the ledgers absent — delivery
is strictly additive.

## Ledgers and identity

New `internal/core/delivery_ledger.go` appends to `.specd/specs/<slug>/releases.jsonl` and
`deployments.jsonl` under `WithSpecLock`, reusing the existing atomic-append + checkpoint-replay so a
torn line yields one complete record or the prior one. `internal/cmd/release.go` and
`internal/cmd/deploy.go` host the verbs; `deployment_id`/`attempt` combine an adapter-provided
identity with a harness monotonic attempt, and the `idempotency_key` makes a duplicate adapter
callback a no-op or a fail-closed conflict — never a second logical deployment.

## Agent bootstrap binding

`internal/core/handshake.go` is extended so one JSON packet binds every identity a production driver
needs: binary version/commit (`internal/version`), state/context/template schema versions, workspace
root, active spec/status/revision, palette/config digests (already present), and — new — a
`managed_digest` over managed `AGENTS.md`/roles/steering content. Any pinned mismatch exits non-zero
before mutation. The packet is typed: harness instructions are a distinct field from untrusted
requirements/source/test-output/adapter-observation, so external text cannot amend authority.

## Orchestrated-mode reachability and brake

`internal/core/state.go` gains a declared, schema-validated `orchestrated` mode; a supported
transition in `internal/cmd/lifecycle.go` enters it through CAS + approval (no `state.json`
hand-edit anywhere in tests or guides). `internal/cmd/brain_run.go`/`Sense` populate cost from
accepted telemetry so the cost/deadline brake actually halts future dispatch — or the dormant public
implication is removed. When production policy requires trusted/attested telemetry or authority and
it is missing, dispatch fails closed with one actionable message.

## Adapter envelope and attestation

`internal/core/adapter_envelope.go` decodes a versioned stdin/file envelope with no implicit
credentials and zero new dependencies; `internal/core/attestation.go` verifies externally attested
CI/runtime identity offline using only Go stdlib crypto. A tampered envelope, wrong
repository/environment/audience, expired assertion, or untrusted key fails. Hostile prose or a
credential in an envelope is stored bounded/redacted (Domain 06 redaction) and never enters standing
instructions.

## Install/upgrade hardening

`scripts/install.sh` + new `scripts/release-smoke.sh` stage the new binary to a temp path, verify
checksum/attestation and `version --json` commit, run handshake/init smoke, preview managed-asset
diffs, atomically swap (rename), and retain the previous binary until post-install smoke passes —
restoring it on failure. A schema preflight refuses an unsupported future state/delivery schema and
an unsafe downgrade before any write. `.github/workflows/release.yml`/`.goreleaser.yml` install a
real just-built archive per OS/arch.

## Feedback, portfolio, and drills

`internal/core/incident.go` + `internal/cmd/incident.go` seed a new spec from a bounded observation
reference (source release/deployment/criterion + evidence refs) without loading raw payloads;
original ledgers stay immutable. `internal/core/program.go` gains a portfolio view identifying the
deployed/healthy/failed/rolled-back release per environment with no network call.
`scripts/upgrade-matrix.sh` + a scheduled workflow drive N-1 → N upgrade and crash-at-each-boundary
recovery against versioned fixtures.

## Verification ladder

- L0 — offline fixtures: every envelope/state-machine rule and each of the 15 production validation
  scenarios validates with networking disabled; unknown schema/environment/state-jump/mismatch fails
  closed.
- L1 — additivity: evidence gate and `complete` behave identically with delivery ledgers present or
  absent; no delivery record satisfies a task gate.
- L2 — bootstrap: a pinned mismatch (binary/schema/root/slug/revision/palette/config/managed) exits
  non-zero before mutation.
- L3 — reachability/brake: a supported path enters `orchestrated` via CAS/approval; the brake halts
  future dispatch on accepted telemetry; no `state.json` hand-edit exists.
- L4 — install/upgrade: staged swap, retained rollback binary, schema preflight, and
  rollback-on-failed-smoke pass on each supported OS/arch; corrupt staged binary is never promoted.
- L5 — delivery loop: canary cannot promote before its full fresh window; failed canary requires
  rollback/exception; rollback complete only after target health; duplicate idempotency key is one
  transition; reports are byte-identical; docs-lint and command-reference/CHEATSHEET mirror.

## W0 baseline observations (08a)

### T01 scout inventory — current behavior of the four P0 surfaces

Read against `internal/core/handshake.go`, `internal/core/state.go`, `scripts/install.sh`,
`scripts/regress-domains.sh`. Each gap is what the corresponding later wave must close.

**Handshake (`BootstrapHandshake`).** Today binds only `version` (literal `"1"`), `agent`,
allowed `tools`, `palette_digest`, `config_digest`, and `tool_contracts`. Gaps for 08b: no binary
release/commit (`internal/version` unbound), no state/context/template **schema** versions, no
workspace root, no active spec/slug/status/revision, no `managed_digest` over managed
`AGENTS.md`/roles/steering content, and no typing that separates harness instructions from untrusted
requirements/source/test-output/adapter-observation. There is no path that **exits non-zero before
mutation** on a pinned mismatch.

**Mode (`state.go`).** Modes are `default` and `agent` only; there is no declared, schema-validated
`orchestrated` mode and no reachable CLI/CAS/approval transition into it (08c). The cost/deadline
brake is not production-wired.

**Installer (`scripts/install.sh`).** Verifies the archive checksum, then installs the extracted
binary **directly** with `install -m 0755` over `$BIN`. Gaps for 08f: no staged temp path + atomic
rename, no retained previous binary, no rollback-on-failed-smoke, no attestation verification, no
`version --json` commit smoke, no state/delivery **schema preflight**, and no managed-asset diff
preview.

**Regress (`scripts/regress-domains.sh`).** The advertised **W0 honesty** invariant parses a
`| … | done |` table cell from `specs/progress.md`, but `progress.md` now uses `[x]`/`[ ]` checkbox
rows — so the awk yields **zero rows** and the check reports a pass over absent input (fail-open).
08a/T04 reproduces and labels this (`input absent: fail-open reproduced`); 08e/T22 replaces the line
with a fail/skip so absent input never passes.

### T05 auditor sign-off — 15 scenarios each have a planned fixture

Audited `docs/delivery-contract.md` §Fixture plan against the 15 production validation scenarios in
`docs/google-sdlc-alignment/08-deployment-and-production-assurance.md`. Every scenario (1–15) maps to
a named planned fixture and its landing wave; the four canonical envelope fixtures
(`release_candidate.json`, `deployment.json`, `health_observation.json`, `rollback.json`) are landed
by 08a and pinned by `TestDeliveryFixture`. Coverage is complete — no scenario is unmapped.
