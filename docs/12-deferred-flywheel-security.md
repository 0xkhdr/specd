# Extended Loop / Flywheel (Triage Tier)

This document describes the triaged status of the extended loop (flywheel) features in `specd` (v2), the re-entry contracts for deferred modules, and the pluggable security gate.

---

## 1. Subtractive Triage (v2 Posture)

To focus the MVP on process enforcement, the vast majority of the post-build feedback loop is deferred:

*   **`security`:** **KEEP-as-plugin-gate**. Secrets/injection scans run as validation gates.
*   **`eval` / `review`:** **DEFER commands; keep gate hooks**. Commands are deferred; the validation hooks remain so the pluggable gate interface is verified.
*   **`deploy` / `observe` / `ingest` / `harness`:** **DEFER**. Deployment checks, production error correlation, codebase coverage gates, and package bundle registries are deferred.
*   **`submit` / `migrate`:** **CUT**. The shell execution wrapper `submit` is deleted; database schema migration `migrate` is removed until a new schema change arises.

*Origin:* Triage decisions detailed in [00-scope-triage.md](file:///var/www/html/rai/up/specd/fresh-start/00-scope-triage.md).

---

## 2. Pluggable Security Gate (Gate 12)

While the `security` command surface has been removed, security scanning remains active as an opt-in validation gate during checkouts:

*   **Scanners:** Custom, Go-stdlib-only regex scanners looking for high-risk issues:
    *   Exposed private keys and API tokens.
    *   Dynamic code evaluation or environment leakage.
    *   Potential dependency squatting (slopsquatting).
*   **Allowlist:** Bypassing files (e.g. mock test variables) requires adding an entry to `.specd/security/allow.json`.
*   **Mandatory Reasons:** Any entry in the allowlist lacking a populated `reason` field is treated as a hard error, failing the security gate.

*Origin:* Security scanner logic in [internal/core/security/](file:///var/www/html/rai/up/specd/reference/internal/core/security/).

---

## 3. Two-Seam Re-Entry Contract

When deferred flywheel modules are reintroduced in future releases, they must conform to a strict **Two-Seam Re-Entry Contract** to prevent code bloat:

1.  **Gate Seam:** Features must implement the `Gate` interface (see [03-validation-gates.md](file:///var/www/html/rai/up/specd/docs/03-validation-gates.md)) and run as validation gates. They must **not** add hardcoded conditional branches to the core check logic.
2.  **State Seam:** Persistent records (metrics, approvals, reports) must write to the `state.records` extension map as JSON (see [02-spec-lifecycle-state.md](file:///var/www/html/rai/up/specd/docs/02-spec-lifecycle-state.md)). They must **not** add new fields to the core `state.json` schema.

---

## 4. Preserved Evidence Schemas

To ensure future compatibility, `specd` documents the expected schemas for deferred evidence records:

### DeployApproval
Tracks manual or CI production approvals:
```json
{
  "spec": "feature-x",
  "approvedBy": "Jane Doe",
  "approvedAt": "2026-07-04T12:00:00Z",
  "environment": "production",
  "gitHead": "a1b2c3d4..."
}
```

### ReviewRecord
Stores review feedback:
```json
{
  "reviewer": "agent-auditor",
  "verdict": "approve",
  "file": "docs/deferred-flywheel.md",
  "comments": []
}
```

### EvalRecord
Saves model evaluation benchmarks:
```json
{
  "testSuite": "regression-v1",
  "passedCount": 42,
  "failedCount": 0,
  "latencyMs": 14022
}
```

### IngestRecord
Tracks codebase migration coverage:
```json
{
  "filesMapped": 12,
  "filesUnmapped": 0,
  "waivers": {}
}
```
