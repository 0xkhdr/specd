# SPEC-04: Security Tooling Hardening

## Overview
- **Domain:** Security Hardening (Analysis Plan Domain 5)
- **Risk Level:** Medium (verify lines are arbitrary shell by design — the sandbox story must be
  airtight and documented; the evidence-integrity gate is the trust anchor)
- **Priority:** P1
- **Dependencies:** SPEC-01 (the `govulncheck` pin overlaps SPEC-01). Otherwise independent.

## Current State

- **Opt-in security gate:** `check --security` scans git-tracked files with three scanners under
  `internal/core/gates/security/`: `secrets`, `injection`, `slopsquat` (dependency-typosquat
  defense), each with testdata fixtures. Severity is `off|warn|error`. An allowlist keyed by
  fingerprint **fails closed on load error**. The scan boundary excludes lockfiles, `testdata/`,
  `.specd/`, `reference/`, `vendor/`, `.git/`.
- **Trust boundaries:** slug validation was recently added (commit `df76d4c`) — it must cover path
  traversal into `.specd/specs/<slug>/`. Verify lines are shell-executed (command-injection
  surface by design); the sandbox story is `--sandbox` / bwrap plus `--revert-on-fail`.
- **Evidence integrity:** a task completes only against a passing verify record pinned to a real
  git HEAD — the trust anchor.
- **Supply chain:** zero runtime deps (strong). The only external fetch, `govulncheck`, is
  **pinned** to `@v1.5.0` (`ci.yml:86`, applied by SPEC-01 T-01-06 in commit `a5e3935`) — the
  former `@latest` supply-chain hole is closed. Both supply-chain checks run: `govulncheck` in CI
  and the `slopsquat` typosquat scanner in the security gate.
- **Doc gap:** no consolidated threat model / `SECURITY.md`; no vulnerability-disclosure policy.

## Target State

Every scanner is regression-tested against its fixture and proven to respect the scan boundary;
fail-closed paths are proven to hold; trust boundaries (slug validation, verify-command execution)
are validated and documented; `govulncheck` is pinned; a `SECURITY.md` with a threat model and
disclosure policy exists.

## Scope Boundaries

- **In Scope:** `internal/core/gates/security/` scanners + fixtures; slug validation coverage;
  verify sandboxing (`--sandbox`/bwrap) and `--revert-on-fail` isolation story; the `govulncheck`
  pin (coordinated with SPEC-01); `SECURITY.md` / threat model; supply-chain confirmation
  (govulncheck + slopsquat both run).
- **Out of Scope:** the broader CI reconciliation (SPEC-01); release artifact signing internals
  (SPEC-03 owns release.yml, though supply-chain intent is shared); performance of scanners;
  anything under `reference/` (it is explicitly excluded from scanning and from edits).

## Technical Requirements

1. **Per-scanner regression:** each of `secrets`, `injection`, `slopsquat` must fire on its
   fixture and **not** fire outside the documented boundary. Add/confirm tests asserting both.
2. **Fail-closed allowlist:** prove that a corrupt/unloadable allowlist causes the gate to fail
   closed (not silently pass).
3. **Slug validation:** confirm slug validation rejects path-traversal (`../`, absolute paths,
   separators) so it cannot escape `.specd/specs/<slug>/`. Add a test if missing.
4. **Verify-command isolation:** document and test the sandbox story — `--sandbox`/bwrap isolation
   for shell-executed verify lines, and that `--revert-on-fail` restores state and leaks no temp
   files. Confirm no secrets are written to logs.
5. **Pin `govulncheck`:** DONE — pinned to `@v1.5.0` at `ci.yml:86` (SPEC-04 chose the version;
   SPEC-01 applied it via T-01-06, commit `a5e3935`). Both `govulncheck` and the `slopsquat`
   scanner confirmed running.
6. **`SECURITY.md`:** author a threat model (attacker model: hostile spec/tasks content, hostile
   verify lines, hostile dependency names) and a vulnerability-disclosure policy.

## Verification Strategy

- `go test ./internal/core/gates/security/... -race` green, including boundary and fail-closed
  cases.
- A traversal-attempt test proves slug validation rejects escapes.
- A sandbox test proves verify lines run isolated and `--revert-on-fail` leaves a clean tree.
- ci.yml shows a pinned `govulncheck` version; the govulncheck job passes; `slopsquat` runs.
- `SECURITY.md` exists with a threat model and disclosure contact/policy.
- No LLM in gate/DAG/report paths; **no evidence-bypass flag added** (explicitly re-verified —
  the evidence gate is the security trust anchor); `reference/` untouched.

## References
- Analysis Plan: Domain 5; Cross-Cutting Concerns 4 (supply chain) & 6 (determinism);
  Recommended Spec Breakdown row SPEC-04.
- Related Specs: SPEC-01 (applies govulncheck pin), SPEC-03 (release signing / supply chain).
- Source Files: `internal/core/gates/security/` (secrets/injection/slopsquat + testdata),
  slug-validation code (commit `df76d4c`), verify sandbox (`--sandbox`/bwrap, `--revert-on-fail`),
  evidence-integrity gate, `.github/workflows/ci.yml`.
