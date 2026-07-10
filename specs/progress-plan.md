# Program progress plan — Google SDLC alignment

Single source of truth for **build order across all ten domains**. Each domain spec
(`specs/0N-.../`) owns its own wave DAG and evidence-gated tasks; this file coordinates the
**cross-domain sequencing** those specs only describe in prose, tracks program-level status, and
records the gap analysis that produced Domain 10 and this plan.

Read this with `progress-prompt.md` (the per-turn implementation protocol). The alignment intent,
verification ladder (L0–L7), and P0/P1/P2 framing come from
`docs/google-sdlc-alignment/README.md`.

---

## 1. Gap analysis (plan → spec)

Each domain plan in `docs/google-sdlc-alignment/` was compared against its implementation spec in
`specs/`. The nine existing specs (01–09) are mature: each cites its plan's P0/P1/P2, carries an
ownership matrix, a wave DAG, EARS requirements, program rules, and a completion claim mapped to
the plan's validation scenarios. The gaps were structural and cross-cutting, not per-requirement:

| # | Gap | Severity | Resolution |
|---|---|---|---|
| G1 | **Domain 10 had no spec**, yet `03f/03g, 04f/04g, 05f, 06i, 07i/07j, 08i/08k/08l, 09l` all list "Domain 10 adapter/boundary contract" as a prerequisite. Those late waves had an unresolvable dependency. | Critical | Authored `specs/10-scope-boundaries-and-interoperability/` (README/requirements/design/tasks). |
| G2 | **No program-level build order.** Cross-domain dependencies lived only as prose in each README ("Domain 05 mission → 06c"). No single file sequenced the global wave order or let an agent pick the next eligible wave. | High | This file (§3–§5) plus `progress-prompt.md`. |
| G3 | **Domain 10 is numbered last but is a P0 foundation.** Its common envelope is consumed by every adapter wave in 03–09, and its plan says freeze it from those domains' field demands. If built last, the adapter waves either block or invent divergent envelopes. | High | Pulled `10a–10e` into program stage **P0** (§3); `10c` freezes only after 04/05/07/08 record their P0 field demands (spec 10 `T01`, cross-wave rule). |
| G4 | **Zero-runtime-dependency invariant enforced only by convention.** No static test rejects a provider/OTel/deploy SDK import in trusted core. | Medium | Spec 10 `R1` + `10b-core-import-invariant` (static import/`go.mod` test). |
| G5 | **Data-classification boundary undefined.** Multiple domains redact secrets, but no shared taxonomy said which `.specd/` fields may cross a process/network/CI/telemetry boundary. | Medium | Spec 10 `R4` + `10e`; Domain 06 enforces, Domain 10 defines the taxonomy. |
| G6 | Spec 02 uses a "Release slices" bullet instead of the lettered deliverable table the other nine specs use. | Low (cosmetic) | Left as-is; its task waves `W0–W6` are unambiguous. Noted here rather than churning a mature spec. |

**Deliberately not changed:** the nine existing specs' requirements, designs, and task DAGs. They
are internally consistent and faithful to their plans; inventing edits would risk their consistency
for no correctness gain (subtractive bias). All identified gaps are addressed by adding Domain 10
and these two coordination documents, which is where the real missing surface was.

---

## 2. Domain map

| Domain | Spec dir | Role in program |
|---|---|---|
| 01 | `01-lifecycle-and-structured-intent` | Deterministic planning spine: requirement/design/task contract, coverage, amendment staleness, profiles. |
| 02 | `02-context-knowledge-and-skills` | Typed context manifest, budget, receipts, portable skills, memory provenance. |
| 03 | `03-agent-tool-driving-and-native-guidance` | Self-describing driver: legal next-action, guidance drift, MCP handoff, host conformance. |
| 04 | `04-verification-evals-and-quality` | Evidence classes, eval/trajectory contracts, freshness, risk coverage, dataset governance. |
| 05 | `05-orchestration-multi-agent-and-model-routing` | Worker claim/lease/heartbeat/report lifecycle, mission pinning, capability routing. |
| 06 | `06-security-permissions-and-governance` | Deterministic authority/scope/sandbox/exception enforcement, policy digest. |
| 07 | `07-observability-cost-and-operational-economics` | Run/span telemetry, honest cost, context sufficiency, privacy/cardinality, export. |
| 08 | `08-deployment-and-production-assurance` | Release/deploy/environment/canary/rollback ledgers, bootstrap binding, install hardening. |
| 09 | `09-maintenance-modernization-and-operating-model` | Successor links, typed intake, decision/memory lifecycle, drift, recurring invariants, portfolio. |
| 10 | `10-scope-boundaries-and-interoperability` | Boundary invariant, common adapter envelope, identity, classification, runner, conformance, A2A/MCP/OTel. |

---

## 3. Program build order

Wave-level topological order aligned to the alignment README's P0/P1/P2. **Domains couple only at
their late waves**; every domain's baseline (`Xa`) and early waves are independent and may run in
parallel. A wave is eligible when its **local `depends-on` evidence has passed** *and* its
**cross-domain "Requires" (per its README) are complete** in the ledger (§5).

### Stage P0 — establish truthful contracts and the boundary keystone

Goal (README P0): "dispatch" has one meaning, a real worker owns every lease, no result completes
outside the correlated mission/evidence chain, and the deterministic boundary is enforced.

1. **All baselines in parallel:** `01a, 01b, 02·W0, 03a, 04a, 05a, 06a, 07a, 08a, 09a, 10a`
   (read-only inventories + RED fixtures; no cross-domain code deps).
2. **Boundary keystone (early, not last):** `10b` (import invariant), then `10c` (common envelope)
   — `10c` freezes after `04/05/07/08` record their adapter field demands in `10a`'s inventory —
   then `10d` (identity), `10e` (classification).
3. **Lifecycle spine:** `01c → 01d → 01e → 01f` (design/guidance/task-trace/coverage) and
   `02·W1–W2` (manifest V2 + driver contract), `03b/03c → 03d` (truthful context, driver
   projection). `04b` (evidence envelope) after `01` task metadata.
4. **Execution identity:** `05b` (mission + pending dispatch, needs `03` envelope) → `05c` (worker
   claim/heartbeat/report). `06b` (profiles/required gates), `06c` (declared scope from `05` report),
   `06d` (authority packets from `05` mission).

**Exit P0:** `01a–01f, 02·W0–W2, 03a–03d, 04a–04b, 05a–05c, 06a–06d, 10a–10e` complete.

### Stage P1 — operational and economic substrate

Goal (README P1): observable run events with trusted/unknown provenance, capability routing with
budget/deadline brakes, eval/trace/deploy/rollback/feedback adapter contracts, governed exceptions
and drift.

1. **Complete verification & orchestration:** `04c/04d/04e` (import/freshness/completion,
   trajectory, risk coverage — need `05` event identity + `06` profile), `05d/05e` (recovery,
   routing/brakes), `06e/06f` (context scan, mandatory sandbox).
2. **Observability & security governance:** `07b–07h` (envelope, run identity, context accounting,
   honest cost brake, privacy, spans, provider annotation — need `02/05/06`), `06g/06h`
   (dependency governance, exceptions ledger — needs `07` export).
3. **Deployment core:** `08b/08c` (bootstrap binding, orchestrated mode + brake), `08d–08h`
   (delivery envelopes, install hardening, ledgers, environment gates — need `05/06`).
4. **Reference adapter seams:** `10f/10g` (runner, capability doctor), `10h/10i`
   (offline-continuity proof, conformance suite).

**Exit P1:** verification, orchestration, security, observability, and deployment ledgers exist and
fail closed; reference adapters run through the conformance suite; core stays green with no adapters.

### Stage P2 — ecosystem and portfolio scale

Goal (README P2): A2A round-trip without weakening authority/evidence, portable skills, cross-host
conformance, canary/rollback suites, portfolio governance, optional org templates/dashboards.

1. **Adapter-consuming finales:** `03f/03g` (host conformance, remote dispatch), `04f/04g`
   (eval adapter, quality flywheel), `05f` (A2A/adapter conformance), `06i` (cross-platform
   adapters), `07i/07j` (neutral event schema, attested ingestion/rollups),
   `08i–08l` (deploy adapter, canary/rollback, CI binding, incident/portfolio).
2. **Maintenance & operating model:** `09b–09l` (successor links, intake, lifecycle, drift,
   recurring invariants, incident successor, portfolio governance, archive).
3. **Ecosystem mappings:** `10j/10k/10l/10m` (A2A/MCP, OTel, release/feedback, versioning + release
   proof).

**Exit P2 / program done:** every domain's validator wave (final `T`) is green against a fresh
release binary, and the L0–L7 ladder (§4) is satisfied end-to-end.

---

## 4. Verification ladder mapping (L0–L7)

"Production-ready" is an evidence profile, not a label. Each ladder level is proven by specific
waves; a level is claimable only when its waves are green.

| Level | Proven by | Representative waves |
|---|---|---|
| L0 Core integrity | Existing CI: build, unit/integration/`-race`, `gofmt`/vet/lint, repeated-order, regress scripts | every wave's `verify`; `regress-all.sh`, `regress-domains.sh` |
| L1 Fresh-project lifecycle | Installed binary drives a clean repo through every phase | `01j`, `08e` (`production-smoke.sh`), `10h` |
| L2 Agent/host conformance | No-prior-knowledge agent gets correct context/actions per host | `02·W6`, `03f`, `05d`, `10i` |
| L3 Failure & recovery | Crash/concurrency/timeout/stale/retry preserve safety | `01g`, `05d`, `07c`, `08l`, `10h` |
| L4 Authority/security | Roles/scope/tools/sandbox/context/exceptions fail closed | `06b–06i`, `10b`, `10e` |
| L5 Quality/evals | Tests + output/trajectory evals + rubrics/datasets + freshness compose | `04c–04g`, `01h` |
| L6 Release assurance | CI/CD identity, health, observation, rollback correlated & recoverable | `08d–08l`, `07j`, `10l` |
| L7 Long-running operation | Drift, recurring invariants, incidents, aging, portfolio scale | `09b–09l`, `07j`, `10m` |

Note the alignment README records a known **W0 regression defect** meaning L0 is not yet wholly
clean despite a passing process exit; treat closing that as a precondition to claiming L0.

---

## 5. Program ledger (rollup)

Fine-grained truth is the `[ ]`/`[x]` checkboxes in each spec's `tasks.md` (evidence-gated). This
table is the **program rollup** — update a domain's stage cell when all its waves in that stage pass
its validator. Status values: `todo` / `wip` / `done` / `blocked(<reason>)`.

| Domain | P0 waves | P1 waves | P2 waves | Notes |
|---|---|---|---|---|
| 01 | todo | todo | todo | Spine; unblocks 02/03/04/06. |
| 02 | todo | todo | todo | Needs 01 task metadata for exact selectors. |
| 03 | todo | todo | todo | Needs 01 roles/phase, 02 V2; 03f needs 10. |
| 04 | todo | todo | todo | 04d needs 05 event id; 04f needs 10 adapter. |
| 05 | todo | todo | todo | 05b needs 03 envelope; 05f needs 10. |
| 06 | todo | todo | todo | Needs 01/02/04/05; 06h needs 07; 06i needs 10. |
| 07 | todo | todo | todo | Needs 02/05/06; 07i/07j need 10. |
| 08 | todo | todo | todo | Needs 05/06/07; 08i/08k need 10; 08l needs 09. |
| 09 | todo | todo | todo | Needs 06/02/04/07/08; 09l needs 10. |
| 10 | todo | todo | todo | **Keystone.** P0 (10a–10e) unblocks all adapter waves. |

**Blocked-wave log** (append when a wave cannot start; clear when its cross-domain prereq lands):

| Date | Wave | Blocked on | Cleared |
|---|---|---|---|
| — | — | — | — |

---

## 6. Definition of done (per wave)

A wave is `done` only when **all** hold:

1. Every task row in that wave is `[x]` in the spec's `tasks.md`, each backed by a passing
   `specd verify` record (exit 0 pinned to a real git HEAD) — no bypass.
2. The wave's validator/`verify` command passes on a freshly built binary.
3. `gofmt -l .` empty, `go vet ./...` clean, `go mod tidy` no diff, `./scripts/docs-lint.sh` and
   `./scripts/test-lint.sh` green; `./scripts/regress-domains.sh` invariant holds.
4. The program ledger (§5) cell and, if applicable, the blocked-wave log are updated in the same
   change.
5. No previously `done` wave regressed (`./scripts/regress-all.sh`).

The domain is `done` when its final validator task is green against a fresh release binary and its
README completion claim is fully demonstrated. The **program** is done when all ten domains are
`done` and the L0–L7 ladder (§4) is satisfied.
