# SPEC-07: DX & Doc Accuracy

## Overview
- **Domain:** Code Quality & Maintainability (Analysis Plan Domain 4) + Developer Experience &
  Documentation (Domain 7)
- **Risk Level:** Low (docs are extensive and mostly accurate; lint discipline is mature — drift is
  the issue)
- **Priority:** P2
- **Dependencies:** SPEC-01 (lint/CI must run) and SPEC-02 (consumes the verified feature map).

## Current State

**Code quality (D4):** strong existing gates — `gofmt -l` must be empty, `go vet`,
`golangci-lint v2.1.6` (staticcheck), `go mod tidy` diff check, `test-lint.sh` (no banned suffixes
/ space-separated subtest names / dup helpers), `regress-lint.sh` (verify-table smell audit incl.
"smell A": runtime `.specd/specs/` vs planning `specs/` mixups). The missing-CI-script drift
suggests **dead references accumulate** — confirmed orphan scripts include `scripts/stress-brain.sh`
and `scripts/verify-progress.sh` (no workflow calls them).

**Docs (D7):** 12 doc files (`README`/index, `concepts`, `user-guide`, `command-reference`,
`CHEATSHEET`, `validation-gates`, `agent-integration`, `mcp-guide`, `open-spec-format`,
`github-action`, `troubleshooting`, `contributor-guide`), plus `CLAUDE.md` and per-verb
`Examples[]`. `docs-lint.sh` keeps CHEATSHEET ↔ command-reference byte-identical. Known gaps:
- **No CI step runs the doc examples** — example runnability is untested.
- Go-version claims ("1.22+") contradict the `go 1.26` floor (B4) — release-doc claims are SPEC-03;
  general doc-body claims land here.
- No CHANGELOG / versioning-policy doc; no lightweight CONTRIBUTING quick-start distinct from the
  architecture-heavy `contributor-guide.md`.
- `contributor-guide.md` §3 invariants must match the tooling reconciled by SPEC-01.

## Target State

Every documented command example runs verbatim against a fresh `specd init`'d project (checked in
CI or a documented cadence); a CHANGELOG + versioning policy exist; dead scripts are swept; and the
lint-enforced-single-source pattern is extended to gate counts and version strings so B4/gate-count
drift cannot recur.

## Scope Boundaries

- **In Scope:** doc-example runnability check; CHANGELOG + versioning-policy doc; a CONTRIBUTING
  quick-start; dead-script sweep (`stress-brain.sh`, `verify-progress.sh`, any others);
  extending lint-enforced sync to gate counts + Go-version strings; confirming `contributor-guide.md`
  §3 matches post-SPEC-01 tooling; general doc-body version-claim fixes.
- **Out of Scope:** the verb→handler→doc *map* and the gate-count *number fix* (SPEC-02 — SPEC-07
  adds the *durable guard*); release-doc version claims + release.yml (SPEC-03); coverage docs /
  `TESTING.md` (SPEC-05); anything under `reference/` (dead-script sweep must never touch it).

## Technical Requirements

1. **Runnable-example check:** author a check (script + CI step, or documented cadence) that runs
   each documented command example verbatim against a fresh `specd init`'d project and asserts it
   succeeds. Depends on SPEC-02's verified map for the authoritative example set.
2. **Dead-script sweep:** remove scripts no workflow references (`stress-brain.sh`,
   `verify-progress.sh`, and any others found), or wire them in if intentionally kept — decide per
   script and record it. Never touch `reference/`.
3. **CHANGELOG + versioning policy:** author a CHANGELOG and a short versioning-policy doc (how
   versions are cut, what the Go floor is).
4. **CONTRIBUTING quick-start:** a lightweight onboarding distinct from the architecture-heavy
   `contributor-guide.md`.
5. **Drift-proof single sources:** extend the `docs-lint.sh` pattern so the gate count and the Go
   version string are lint-enforced from one authoritative location — closing the B4/gate-count
   drift class permanently.
6. **Invariant sync:** confirm `contributor-guide.md` §3 matches the tooling SPEC-01 reconciled;
   fix general doc-body "1.22+" claims to the real floor.

## Verification Strategy

- The runnable-example check passes in CI (or a documented cadence exists) — every documented
  example runs green against a fresh `specd init`.
- `grep` finds no dead-script references and no incorrect Go-version claims in doc bodies.
- The new drift-guard lint fails on an intentionally mismatched gate count / version string
  (prove with a temporary edit), then passes when consistent.
- `docs-lint.sh`, `test-lint.sh`, `regress-lint.sh`, `gofmt -l`, `go vet`, `go mod tidy` all green.
- CHANGELOG, versioning policy, and CONTRIBUTING exist and link from the docs index.
- No LLM in gate/DAG/report paths; no bypass flag; `reference/` untouched.

## References
- Analysis Plan: Domains 4 & 7; Cross-Cutting Concern 2 (lint-enforced single source);
  Recommended Spec Breakdown row SPEC-07.
- Related Specs: SPEC-01 (tooling reconciliation), SPEC-02 (feature map + gate-count number fix),
  SPEC-03 (release-doc version claims), SPEC-05 (TESTING.md / coverage docs).
- Source Files: `README.md`, `docs/*.md` (12 files), `CLAUDE.md`, per-verb `Examples[]`,
  `scripts/docs-lint.sh` / `test-lint.sh` / `regress-lint.sh`, orphan scripts
  `scripts/stress-brain.sh` / `scripts/verify-progress.sh`.
