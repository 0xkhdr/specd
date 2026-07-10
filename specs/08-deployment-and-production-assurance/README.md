# Domain 08 — Deployment and production assurance

## Goal

Let a `specd`-managed change graduate from evidence-backed source work to an identifiable,
observable, recoverable production release **without teaching an agent to bypass the six-phase
lifecycle**. Delivery is a parallel, additive ledger domain — release candidacy, environment
policy, deployment attempts, canary/health observation, promotion, and rollback — all as pure
functions of on-disk `.specd/` state plus validated adapter envelopes. No LLM in any delivery gate,
state transition, or report path. No cloud engine, no phone-home, no implicit network in core.
`specd` validates identities, required transitions, and recorded evidence; external CI/CD and
runtime adapters perform actions and return bounded, versioned envelopes. Source completion and
environment health stay different states that reports never conflate.

## Source and intent

Derived from `docs/google-sdlc-alignment/README.md` and
`docs/google-sdlc-alignment/08-deployment-and-production-assurance.md`.
Paper position: deployment is an essential part that turns a prototype into a service — hosting,
identity, observability, production infrastructure (`sdlc-paper.md:95-112`); AI-aware delivery
monitors health, auto-rolls-back bad releases, and feeds production behavior back into development
(`sdlc-paper.md:228-234`); production agents need persistent state, scoped permissions, eval
coverage, deployment, observability, governance, and an explicit prototype/production boundary built
before scaling (`sdlc-paper.md:384-414`, `sdlc-paper.md:468-500`). The paper does **not** require
every harness to deploy workloads itself; full adherence is achievable by a vendor-neutral,
fail-closed evidence protocol that CI/CD and runtime adapters use.

Current state: deterministic six-phase ratchet with human approvals; exit-0 git-pinned verify
evidence with no bypass flag; acceptance/review/submit surfaces; a CI PR action and repo CI/release
pipelines that publish the `specd` binary with checksums and SBOMs; a checksum-verifying installer;
`version --json` runtime identity; managed `AGENTS.md`/roles/steering with palette/config handshake
digests; MCP tool denial, Brain authority gating, and orchestration checkpoint/recover.
Gaps: no release/deployment ledger, environment registry, canary policy, post-deploy health gate,
rollback command/evidence, or production-observation feedback; CI verdict is not bound to an
artifact digest/SBOM/provenance/environment; the handshake does not bind binary release/commit,
state schema, or managed role/steering content in one preflight; `orchestrated` mode has no
reachable CLI path; the cost/deadline brake is not production-wired; installer has no staged atomic
swap, retained-previous binary, attestation, schema preflight, or rollback-on-smoke; one advertised
regression invariant fails open on missing input.

## Ownership

| Area | Domain 08 owns | Other domain owns |
|---|---|---|
| Release candidacy | immutable candidate identity (spec revision, HEAD, evidence-set digest, artifact digest, SBOM/provenance refs, bootstrap digest), `releases.jsonl` | Domain 04 verify/evidence completion authority; Domain 01 spec revision/approval |
| Deployment ledger | `deployments.jsonl`, delivery state machine, idempotency key, attempt monotonicity, crash-safe append | Domain 05 mission/lease/ACP controller recovery |
| Environment policy | closed environment names, per-environment strategy/approver/freshness/rollback-target rules, pure delivery gates | Domain 06 authority/attestation trust, Domain 01 approve transition |
| Adapter envelope | versioned stdin/file envelope schema, no-credential contract, idempotency, trust label | Domain 10 transport/adapter packaging; Domain 06 redaction |
| Canary/health/rollback | deterministic promotion/rollback verdict from fresh criteria, health evidence, rollback records + reports | Domain 07 measurement/telemetry trust, external monitoring |
| Agent bootstrap binding | one fail-closed packet binding binary/state/context/template/palette/config/managed identities | Domain 02 context manifest content; Domain 03 native guidance |
| Install/upgrade assurance | staged atomic swap, retained rollback binary, schema preflight, managed-asset diff preview, smoke-gated promotion | Domain 09 maintenance/modernization operating model |
| Incident feedback | bounded observation refs seeding a new spec, portfolio release/environment view | Domain 07 observation typing; Domain 09 incident lifecycle |

## Deliverable specs

| Wave | Slug | Result | Requires |
|---|---|---|---|
| W0 | `08a-delivery-assurance-baseline` | observed behavior, corrected docs wording, delivery-contract doc drafts, failing fixtures for every P0 gap | — |
| W1 | `08b-agent-bootstrap-binding` | one JSON packet binds binary/state/context/template/palette/config/managed identities; any pinned mismatch exits non-zero before mutation | 08a |
| W2 | `08c-orchestrated-mode-reachability` | supported CLI/config path enters `orchestrated` via CAS + approval; invalid mode fails schema validation; production cost/deadline brake wired | 08a, Domain 05 dispatch |
| W3 | `08d-delivery-envelopes-and-state-machine` | release/environment/deployment/health/rollback schema + explicit fail-closed transitions; offline canonical fixtures | 08a |
| W4 | `08e-installed-lifecycle-e2e-and-regression-prereqs` | `production-smoke.sh` runs the documented lifecycle from an empty repo; `regress-domains.sh` proves input exists before claiming a pass | 08b |
| W5 | `08f-release-install-upgrade-hardening` | staged atomic swap, retained previous binary, attestation/schema preflight, rollback-on-failed-smoke | 08b |
| W6 | `08g-release-and-deployment-ledgers` | `releases.jsonl`/`deployments.jsonl`, `release candidate` + `deploy` verbs, immutable/replayable/crash-safe, no task-gate crossover | 08d |
| W7 | `08h-environment-policy-and-delivery-gates` | environment policy config + pure delivery gates; production requires adapter/authority/artifact identity/freshness/rollback target | 08d,08g, Domain 06 authority |
| W8 | `08i-deployment-adapter-envelope` | versioned stdin/file adapter protocol, no implicit credentials, idempotent, malformed/untrusted rejected, zero new deps | 08g, Domain 10 adapter |
| W9 | `08j-canary-health-promotion-rollback` | canary cannot promote before full fresh window; failed canary requires rollback/exception; rollback complete only after target health passes; byte-identical reports | 08h,08i, Domain 07 measurement |
| W10 | `08k-ci-delivery-binding-and-attestation` | CI action binds source evidence to artifact + environment; artifact substitution fails digest check; fork PRs get no production credentials; attested identity | 08i, Domain 10 adapter |
| W11 | `08l-incident-portfolio-and-recovery-drills` | bounded incident/observation refs seed a new spec; portfolio release/environment view; N-1 upgrade + crash-boundary recovery drills; release proof E2E | 08j,08k, Domain 09 maintenance |

## DAG

```text
08a ─┬─> 08b ─┬─> 08e ───────────────────────────────────────┐
     │        └─> 08f ───────────────────────────────────────┤
     ├─> 08c ─────────────────────────────────────────────┐  │
     └─> 08d ─┬─> 08g ─┬─> 08h ─┐                          │  │
              │        └─> 08i ─┼─> 08j ─> 08l             │  │
              └────────────────>┘        ↑   ↑   ↑         │  │
                          08i ─> 08k ─────┘   │   │        │  │
Domain 05 dispatch ─> 08c ─────────────────────┘   │       │  │
Domain 06 authority ─> 08h                         │       │  │
Domain 07 measurement ─> 08j                       │       │  │
Domain 10 adapter ─> 08i, 08k                      │       │  │
Domain 09 maintenance ─> 08l <──────────────────────┴───────┴──┘
```

## Program rules

1. No LLM, network call, or agent prose in any delivery gate, state transition, report, or
   promotion/rollback verdict. Everything is a pure function of `.specd/` ledgers + validated
   adapter envelopes.
2. Task completion authority is untouched. Deployment, health, telemetry, or rollback evidence
   never substitutes for passing verify evidence at a git HEAD, and no delivery record can
   retroactively change `complete`.
3. Source completion and environment health are distinct states. `complete`/`submit` never means
   "deployed"; reports label them separately.
4. Delivery is additive and parallel — a separate ledger/domain, never a new value on the six
   lifecycle status states. The existing ratchet is unchanged whether delivery is used or not.
5. Release candidate identity is immutable and reproducible once created; artifact substitution
   after candidacy fails a digest check and cannot be waived by agent text.
6. Production policy cannot be relaxed by task text, agent prompt, or adapter response. Production
   requires an allowlisted environment/adapter, explicit authority/CI identity, artifact identity,
   fresh observation, and a rollback target; a local CLI flag is not production identity.
7. Adapters receive no implicit credentials from `specd`; provider credentials never enter
   `.specd/`. A JSON envelope is data, not proof — trust source/attestation is always visible and
   allowlisted. A duplicate idempotency key is a no-op or conflict, never a second deployment.
8. Missing, malformed, stale, or mismatched observation fails closed. A canary is never healthy by
   timeout default; a canary cannot promote before its full fresh window; "command issued" is not
   "rollback succeeded".
9. Install/upgrade stages and verifies before swapping: checksum/attestation, `version --json`
   commit, handshake, and schema preflight gate promotion; a retained previous binary auto-restores
   on failed smoke; unsupported future schema and unsafe downgrade fail before any write.
10. Stdlib-only, offline, deterministic core. External CI/CD, runtime, and attestation adapters
    produce pinned artifacts the core validates. No `reference/` edits. Delivery verbs/flags are
    declared once in `internal/core/commands.go`, derived into MCP/help, mirrored in
    `docs/command-reference.md` and `docs/CHEATSHEET.md`; high-risk production mutations stay out of
    the general MCP palette.

## Completion claim

The domain is complete when: (1) a coding agent can bootstrap with one fail-closed packet binding
every relevant identity and refuse a stale/wrong workspace before any mutation; (2) `orchestrated`
mode is reachable through a supported CAS/approval path with a production-wired cost/deadline brake;
(3) release candidacy, deployment attempts, environment policy, canary/health/promotion, and
rollback exist as immutable, replayable, offline ledgers with pure gates that never touch task
completion authority; (4) an optional stdin/file adapter protocol ingests bounded, attested,
idempotent envelopes with no implicit credentials; (5) the repo's own install/upgrade path stages,
verifies, atomically swaps, and rolls back on failed smoke, with schema preflight and managed-asset
diff preview; (6) production observations seed new specs by bounded reference without loading raw
logs, a portfolio view reports deployed/healthy/failed/rolled-back release per environment, and
N-1 upgrade + crash-boundary recovery drills pass; and every one of the 15 production validation
scenarios in `08-deployment-and-production-assurance.md` has a deterministic offline fixture.
