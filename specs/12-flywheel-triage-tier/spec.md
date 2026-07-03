# Spec 12 — Extended Loop / Flywheel (triage tier)

> **Authoring order:** 12 / 12 · **Critical path:** no (mostly deferred; v1 authors only the security gate)
> **Sources:** `fresh-start/12-flywheel-triage-tier.md`, paper p.30
> **ADRs:** ADR-4, ADR-5, ADR-6, ADR-8, ADR-9
> **Reference:** `reference/internal/cmd/{deploy,observe,eval,review,security,submit,ingest,migrate}.go`, `reference/internal/core/{eval,review,ingest,harness,deploy,observe,maintenance}.go`, `reference/internal/core/security/*`, `reference/docs/flywheel.md`

The far-right of the SDLC — maintenance/feedback. For a fresh MVP, **none of it is on the
critical path** from "write a spec" to "ship verified code". Default posture: **DEFER to v2 or
plugin.** This is the primary redundancy-shedding domain. v1 ships **no flywheel commands** —
only the `security` gate module (via the Spec 03 interface).

---

## 1. Purpose & principles
- **Principles served:** P3 (evidence gates extend into deploy/observe), P7 (deterministic
  records of maintenance actions).
- **Paper concept:** the maintenance/feedback phase — hooks + observability in live/near-live
  environments; the loop that turns production signal back into specs (p.30).

## 2. Verdicts (per command, justified against a core use case)

| Command | Verdict | Why / reference |
|---|---|---|
| `security` | **KEEP-as-plugin-gate** | Stdlib-only, deterministic, off by default; fits Spec 03 exactly. `reference/internal/core/security/*` |
| `eval` | **DEFER** (keep Gate 10 hook) | Rubric command is plugin/v2; gate proves the interface |
| `review` | **DEFER** (keep Gate 11 hook) | Human approval covers MVP semantic-gate need (Spec 02) |
| `deploy` | **DEFER** | Maintenance-phase; preserve `DeployApproval` evidence shape |
| `observe` | **DEFER** | Feedback-phase; no MVP dependency |
| `ingest` | **DEFER** | Brownfield onboarding + Gate 13; strong v1.1, not MVP |
| `harness` | **DEFER** | Real value (quarantine, SHA256 pinning) but needs a bundle ecosystem |
| `submit` | **CUT** | No harness value over a user script |
| `migrate` | **CUT** (from MVP) | No legacy to migrate in a fresh tree (ADR-2) |
| `maintenance` / program tier | **DEFER** (with Spec 09 program tier) | ADR-9 |

**Minimal accurate surface (v1):** *no commands.* Only `internal/core/gates/security/*`
registered through the Spec 03 interface, plus dormant gate hooks for eval/review so the
interface stays honest.

## 3. Requirements (EARS)
- **R12.1** The system shall not ship `deploy`, `observe`, `eval`, `review`, `ingest`,
  `harness`, `submit`, or `migrate` as commands in the v1 MVP.
- **R12.2** When security scanning is enabled, the system shall run it as a registered gate
  through the Spec-03 gate interface, not as a standalone command.
- **R12.3** When a deferred flywheel module returns, the system shall integrate it only via the
  pluggable-gate interface (ADR-4) and the `state.records` extension map (ADR-6), without
  adding a core schema field or a hardcoded `check` branch.
- **R12.4** When security findings require an allowlist entry, the system shall reject any
  entry lacking a reason.
- **R12.5** The system shall preserve documented evidence shapes for deferred features so their
  v2 reintroduction requires no contract change.

## 4. Design

### Module boundaries
- v1 ships only `internal/core/gates/security/*` (registered gate). Deferred modules are
  documented as future `internal/plugins/<name>` implementing the `Gate` interface and/or
  writing `state.records[<name>]`.

### Key types (retained as documented contracts, not v1 code)
- `DeployApproval`, `EvalSummary`, `ReviewVerdict`, inventory `waivers`.

### On-disk contracts
- `.specd/security/allow.json` (reason mandatory). Future modules write only under
  `state.records` and their own `.specd/<feature>/` dirs.

### External interfaces
- The `Gate` interface (Spec 03); `state.records` (Spec 02) — the **only two re-entry seams**.

## 5. Invariants preserved (ADR-8)
Opt-in gates byte-identical when off (Spec 03); human approval final for review; deploy
preconditions read evidence only, never re-run gates; harness import quarantine + SHA256
pinning (when the feature returns).

## 6. Cross-domain dependencies
- Depends on: Spec 03 (gate interface — the only v1 entry point), Spec 02 (`state.records`).
- Related: Spec 09 (program-tier maintenance defers together), Spec 05 (deploy/eval consume
  evidence when they return).

## 7. Risks & open questions
- **Risk:** deferring makes specd look "less capable" than the current tree. → frame v1 as the
  *harness core*; the flywheel is a documented, contract-stable plugin roadmap — subtraction
  with a re-entry plan, not amputation.
- **Open:** is any flywheel command a genuine MVP dependency for a design partner? If a
  concrete user needs (say) `deploy` approval on day one, promote *only that one* from DEFER to
  a late wave — the gate/records seams already support it.
