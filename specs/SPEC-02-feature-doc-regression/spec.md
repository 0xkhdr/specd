# SPEC-02: Feature ↔ Doc Regression

## Overview
- **Domain:** Feature Completeness & Functional Regression (Analysis Plan Domain 2)
- **Risk Level:** Medium (the product works; the risk is silent doc/behavior drift misleading users)
- **Priority:** P1
- **Dependencies:** SPEC-01 (needs trustworthy CI to verify). Feeds SPEC-05 (coverage targets)
  and SPEC-07 (DX docs).

## Current State

specd exposes **23 verbs** declared once in `internal/core/commands.go` (one is deferred:
`triage`). Handlers live one-per-verb in `internal/cmd/`. The command palette is the declared
single source of truth (`help --json`, `HelpSchemaVersion 1`); `docs/command-reference.md` claims
to be generated from it and `docs/CHEATSHEET.md` is a byte-identical mirror enforced by
`docs-lint.sh`.

Known correctness anchors and gaps:

- **Fail-closed invariant:** unknown verbs and bad flag values exit 2 (documented). Deferred
  verbs print a deferral notice and exit 0 — never a silent no-op.
- **Slug-position correctness:** handlers read the spec slug via `SpecSlugArg` — `brain` uses
  `argAt(1)`, others `argAt(0)`. The argv position must match the actual layout per verb.
- **Gate-count contradiction (confirmed doc bug):** `README.md` line 15 says "12 core gates";
  line 74 says "14 core validation gates". `docs/validation-gates.md` authoritatively lists **14**
  (12 always-on + `criteria` & `review` opt-in). This must be normalized to 14 everywhere.
- **Orphan risk:** `brain_worker.go`, `dispatch.go` etc. dispatch sub-behaviors — some may have
  no doc entry, and docs may describe behavior with no handler.

## Target State

A verified, regression-guarded map: every documented verb ↔ a real handler ↔ a runnable example.
No verb/flag documented that a handler rejects; no handler behavior undocumented. Gate count is a
single consistent number (14). Deferred and fail-closed invariants are asserted by a test.

## Scope Boundaries

- **In Scope:** `internal/core/commands.go`, all `internal/cmd/` handlers, and the doc surface
  `docs/command-reference.md` / `docs/CHEATSHEET.md` / `docs/user-guide.md` / `docs/concepts.md`;
  the README gate-count normalization; a parity test asserting verb↔handler↔doc.
- **Out of Scope:** rewriting docs for style/onboarding (SPEC-07); coverage-number targets
  (SPEC-05); security-scanner behavior (SPEC-04); anything under `reference/`. No new verbs — this
  is a regression/consistency spec, not a feature spec.

## Technical Requirements

1. **Verb→handler→doc map:** enumerate all 23 verbs from `commands.go`; for each, confirm a
   registered handler in `internal/cmd/registry.go` and a doc entry in `command-reference.md`.
   Assert the palette→doc generation is still faithful (extend or rely on `docs-lint.sh`).
2. **Deferred-verb check:** assert `triage` prints a deferral notice and exits 0.
3. **Fail-closed check:** assert an unknown verb exits 2 and a bad flag value exits 2.
4. **Slug-position check:** assert each verb reads the slug from the correct argv index
   (`brain`→`argAt(1)`, others→`argAt(0)`).
5. **Orphan sweep:** flag any handler with no doc entry and any documented behavior with no
   handler, including sub-behaviors dispatched from `brain_worker.go` / `dispatch.go`. Resolve each
   (document it, or record why it is intentionally internal).
6. **Gate-count normalization:** replace every "12 core gates" with "14" across README (lines 15
   and 74) and any other doc; `docs/validation-gates.md` (14) is authoritative. Ideally guard this
   with a lint so it can't drift again (the durable-guard mechanism is designed in SPEC-07; SPEC-02
   just fixes the number and files the guard request).
7. **Lifecycle/role exercise:** drive each phase transition (perceive→…→reflect) and each role
   (scout/craftsman/validator/auditor) end-to-end against the acceptance described in docs.

## Verification Strategy

- A new/extended parity test (near `internal/core/gates/parity_test.go`) asserts verb↔handler
  parity and the deferred/fail-closed/slug-position invariants; runs green under
  `go test ./... -race`.
- `docs-lint.sh` stays green (CHEATSHEET ↔ command-reference byte-identical).
- Grep proves zero remaining "12 core gates" occurrences.
- Every command example in `command-reference.md` runs verbatim against a fresh `specd init`'d
  project (this runnability check is formalized in SPEC-07; SPEC-02 confirms the map is accurate).
- No LLM in gate/DAG/report paths; no bypass flag; `reference/` untouched.

## References
- Analysis Plan: Domain 2; Recommended Spec Breakdown row SPEC-02.
- Related Specs: SPEC-01 (CI), SPEC-05 (coverage of new tests), SPEC-07 (doc runnability +
  drift-guard lint).
- Source Files: `internal/core/commands.go`, `internal/cmd/*` (incl. `registry.go`,
  `brain_worker.go`, `dispatch.go`), `docs/command-reference.md`, `docs/CHEATSHEET.md`,
  `docs/validation-gates.md`, `README.md`.
