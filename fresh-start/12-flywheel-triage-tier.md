# Domain: Extended Loop / Flywheel (triage tier)

## 1. Purpose & value mapping
- **Principles served:** P3 (evidence gates extend into deploy/observe), P7 (deterministic
  records of maintenance actions). Realizes the paper's **maintenance/feedback phase**
  ("Code Review, Deployment, & Maintenance — Observing the Harness," p.30).
- **Paper concept realized:** the far-right of the SDLC — hooks + observability in live/
  near-live environments; the feedback loop that turns production signal back into specs.
- **Core use case (honest test):** most of this tier serves teams already running specd in
  production across many specs. For a *fresh MVP*, none of it is on the critical path from
  "write a spec" to "ship verified code." **Default posture: DEFER to v2 or plugin.**
- **If none → CUT:** several here are CUT outright; the rest are DEFER. This is the primary
  redundancy-shedding domain.

## 2. Current-state analysis (from specd)
- **Reference files read:** `internal/cmd/{deploy.go,observe.go,eval.go,review.go,
  security.go,submit.go,ingest.go,migrate.go}`, `internal/core/{deploy.go,observe.go,
  eval.go,review.go,ingest.go,harness.go,migrate.go,maintenance.go}`,
  `internal/core/security/*`, `docs/validation-gates.md` (gates 8–13, deploy/harness
  preconditions), `docs/flywheel.md`.
- **What exists today; key contracts/invariants:**
  - `eval.go` (17.7K core) — rubric engine + `EvalSummary`; Gate 10 blocks approve until a
    passing eval is recorded (opt-in, off by default).
  - `review.go` — `ReviewVerdict`; Gate 11 requires a fresh approving `review_report.md`
    (opt-in). Human approval stays final; the report is evidence.
  - `security/*` — stdlib-only scanners (secrets/injection/slopsquat), Gate 12
    (`check --security`), off by default; allowlist requires a reason.
  - `deploy.go` — `DeployPreconditions`: refuses unless spec complete, required gates green,
    and (production) a human deploy-approval record exists. Reads evidence only; never
    re-runs a gate.
  - `observe.go` — offline error-payload correlation with byte caps.
  - `ingest.go` — brownfield inventory (`inventory.json`) + Gate 13 coverage (every
    inventoried file mapped or waived-with-reason).
  - `harness.go` (20.5K core) — shareable/versioned policy bundles with SHA256 pinning and
    **import quarantine** (executables quarantined until `harness enable`).
  - `migrate.go` — idempotent spec-state migration; never writes policy.
  - `submit.go` — thin timeout wrapper around a configured submit command.
  - `maintenance.go` — recurring maintenance programs (program tier, `WithProgramLock`).
- **Redundancy / complexity / drift found (evidence):**
  - This tier is the bulk of the "wide" surface: 8 commands + ~90K of core, most gated off by
    default. It realizes a real vision but is not load-bearing for the MVP loop.
  - `submit` is a 115-LOC wrapper with no harness value beyond a user's own script.
  - `migrate` exists to move legacy schema forward — irrelevant to a fresh tree.
  - `promote` (memory) leaked into `eval.go` — a drift symptom of the tier's sprawl.

## 3. Fresh-start decision (per command, justified against a core use case)
- **`security` → KEEP-as-plugin-gate.** Stdlib-only, deterministic, off by default — it fits
  the pluggable-gate interface (domain 03) exactly and needs no command. The one flywheel
  piece that is genuinely core-shaped.
- **`eval` → DEFER (keep Gate 10 hook).** The gate module stays registrable so the interface
  is proven; the rubric *command* is a plugin/v2.
- **`review` → DEFER (keep Gate 11 hook).** Same pattern; human approval already covers the
  MVP's semantic-gate need (domain 02).
- **`deploy` → DEFER.** Valuable production-approval discipline, but a maintenance-phase
  concern. Preserve the `DeployApproval` evidence shape for when it returns.
- **`observe` → DEFER.** Feedback-phase; no MVP dependency.
- **`ingest` → DEFER.** Brownfield onboarding + Gate 13; a strong v1.1 feature, not MVP.
- **`harness` → DEFER.** Real security value (quarantine, SHA256 pinning), but presupposes a
  bundle ecosystem that does not exist at MVP.
- **`submit` → CUT.** No harness value over a user script.
- **`migrate` → CUT (from MVP).** No legacy to migrate in a fresh tree; reintroduce when the
  v1 schema first evolves.
- **`maintenance` / program tier → DEFER** (with domain 09's program tier).
- **Minimal accurate surface (this domain, v1):** *no commands.* Only the `security` gate
  module registered through domain 03's interface, plus retained (dormant) gate hooks for
  eval/review so the interface stays honest.
- **Architecture & flexibility improvements:**
  - **Everything here re-enters through two seams only:** the pluggable-gate interface
    (domain 03) and the `state.records` extension map (domain 02). No flywheel feature may
    add a core schema field or a hardcoded check branch. This makes the whole tier a set of
    optional modules rather than baked-in surface — the exact opposite of today.
  - **Deferred, not deleted:** keep the evidence *shapes* (`DeployApproval`, `EvalSummary`,
    inventory coverage) documented so v2 modules slot in without re-litigating contracts.

## 4. Requirements (EARS-shaped) — seed for requirements.md
1. The system shall not ship `deploy`, `observe`, `eval`, `review`, `ingest`, `harness`,
   `submit`, or `migrate` as commands in the v1 MVP.
2. When security scanning is enabled, the system shall run it as a registered gate through
   the domain-03 gate interface, not as a standalone command.
3. When a deferred flywheel module returns, the system shall integrate it only via the
   pluggable-gate interface and the `state.records` extension map, without adding a core
   schema field or a hardcoded `check` branch.
4. When security findings require an allowlist entry, the system shall reject any entry
   lacking a reason.
5. The system shall preserve documented evidence shapes for deferred features so their v2
   reintroduction requires no contract change.

## 5. Design notes — seed for design.md
- **Module boundaries:** v1 ships only `internal/core/gates/security/*` (registered gate).
  Deferred modules are documented as future `internal/plugins/<name>` implementing the
  `Gate` interface and/or writing `state.records[<name>]`.
- **Key types (retained as documented contracts, not v1 code):** `DeployApproval`,
  `EvalSummary`, `ReviewVerdict`, inventory `waivers`.
- **Data/on-disk contracts:** `.specd/security/allow.json` (reason mandatory); future
  modules write only under `state.records` and their own `.specd/<feature>/` dirs.
- **Invariants to preserve:** opt-in gates byte-identical when off (domain 03); human
  approval final for review; deploy preconditions read evidence only, never re-run gates;
  harness import quarantine + SHA256 pinning (when the feature returns).
- **External interfaces:** the `Gate` interface (domain 03); `state.records` (domain 02).

## 6. Proposed task DAG — seed for tasks.md

### Wave 1 — security gate only (the sole v1 deliverable of this domain)
| id | role | files | depends-on | verify | acceptance |
|---|---|---|---|---|---|
| T12.1 | craftsman | `internal/core/gates/security/scanners.go` | domain 03 (T3.1) | `go test ./internal/core/gates/security -run TestScanners` | secrets/injection/slopsquat deterministic |
| T12.2 | craftsman | `internal/core/gates/security/allow.go` | T12.1 | `go test ./... -run TestAllowlistReasonRequired` | reasonless allowlist entry is a hard error |
| T12.3 | craftsman | `docs/deferred-flywheel.md` | — | `grep -q DeployApproval docs/deferred-flywheel.md` | evidence shapes documented for v2 |
| T12.4 | validator | `internal/core/gates/security/off_test.go` | T12.1 | `go test -run TestSecurityOffByDefault` | check output unaffected when security off |

## 7. Risks, open questions, cross-domain dependencies
- **Risk:** deferring the flywheel makes specd look "less capable" than the current tree.
  Mitigation: frame v1 as the *harness core*; the flywheel is a documented, contract-stable
  plugin roadmap — subtraction with a re-entry plan, not amputation.
- **Open question:** is any flywheel command a genuine MVP dependency for a design partner?
  If a concrete user needs (say) `deploy` approval on day one, promote *only that one* from
  DEFER to a late wave — the gate/records seams already support it.
- **Cross-domain deps:** domain 03 (gate interface is the only v1 entry point), domain 02
  (`state.records`), domain 09 (program-tier maintenance defers together), domain 05
  (deploy/eval consume evidence when they return).
