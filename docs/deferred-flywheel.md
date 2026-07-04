# Deferred Flywheel — evidence shapes & re-entry seams

> **ADR-5 (subtractive bias).** The "learning flywheel" / extended-loop tier (deploy approvals,
> eval gating, inventory waivers) is **cut from the shipped surface** but not from the design. It
> was accreted mass in v1; re-adding it now would violate the minimal-core north star. This doc
> records what was deferred and the two seams through which it re-enters **without** a refactor,
> so the cut is reversible by data, not by rewrite. Subtractive-bias rule: unsure = DEFER,
> recorded — this file is that record.

## Deferred evidence shapes

These are evidence record shapes intentionally **not** produced by any shipped verb. When the
flywheel tier is reinstated, each is written into `state.records` under its own key (see seam 2):

- **`DeployApproval`** — a human/CI attestation that a build was approved for an environment
  (who, when, target, the approved artifact digest). The gate that would consume it: "no promote
  without a recorded DeployApproval for this revision."
- **`EvalSummary`** — a deterministic roll-up of an evaluation suite (pass/fail counts, thresholds,
  the command + exit code that produced it). Gates it on: "no completion of an eval-tagged task
  without an EvalSummary meeting threshold." Computed, never model-authored (P3/P7).
- **Inventory waivers** — recorded, reason-bearing exceptions to an inventory/security finding
  (finding id, justification, approver, expiry). Mirrors the shipped security-allowlist rule that
  a waiver **requires a reason**; the flywheel generalizes it to inventory findings.

Each shape stays a plain JSON record so it round-trips through the existing evidence discipline
(append-only, exit-code + git-HEAD stamped) with no new storage engine.

## The two re-entry seams (already in the tree)

The deferred tier re-enters through two seams that **already exist**, so reinstating it adds
handlers, not architecture:

1. **The `Gate` interface** — `internal/core/gates/registry.go` (`type Gate interface`). New
   flywheel gates (deploy-approval, eval-threshold, inventory-waiver) register as ordinary gates
   in the registry; the check/verify pipeline runs them like any core gate. No change to the gate
   runner is required.
2. **`state.records`** — `internal/core/state.go` (`State.Records map[string]json.RawMessage`).
   Every deferred evidence shape above is a value in this map keyed by kind. The map already
   exists, is CAS-guarded, and persists through atomic writes — so `DeployApproval` / `EvalSummary`
   / waiver records land with zero schema migration.

## Why deferred, not deleted

Deleting the seams would make re-entry a refactor; keeping them (an empty map + an open interface)
costs nothing and honors ADR-5: the flywheel is a **data** decision the team can reverse later by
writing records and registering gates, never a re-architecture. Restoring the flywheel to satisfy
an unrelated finding is explicitly out of bounds (review-specs global guardrails, F7).
