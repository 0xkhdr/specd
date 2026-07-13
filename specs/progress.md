# progress.md — flat wave-by-wave execution checklist

Single source of truth for **build order across all ten domains**. Each domain spec
(`specs/0N-.../tasks.md`) owns its own wave DAG and evidence-gated tasks; this file coordinates
the **cross-domain sequencing** those specs only describe in prose, and tracks program-level
status. Read this together with `specs/prompt.md` (the per-turn implementation protocol).

This is the **flat DAG walk order**: work top to bottom, stage by stage; within a stage, waves
listed are parallel-eligible unless a "needs" note says otherwise. Tick `[x]` only when the wave's
own `tasks.md` rows are all `[x]`, its validator task is green (§ Definition of done), **and** the
user has reviewed and confirmed the wave (see `specs/prompt.md`). The checkboxes in each domain's
`tasks.md` are the fine-grained truth; this file is the program rollup — keep both in sync.

Re-run `./scripts/regress-all.sh` after any wave flips to `[x]`.

---

## Stage P0 — truthful contracts + boundary keystone

- [x] 01 W0 — Contract and baseline
- [x] 01 W1 — Design and planning guidance (needs 01 W0)
- [x] 01 W2 — Task trace/risk contract (needs 01 W0,W1)
- [x] 02 W0 — contract, fixtures, migration
- [x] 02 W1 — typed v2 foundation (needs 02 W0)
- [x] 02 W2 — required lanes and driver contract (needs 02 W1, 01 task metadata)
- [x] 03 W0 — contract baseline
- [x] 03 W1 — truthful paths, scaffold, resolution, doctor (needs 03 W0)
- [x] 03 W2 — driver projection (needs 03 W1, 02 manifest v2)
- [x] 04 W0 — baseline and contract decision
- [x] 04 W1 — evidence envelope and declaration (needs 04 W0, 01 task metadata)
- [x] 05 W0 — inventory, wording, contract baseline
- [x] 05 W1 — mission and pending dispatch (needs 05 W0, 03 dispatch envelope)
- [x] 05 W2 — worker lifecycle and normal completion (needs 05 W1, 04/06 contracts)
- [x] 06 W0 — inventory, wording, contract baseline
- [x] 06 W1 — operating profiles and required gates (needs 06 W0, 01)
- [x] 06 W2 — declared scope from harness diff (needs 06 W1, 05 report)
- [x] 06 W3 — role authority packets and tool policy (needs 06 W2, 05 mission)
- [x] 07 W0 — inventory, wording, contract baseline
- [x] 08 W0 — `08a-delivery-assurance-baseline`
- [x] 09 W0 — `09a-maintenance-baseline`
- [x] 10 W0 — baseline and boundary invariant
- [x] 10 W1 — envelope, identity, classification (10c freezes only after 04/05/07/08
      record P0 adapter field demands against 10 W0's inventory)

**Exit P0:** all rows above `[x]` and confirmed by the user.

---

## Stage P1 — operational and economic substrate

- [x] 04 W2 — import, freshness, trajectory (needs 05 event identity, 06 profile)
- [x] 04 W3 — coverage and gate composition (needs 04 W2)
- [x] 05 W3 — recovery, cancellation, conformance (needs 05 W2)
- [x] 05 W4 — routing, limits, observation (needs 05 W2, 06/07 policy)
- [x] 06 W4 — context and change-boundary scan (needs 06 W3)
- [x] 06 W5 — mandatory sandbox and secret isolation (needs 06 W4)
- [x] 07 W1 — versioned run/telemetry envelope (needs 02, 05, 06)
- [x] 07 W2 — run correlation and attempt identity (needs 07 W1)
- [x] 07 W3 — context accounting and sufficiency (needs 07 W2, 02)
- [x] 07 W4 — honest cost brake (needs 07 W3, 05)
- [x] 07 W5 — privacy and cardinality policy (needs 07 W4, 06)
- [x] 07 W6 — metadata run spans and trace export (needs 07 W5)
- [x] 06 W6 — dependency and dangerous-change governance (needs 07 export)
- [x] 06 W7 — governed exceptions and mission audit (needs 06 W6)
- [x] 08 W1 — `08b-agent-bootstrap-binding` (needs 08 W0)
- [x] 08 W2 — `08c-orchestrated-mode-reachability` (needs 08 W0, Domain 05 dispatch)
- [x] 08 W3 — `08d-delivery-envelopes-and-state-machine` (needs 08 W0)
- [x] 08 W4 — `08e-installed-lifecycle-e2e-and-regression-prereqs` (needs 08 W1)
- [x] 08 W5 — `08f-release-install-upgrade-hardening` (needs 08 W1)
- [x] 08 W6 — `08g-release-and-deployment-ledgers` (needs 08 W3)
- [x] 08 W7 — `08h-environment-policy-and-delivery-gates` (needs 08 W3,W6, Domain 06 authority)
- [x] 10 W2 — runner and capability inspection (needs 10 W1)
- [x] 10 W3 — offline continuity and conformance (needs 10 W2)

**Exit P1:** verification/orchestration/security/observability/deployment ledgers exist and
fail closed; reference adapters pass the conformance suite; core stays green with no adapters.

---

## Stage P2 — ecosystem and portfolio scale

- [x] 01 W3 — Coverage gates (needs 01 W2)
- [x] 01 W4 — Amendment staleness (needs 01 W3)
- [x] 01 W5 — Production profile (needs 01 W4)
- [x] 01 W6 — Bounded spikes (needs 01 W4,W5)
- [x] 01 W7 — Conformance and reports (needs 01 W1–W6)
- [x] 02 W3 — progressive static lanes (needs 02 W2)
- [x] 02 W4 — portable skills (needs 02 W3)
- [x] 02 W5 — receipts and durable knowledge (needs 02 W4)
- [x] 02 W6 — conformance and release proof (needs 02 W5)
- [x] 03 W3 — drift, context metadata, handoff (needs 03 W2)
- [x] 03 W4 — host conformance and capabilities (needs 03 W3, 10)
- [x] 03 W5 — remote envelope, release proof (needs 03 W4)
- [x] 04 W4 — adapters, dataset/rubric governance (needs 04 W3, 10)
- [x] 04 W5 — quality packet, review, flywheel, release proof (needs 04 W4, 02/07/09)
- [x] 05 W5 — adapters and release proof (needs 05 W3,W4, 10)
- [x] 06 W8 — cross-platform adapters and release proof (needs 06 W7, 10)
- [x] 07 W7 — provider-neutral annotation expansion (needs 07 W6)
- [x] 10 W4 — ecosystem mappings (needs 10 W3)
- [x] 07 W8 — neutral event schema and context efficiency (needs 07 W7, 10)
- [x] 07 W9 — attested ingestion, routing, roll-ups, release proof (needs 07 W8, 10)
- [x] 08 W8 — `08i-deployment-adapter-envelope` (needs 08 W6, Domain 10 adapter)
- [ ] 08 W9 — `08j-canary-health-promotion-rollback` (needs 08 W7,W8, Domain 07 measurement)
- [ ] 08 W10 — `08k-ci-delivery-binding-and-attestation` (needs 08 W8, Domain 10 adapter)
- [ ] 08 W11 — `08l-incident-portfolio-and-recovery-drills` (needs 08 W9,W10, Domain 09)
- [ ] 09 W1 — `09b-successor-link-kinds` (needs 09 W0)
- [ ] 09 W2 — `09c-typed-intake-provenance` (needs 09 W0)
- [ ] 09 W3 — `09d-decision-exception-lifecycle` (needs 09 W0, Domain 06 authority)
- [ ] 09 W4 — `09e-memory-provenance-and-aging` (needs 09 W0, Domain 02 context)
- [ ] 09 W5 — `09f-maintenance-templates` (needs 09 W1,W2)
- [ ] 09 W6 — `09g-drift-projection` (needs 09 W2,W3, Domain 04 evidence)
- [ ] 09 W7 — `09h-recurring-invariants` (needs 09 W2, Domain 07 measurement)
- [ ] 09 W8 — `09i-incident-successor-and-prevention` (needs 09 W1,W5, Domain 08 observation)
- [ ] 09 W9 — `09j-portfolio-governance-status` (needs 09 W1,W3)
- [ ] 09 W10 — `09k-memory-conflict-lint` (needs 09 W4)
- [ ] 09 W11 — `09l-org-adoption-and-archive` (needs 09 W8,W9,W10, Domain 10 boundary)
- [ ] 10 W5 — release/feedback contract and proof (needs 10 W4, 08)

**Exit P2 / program done:** every domain's final validator wave is green against a fresh
release binary and confirmed by the user.

---

## Definition of done (per wave)

A wave is `done` only when **all** hold:

1. Every task row in that wave is `[x]` in the spec's `tasks.md`, each backed by a passing
   `specd verify` record (exit 0 pinned to a real git HEAD) — no bypass.
2. The wave's validator/`verify` command passes on a freshly built binary.
3. `gofmt -l .` empty, `go vet ./...` clean, `go mod tidy` no diff, `./scripts/docs-lint.sh` and
   `./scripts/test-lint.sh` green; `./scripts/regress-domains.sh` invariant holds.
4. No previously `done` wave regressed (`./scripts/regress-all.sh`).
5. The user has reviewed the implementation against best practices and explicitly confirmed —
   only then is the row here and in `tasks.md` marked `[x]` (see `specs/prompt.md`).

The domain is done when its final validator task is green against a fresh release binary and its
README completion claim is fully demonstrated. The **program** is done when all ten domains are
done.

---

## How to use this file

1. Pick the first unchecked row in stage order.
2. Implement the wave fully, task-by-task, per `specs/prompt.md`.
3. Stop and present the wave to the user for review — do not check any box yet.
4. Only after the user confirms: check the row here, flip the domain's `tasks.md` rows `[x]`.
5. Repeat until every row in every stage is checked.
