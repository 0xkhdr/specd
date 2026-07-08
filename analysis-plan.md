# Analysis Plan: `specd` Production Readiness

> **Phase 1 deliverable.** Discovery & analysis only — no implementation. This document is
> self-contained: a fresh context can generate every `spec.md` / `tasks.md` from it without
> re-reading the codebase. Verified facts are marked ✅ (checked against the tree this session);
> inferences are marked ⚠️.

---

## Executive Summary

`specd` is a Go (stdlib-only, zero runtime deps) spec-driven coding-harness CLI. Module
`github.com/0xkhdr/specd`. ~9,324 non-test LOC + ~5,819 test LOC across `internal/` (test:code
ratio ≈ 0.62). ✅ It **builds clean** (`go build -o specd .` → exit 0) and the **full test
suite passes locally** (`go test ./...` → all `ok`). ✅ The core product — the 23-verb command
palette, 14 validation gates, DAG/frontier executor, evidence-gated completion, MCP server, and
Brain/Pinky orchestration — is implemented and internally consistent.

**The production risk is not in the product code; it is in the release scaffolding.**

### Critical blockers (found during audit)

| # | Blocker | Evidence | Impact |
|---|---------|----------|--------|
| **B1** | **CI invokes a `Makefile` that does not exist.** `.github/workflows/ci.yml:109` runs `make perf-gate`; there is **no Makefile anywhere in the repo** (✅ `find . -name Makefile` outside `reference/` → empty; CLAUDE.md itself states "no root Makefile"). | ci.yml:109 | The `test` job **fails on every push/PR**. |
| **B2** | **CI references 6 stress scripts + `coverage-check.sh` that do not exist.** ci.yml calls `scripts/coverage-check.sh`, `scripts/stress.sh`, `scripts/stress-acp.sh`, `scripts/stress-orchestration.sh`, `scripts/stress-program.sh`, `scripts/stress-brain-recovery.sh`, `scripts/stress-checkpoint-fault.sh`. ✅ Present scripts: `docs-lint.sh`, `regress-all.sh`, `regress-domains.sh`, `regress-lint.sh`, `stress-brain.sh`, `test-lint.sh`, `verify-progress.sh`. None of the 7 CI-referenced files exist; `stress-brain.sh` (which does exist) is **never referenced** by CI. | ci.yml:130,144,157,171,185,199,213 | `coverage-floor` + all **5 stress jobs fail**. 6 of 9 CI jobs are red. |
| **B3** | **CI test matrix pins a Go version the module rejects.** ✅ `go.mod` is 3 lines: `go 1.26` — no `toolchain` line, minimum = **1.26**. CI matrix is `go: ["1.22", "stable"]` (ci.yml:90). Go 1.22 cannot build a `go 1.26` module. | go.mod:3, ci.yml:90 | The `go 1.22` matrix leg fails to compile. |
| **B4** | **Documentation misrepresents the build contract.** CLAUDE.md claims "Requires Go 1.22+ (declared min in `go.mod`; toolchain line pins 1.26)" and README says "Go (1.22+)". ✅ Reality: `go.mod` declares **1.26 as the floor** and has **no toolchain line**. | go.mod, CLAUDE.md, README.md:44 | Contributors on Go 1.22–1.25 cannot build; onboarding docs are wrong. |

**B1–B4 are a single root cause: the CI workflow and the on-disk build tooling have drifted
apart.** No spec that depends on "green CI" is meaningful until this is reconciled. Resolve the
CI/tooling domain (SPEC-01) **first**; it gates verification of every other spec.

Everything else is polish: doc/count drift, an unpinned `govulncheck@latest`, and coverage/
observability formalization. None block the binary from working today.

### Recommended wave order

1. **Wave 0 — SPEC-01 (CI/CD & build tooling reconciliation).** Unblocks verification of all
   later specs. P0. Nothing else can be trusted "green" until this lands.
2. **Wave 1 — SPEC-02 (feature↔doc regression) + SPEC-03 (packaging/release) + SPEC-04
   (security-tooling hardening).** Independent; parallelizable once CI is trustworthy.
3. **Wave 2 — SPEC-05 (test-coverage formalization) + SPEC-06 (observability) + SPEC-07 (DX/doc
   accuracy).** Depend on a stable CI baseline and on SPEC-02's feature map.

---

## Audit Methodology (see also Appendix)

- Read: `CLAUDE.md`, `README.md`, all 12 files under `docs/`, `internal/core/commands.go`
  (the command palette source of truth), `internal/core/gates/registry.go` context via
  `docs/validation-gates.md`, `.github/workflows/ci.yml`, `go.mod`.
- Enumerated the tree (`find`), counted LOC, listed `scripts/`, listed `internal/` package
  layout, and cross-checked every CI step against files actually present.
- Verified live: `go build` (✅ exit 0), `go test ./...` (✅ all pass), Makefile absence,
  script presence, `go.mod` contents, composite action presence.
- **Excluded from all scope:** `reference/` — the frozen v1 museum. Per CLAUDE.md it is a
  read-only artifact: never import, build, edit, or spec against it. No production spec touches it.
- **Note on `specs/`:** the top-level planning `specs/` directory described in the Phase-2 brief
  **does not exist yet** — Phase 2 will create it. Runtime specs live under `.specd/specs/`
  (different path); do not conflate them.

---

## Domain Analysis

### Domain 1: CI/CD Pipeline & Build Tooling — **the blocker domain**

- **Scope:** `.github/workflows/ci.yml`, `.github/workflows/release.yml`, `.github/actions/specd-pr/action.yml`, `scripts/*.sh`, `go.mod`, and any (missing) `Makefile`.
- **Regression Needs:**
  - Every step in `ci.yml` must invoke a target/script that exists. Today: `make perf-gate`
    (B1), `coverage-check.sh` + 6 stress scripts (B2) are missing.
  - The Go version matrix must be buildable against `go.mod`'s floor (B3).
  - Decide the source of truth for `perf-gate`: either add a root `Makefile` with a `perf-gate`
    target, or rewrite the CI step to call a script directly. CLAUDE.md asserts "no root
    Makefile" as a design choice — so the lazy correct fix is to **replace `make perf-gate` with
    a `scripts/perf-gate.sh`**, not to introduce a Makefile that contradicts the docs.
  - Reconcile the 5 stress jobs: either author the 6 missing stress scripts (their intent is
    documented in ci.yml job names — cross-process contention, ACP ledger, orchestration,
    program scheduler, brain recovery, checkpoint fault injection) **or** collapse them onto the
    one real script (`stress-brain.sh`) and delete the dead jobs. Subtractive bias (CLAUDE.md
    guardrail) favors deleting jobs whose scripts were never written.
  - `coverage-check.sh` must exist for the `coverage-floor` job; define the floor it enforces.
- **Code Quality Checks:** Pin all `go install ...@latest` (✅ `govulncheck@latest` at ci.yml:80
  is unpinned — reproducibility + supply-chain risk); confirm all `uses:` actions are pinned
  (they are — checkout@v5, setup-go@v6, golangci-lint@v6, shellcheck@2.0.0). Verify
  `release.yml` still builds/signs correctly.
- **Doc Gaps:** CLAUDE.md and README both misstate the Go floor and the toolchain line (B4).
  The "CI runs X" claims in CLAUDE.md must match the reconciled workflow.
- **Risk Level:** **HIGH** (production release is impossible with red CI).
- **Dependencies:** Blocks SPEC-02..07 (they need trustworthy CI to verify). Depends on nothing.
- **Estimated Spec Complexity:** **Moderate** (mechanical reconciliation + a few new scripts;
  the hard part is the deliberate keep-vs-delete decision per stress job).

### Domain 2: Feature Completeness & Functional Regression

- **Scope:** `internal/core/commands.go` (23 verbs incl. 1 deferred), all handlers in
  `internal/cmd/`, and the doc surface in `docs/command-reference.md` / `docs/CHEATSHEET.md` /
  `docs/user-guide.md` / `docs/concepts.md`.
- **Regression Needs:**
  - Map every documented verb → handler → example. ✅ Palette is the declared single source of
    truth (`help --json`, `HelpSchemaVersion 1`); `command-reference.md` claims to be generated
    from it and `CHEATSHEET.md` is a byte-identical mirror (enforced by `docs-lint.sh`).
    Regression = assert the doc↔palette generation is still faithful and no verb/flag is
    documented that the handler doesn't accept (or vice-versa).
  - Confirm the one **deferred** verb (`triage`) still prints a deferral notice and exits 0
    (never silently no-ops) — this is an explicit invariant.
  - Exercise each lifecycle phase transition (perceive→…→reflect) and each role
    (scout/craftsman/validator/auditor) end-to-end against acceptance in docs.
  - Orphan check: any handler in `internal/cmd/` with **no** doc entry, or any documented
    behavior with no handler. `brain_worker.go`, `dispatch.go` etc. exist — confirm every
    dispatched sub-behavior is documented.
- **Code Quality Checks:** Fail-closed on unknown verbs / bad flag values (✅ documented exit 2);
  phase enforcement via `SpecSlugArg` index correctness (e.g. `brain` uses `argAt(1)`, others
  `argAt(0)` — verify the slug position matches actual argv layout).
- **Doc Gaps:** ✅ **Gate-count contradiction inside README**: line 15 says "12 core gates",
  line 74 says "14 core validation gates"; `docs/validation-gates.md` authoritatively lists
  **14** (12 always-on + `criteria` & `review` opt-in). Normalize to 14 everywhere.
- **Risk Level:** **MEDIUM** (product works; risk is silent doc/behavior drift misleading users).
- **Dependencies:** Feeds SPEC-05 (coverage) and SPEC-07 (DX docs). Depends on SPEC-01 for CI.
- **Estimated Spec Complexity:** **Moderate** (breadth: 23 verbs × phases × roles).

### Domain 3: Performance & Scalability

- **Scope:** `internal/core/dag.go`, `frontier.go`, `phases.go`, `context/` (bounded manifest),
  `orchestration/`, and the `perf-gate` CI step.
- **Regression Needs:** ✅ CI comment (ci.yml:107) declares a "measured perf gate — disabled-mode
  context manifest does no work" (A4 invariant). That gate is currently unrunnable (B1). The
  spec must restore a runnable perf assertion: disabled-mode context build is O(0) work; DAG
  frontier computation scales with task count without quadratic blowup.
- **Code Quality Checks:** DAG build is acyclic-checked once; frontier recompute cost per wave;
  no N+1 file reads in context manifest assembly; deterministic resource cleanup (locks released,
  temp files removed on verify failure / `--revert-on-fail`).
- **Doc Gaps:** No published benchmark numbers or scale envelope (max tasks/spec, max specs/
  program). Document the intended scale.
- **Risk Level:** **LOW** (local scale is small; single-binary, local FS). Elevated to the extent
  the perf-gate is part of the broken CI (that portion rolls into SPEC-01).
- **Dependencies:** perf-gate restoration overlaps SPEC-01.
- **Estimated Spec Complexity:** **Simple** (a benchmark + one CI assertion + a scale-envelope doc).

### Domain 4: Code Quality & Maintainability

- **Scope:** all of `internal/`, `scripts/test-lint.sh`, `scripts/regress-lint.sh`, gofmt/vet/
  golangci-lint config.
- **Regression Needs:** ✅ Strong existing gates — `gofmt -l` must be empty, `go vet`,
  `golangci-lint v2.1.6` (staticcheck), `go mod tidy` diff check, `test-lint.sh` (no banned
  suffixes / space-separated subtest names / dup helpers), `regress-lint.sh` (verify-table smell
  audit incl. "smell A": runtime `.specd/specs/` vs planning `specs/` mixups). Regression =
  these all still pass and cover new code.
- **Code Quality Checks:** Dead-code sweep (the missing-CI-script drift suggests dead references
  accumulate — audit for dead scripts like `verify-progress.sh` / `stress-brain.sh` that no
  workflow calls); consistent naming; zero runtime deps preserved (`go.mod`/`go.sum` tidy).
- **Doc Gaps:** `docs/contributor-guide.md` §3 documents invariants — confirm it matches the
  reconciled tooling after SPEC-01.
- **Risk Level:** **LOW** (mature lint discipline already in place).
- **Dependencies:** Baseline for all specs. Depends on SPEC-01 (lint runs in CI).
- **Estimated Spec Complexity:** **Simple**.

### Domain 5: Security Hardening

- **Scope:** `internal/core/gates/security/` (scanners: `secrets`, `injection`, `slopsquat` +
  testdata fixtures), verify sandboxing (`--sandbox` / bwrap), `--revert-on-fail`, slug
  validation (recent commit `df76d4c` added it), evidence-integrity gate, `govulncheck` in CI.
- **Regression Needs:** ✅ Opt-in security gate (`check --security`) scans git-tracked files with
  `off|warn|error` severity, allowlist-by-fingerprint that **fails closed** on load error, and a
  documented scan boundary (excludes lockfiles, `testdata/`, `.specd/`, `reference/`, `vendor/`,
  `.git/`). Regression = each scanner still fires on its fixture and respects the boundary;
  fail-closed paths hold.
- **Code Quality Checks:** Input validation at trust boundaries — slug validation (verify it
  covers path traversal into `.specd/specs/<slug>/`), verify-command execution (command
  injection surface: verify lines are shell-executed — confirm sandbox/isolation story),
  no secrets in logs, least-privilege FS access. **Pin `govulncheck@latest`** (ci.yml:80) — an
  unpinned security scanner is itself a supply-chain hole. ✅ `slopsquat` scanner is the
  dependency-typosquat defense; confirm it and `govulncheck` both run.
- **Doc Gaps:** No consolidated threat model / SECURITY.md (no vuln-disclosure policy found).
- **Risk Level:** **MEDIUM** (verify lines are arbitrary shell by design — the sandbox story
  must be airtight and documented; the evidence-integrity gate is the trust anchor).
- **Dependencies:** `govulncheck` pin overlaps SPEC-01. Otherwise independent.
- **Estimated Spec Complexity:** **Moderate**.

### Domain 6: Test Coverage & Reliability

- **Scope:** all `*_test.go` (~5,819 LOC), `internal/core/gates/parity_test.go` (handler/verb
  parity), `conformance_test.go`, `integration_polish_test.go`, `scripts/coverage-check.sh`
  (missing), `scripts/regress-*.sh`, the `-count=2` order-dependence job.
- **Regression Needs:** ✅ `go test ./... -race` and `-count=2` (flaky/iteration-order catch)
  pass locally. The `coverage-floor` job is **broken** (B2: `coverage-check.sh` missing) — the
  floor is unenforced today. Spec must author `coverage-check.sh` with an explicit floor and wire
  the `regress-*.sh` harnesses (which re-run every task verify + wave invariant) into CI or a
  documented cadence.
- **Code Quality Checks:** Per-package coverage gaps (measure once `coverage.out` is produced);
  agent-facing MCP contract tests (`internal/mcp/`) and help-palette schema tests; flaky-test
  identification via the `-count=2` leg.
- **Doc Gaps:** No stated coverage target; `TESTING.md` is referenced by ci.yml:232 — confirm it
  exists and is accurate (⚠️ not verified this session).
- **Risk Level:** **MEDIUM** (tests are strong but the coverage floor is currently a no-op).
- **Dependencies:** Depends on SPEC-01 (coverage job must run) and SPEC-02 (feature map to target).
- **Estimated Spec Complexity:** **Moderate**.

### Domain 7: Developer Experience & Documentation

- **Scope:** `README.md`, all `docs/*.md` (12 files: README/index, concepts, user-guide,
  command-reference, CHEATSHEET, validation-gates, agent-integration, mcp-guide,
  open-spec-format, github-action, troubleshooting, contributor-guide), `CLAUDE.md`, examples in
  each verb's `Examples[]`.
- **Regression Needs:** Every documented command example must run verbatim against a fresh
  `specd init`'d project. Setup instructions (`go build -o specd .`) ✅ work. Fix the Go-version
  claims (B4). Fix gate-count drift (Domain 2). Confirm `docs/README.md` index links resolve.
- **Code Quality Checks:** `docs-lint.sh` keeps CHEATSHEET↔command-reference byte-identical (✅
  enforced); extend the same discipline to gate counts and version strings so they can't drift.
- **Doc Gaps:** No CHANGELOG / versioning-policy doc found; no CONTRIBUTING quick-start distinct
  from the architecture-heavy contributor-guide; example-runnability is untested (no CI step
  executes the doc examples).
- **Risk Level:** **LOW** (docs are extensive and mostly accurate; drift is the issue).
- **Dependencies:** Consumes SPEC-02's verified feature map. Depends on SPEC-01.
- **Estimated Spec Complexity:** **Simple**.

### Domain 8: Observability & Debugging

- **Scope:** `specd report` (`--pr|--metrics|--json|--history|--format prometheus`),
  `specd status --program`, `handshake` digests, `context --hud` operator HUD, orchestration
  ACP ledger (`internal/orchestration/acp.go`), lock/CAS error surfacing.
- **Regression Needs:** ✅ Reporting is deterministic (generated from `state.json` + task
  artifacts, no LLM). Regression = `report --format prometheus` emits valid textfile-collector
  metrics; `--history` replays the audit trail in timestamp order; CAS/lock errors are
  actionable (documented in `troubleshooting.md`).
- **Code Quality Checks:** Consistent error messages (exit 1 vs 2 discipline); no silent
  failures; the ACP ledger append/replay is crash-safe (the missing `stress-acp.sh` /
  `stress-checkpoint-fault.sh` jobs were meant to prove this — see B2/SPEC-01).
- **Doc Gaps:** No documented logging-levels or telemetry strategy for the CLI itself
  (worker-reported `--tokens`/`--cost`/`--duration-ms` are stored verbatim — document where
  they surface in reports).
- **Risk Level:** **LOW** (deterministic reporting is a design strength).
- **Dependencies:** Crash-safety stress jobs overlap SPEC-01. Depends on SPEC-01.
- **Estimated Spec Complexity:** **Simple**.

---

## Cross-Cutting Concerns

1. **CI/tooling drift is the master defect.** B1–B4 all stem from `ci.yml` referencing tooling
   that was never committed (or was deleted) plus a stale version pin. It touches Domains 1, 3,
   5, 6, 8. **Fix once, in SPEC-01**, rather than per-domain — one reconciliation is a smaller,
   safer diff than seven partial patches (root-cause over symptom).
2. **Two sources of truth kept in sync by lint.** The palette↔docs (`docs-lint.sh`) and
   `tasks.md`↔`state.json` (`sync` gate) sync mechanisms are good models. Extend the same
   "lint-enforced single source" pattern to gate counts and the Go version string so B4-class
   drift can't recur.
3. **`.specd/specs/` (runtime) vs `specs/` (planning) confusion.** `regress-lint.sh` smell "A"
   already guards verify lines; every new spec's `verify:` lines must target the runtime path,
   not this repo's planning artifacts. Phase 2 authors must not create ambiguity here.
4. **Supply chain.** Zero runtime deps ✅ (strong). The only external fetch is
   `govulncheck@latest` in CI — pin it. Confirm `release.yml` artifact integrity/signing.
5. **`reference/` isolation.** Every spec must explicitly exclude `reference/`. No spec builds,
   imports, edits, or specs against the frozen museum.
6. **Determinism / no-LLM invariant.** No spec may introduce an LLM into any gate, DAG, or
   report path, or add an evidence-bypass flag. State this as a non-negotiable in every spec's
   acceptance criteria.

---

## Recommended Spec Breakdown

| Spec ID | Domain | Priority | Complexity | Dependencies | Core deliverable |
|---------|--------|----------|------------|--------------|------------------|
| **SPEC-01** | CI/CD & Build Tooling (D1) | **P0** | Moderate | None | Green CI: resolve `make perf-gate` (→ `scripts/perf-gate.sh`, no Makefile), author or delete the 6 missing stress jobs + `coverage-check.sh`, fix the Go-version matrix vs `go 1.26` floor, pin `govulncheck`. |
| **SPEC-02** | Feature ↔ Doc Regression (D2) | P1 | Moderate | SPEC-01 | Verified verb→handler→doc map for all 23 verbs; deferred-verb + fail-closed checks; kill gate-count drift. |
| **SPEC-03** | Packaging & Release Readiness (D3 perf + release) | P1 | Simple | SPEC-01 | Restore perf-gate assertion; validate `release.yml` build/sign; document scale envelope; correct Go-version docs (B4). |
| **SPEC-04** | Security Tooling Hardening (D5) | P1 | Moderate | SPEC-01 | Pin `govulncheck`; verify sandbox/slug-validation trust boundaries; add SECURITY.md/threat model; regression-test each scanner + fail-closed allowlist. |
| **SPEC-05** | Test Coverage Formalization (D6) | P2 | Moderate | SPEC-01, SPEC-02 | `coverage-check.sh` with explicit floor; wire `regress-*.sh` into CI/cadence; close per-package gaps; confirm `TESTING.md`. |
| **SPEC-06** | Observability & Crash-Safety (D8) | P2 | Simple | SPEC-01 | Validate prometheus/`--history`/HUD outputs; document logging & worker-metric surfacing; restore ACP/checkpoint crash-safety stress coverage. |
| **SPEC-07** | DX & Doc Accuracy (D4 + D7) | P2 | Simple | SPEC-01, SPEC-02 | Runnable-example check; CHANGELOG + versioning policy; dead-script sweep; extend lint-enforced sync to counts/versions. |

> **Wave mapping:** SPEC-01 = Wave 0 (blocking). SPEC-02/03/04 = Wave 1 (parallel).
> SPEC-05/06/07 = Wave 2. Priorities: P0 blocks release; P1 required for a credible 1.0;
> P2 hardens and prevents regression recurrence.

---

## Appendix: Audit Methodology

- **How files were reviewed:** Full read of `CLAUDE.md`, `README.md`, `docs/command-reference.md`,
  `docs/validation-gates.md`, `internal/core/commands.go`, `.github/workflows/ci.yml`, `go.mod`;
  structural enumeration (tree, LOC, package layout, script inventory) of the rest. `reference/`
  was excluded by policy.
- **Tools/commands used:** `find` (tree + Makefile/script/action presence), `wc -l` (LOC),
  `go build -o … .` (✅ exit 0), `go test ./... -count=1` (✅ all packages `ok`), direct reads of
  the palette and CI definitions, and one-by-one existence checks of every path `ci.yml` invokes.
- **Assumptions made:**
  - `ci.yml` is the intended CI contract; the missing scripts/Makefile represent drift to be
    reconciled, not an intentional teardown. ⚠️ If the intent was to *delete* those jobs, SPEC-01
    flips from "author scripts" to "delete jobs" — either way SPEC-01 owns the decision.
  - `TESTING.md` (referenced by ci.yml:232) exists and is accurate — ⚠️ not verified this session;
    SPEC-05 confirms.
  - The 14-gate list in `docs/validation-gates.md` is authoritative over README's "12".
  - Local `go test` passing implies the source is healthy; the failures are confined to release
    scaffolding (CI infra), which cannot run locally in one command.
- **Not audited (flagged for spec authors):** exact per-package coverage numbers, `release.yml`
  internals, `.github/actions/specd-pr/action.yml` internals (presence ✅ confirmed), and the
  bodies of the security scanners (behavior taken from `docs/validation-gates.md`).
