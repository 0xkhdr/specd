# specd Production-Grade Optimization — Progress

## Status

| Spec | Status | Notes |
|------|--------|-------|
| S1 security-hardening | Spec drafted | Scope narrowed per `discrepancies.md` D1–D4, D12 |
| S2 performance-optimization | Implemented (T1-T5) | Incremental `FrontierDetector` cache in `frontier.go`; 24-38% faster, 38-46% fewer bytes/op at 20/100/500 tasks, zero regressions; see `tasks.md` evidence |
| S3 code-quality-readability | Implemented (T1-T8) | gocyclo/revive config, package/exported docs, named hotspot refactors; gates pass |
| S4 testing-reliability | Spec drafted | F3 (stress resource bounds) confirmed, D15 |
| S5 observability | Implemented (T1-T7) | Stdlib-only duration metrics, opt-in Prometheus endpoint, build-tag-gated Chrome trace spans; `make test` + `make perf-gate` pass |
| S6 cicd-build-hardening | Spec drafted | `-trimpath` gap real; signing flagged as decision gate, D13 |
| S7 documentation-hygiene | Spec drafted | docs-lint.sh extended not rewritten, D8; D14, D17 |

Validation pass (4 parallel research agents covering security, performance/
observability, code-quality/testing, CI-CD/docs) completed against the live
repository before any spec was drafted, per the action prompt's Validation
Requirement. Full discrepancy log: [`discrepancies.md`](discrepancies.md).

## Implementation Vision (supersedes analysis-plan §6 where evidence disagreed)

**Design direction unchanged:** production-grade hardening with measurable
improvements, preserving the existing layered architecture and the five
documented invariants (INV1–INV5 in `docs/contributor-guide.md`). The live
evidence shows the codebase is materially more mature than the analysis plan
assumed in several areas (checksum-verified installs, fail-closed sandboxing,
complete path-traversal guards, existing structured logging) and the real gaps
are narrower and more specific than the plan's broad-strokes recommendations.

**What changed from the original plan:**

1. **Security scope shrank.** Three of the five original security findings
   (path traversal, checksum verification, sandbox fail-closed behavior) are
   already correctly implemented. S1 now focuses on the one confirmed gap —
   MCP argument-shape validation at the transport boundary — plus regression
   tests that pin the *existing* correct behavior so it can't silently regress
   (slug validation, fail-closed sandbox, checksum-required install).
2. **Performance work is retargeted, not removed.** The DAG/frontier code
   lives in `internal/core/dag.go`/`frontier.go`, not `internal/worker/` as
   the plan assumed. The real optimization is incremental frontier
   maintenance (avoid the full O(V+E) rescan on every task-completion event)
   — confirmed as a genuine O(V·(V+E)) cost over a full orchestration run.
   Spec-parsing "optimization" is dropped: `internal/core/tasksparser.go`
   shows no allocation hotspot at the scale specs actually reach.
3. **Observability scope shrank to metrics + tracing.** Structured `log/slog`
   logging already exists in `internal/obs/log.go`. No further logging work
   is needed; S5 is entirely about adding the missing metrics counters/timers
   and optional compile-time tracing hooks.
4. **Code quality work is now named, not generic.** Five concrete
   high-branch-density functions are the refactor targets (see
   `discrepancies.md` D10), plus a missing complexity linter and six packages
   missing package-level doc comments (D9, D11).
5. **CI/CD scope is precise.** SBOM generation already exists (CycloneDX via
   syft). The real gaps are `-trimpath` (straightforward to add) and artifact
   signing (explicitly deferred upstream pending key-management — flagged as
   a decision gate for the user, not silently implemented).
6. **Testing/reliability work is confirmed as-is.** Stress targets genuinely
   lack `ulimit`/timeout bounds (F3 stands). Coverage-floor enforcement is a
   more sophisticated ratchet system than the plan assumed (11 packages,
   floors from 70%–95%, regression-only) — S4 raises specific package floors
   where evidence supports it rather than a blanket "raise the floor."
7. **Documentation scope corrected.** `scripts/docs-lint.sh` already exists
   (extend, don't recreate). The Windows-support doc claim was overbroad
   (only Brain/Pinky orchestration is POSIX-only, not general usage). A
   genuinely out-of-scope content block (unrelated third-party tool
   instructions in `AGENTS.md:252-293`) was found and flagged as a decision
   gate rather than silently deleted.

**Affected modules (revised):**
- `internal/mcp/server.go`, `internal/mcp/transport.go` — argument-shape
  validation gate.
- `internal/core/dag.go`, `internal/core/frontier.go` — incremental frontier
  maintenance.
- `internal/cmd/pinky.go`, `internal/cmd/init.go`,
  `internal/core/orchestration_driver.go`, `internal/cmd/doctor.go`,
  `internal/core/acp.go` — complexity refactor targets.
- `internal/core`, `internal/cmd`, `internal/cli`, `internal/runner`,
  `internal/pack`, `internal/schema` — missing package doc comments.
- `internal/obs/` — new metrics module (logging already complete).
- `scripts/stress*.sh`, `Makefile` — resource bounds on stress targets.
- `.goreleaser.yml` — `-trimpath`.
- `scripts/docs-lint.sh` — extend dead-list source past the hardcoded
  20-command table.
- `AGENTS.md`, `README.md`, `SECURITY.md` — corrections per D14, D17.

**Interfaces/contracts preserved:** CLI surface, exit codes, `config.json`
schema, spec directory structure, `state.json` schema, MCP tool contract — no
spec in this set changes any of these.

**Rollout (gates from the action prompt, unchanged):**
- G1 — Security review (S1) complete before performance changes (S2).
- G2 — Performance benchmark baseline recorded before observability (S5).
- G3 — All tests pass with `-race` (S4) before observability integration.
- G4 — No performance regression >5% before documentation updates (S7).

**Rollback:** every spec is source-level only; no state-file or schema
migrations are introduced. Rollback is `git revert` per spec/commit.

---

# Rename Base Mode and Roles — Status

**Spec:** `specs/rename-base-mode-and-roles/spec.md` / `tasks.md`
**Overall status:** Complete. All 7 waves implemented and verified.
**Current wave:** none (done)

**Requirement-to-spec coverage:**

| Requirement | Spec section | Status |
|---|---|---|
| R1 mode rename in Go source | Wave 1 | Done |
| R2 mode rename in templates | Wave 2 (scope corrected — near-empty) | Done |
| R3 mode/role docs | Wave 2, Wave 5 | Done |
| R4 `.claude/agents/` | Wave 4 | Done |
| R5 CLI metadata/enums | Wave 1, Wave 3 (schema) | Done |
| R6 role rename | Wave 3, Wave 4 | Done |
| R7 dispatch/context logic | Wave 3 | Done |
| R8 tests/fixtures | Wave 6 | Done |
| R9 `state.json` mode value | Wave 1 — **deviates from spec.md's D2 recommendation**, see below | Done |
| R10 validation gates/schema | Wave 3 | Done |
| R11-R17 (discovered during verification: partial scout rename, scaffold.go, fusion.go, phase-default pickers, orchestration_authoring.go, spec_builder.go, schema v1.json 3 enums, skill/steering/stub templates) | Wave 3, Wave 4 | Done |

**Decision gates:**
- G1 ✅ — User selected role names: `scout`, `craftsman`, `auditor`,
  `validator` (`brain`/`pinky` unchanged).
- G2 ✅ — User resolved D2-D5 explicitly **against** spec.md's recommendation:
  "I don't need to migrate from old versions as we still develop in first
  release." Implementation therefore did a **clean rename with zero legacy
  aliases**, not the permanent-alias design spec.md proposed:
  - `ModeBase`/`"base"` removed outright (renamed to `ModeSimple`/`"simple"`),
    no dual-resolution branch for old on-disk values.
  - The pre-existing `investigator` legacy alias in the role registry (from
    an earlier, unrelated rename) was also removed for consistency — the
    registry now has exactly 8 roles, all canonical, no aliases.
  - `internal/core/fusion.go`'s roles health check requires the new names
    only (no OR-logic accepting old paths) — verified by hand-constructing
    an old-style `.specd/roles/` directory and confirming `specd doctor`
    correctly reports it unhealthy.
  - MCP `prompts.go` exposes only the 8 canonical `role/*` resources (no
    `role/builder`/`role/investigator`/etc duplicates).
  - Schema v1.json's `executionMode` and all three `role` enums list only
    the new names.

**Baselines/targets:** `make test` (race detector) passes; `make ci` lint +
race `-count=2` + stress all pass. `cover-check`'s `internal/worker` floor
failure (87.4% vs 88%) is pre-existing on `HEAD` (confirmed via `git stash`
before any of this spec's edits) and unrelated to this rename — not fixed
here, out of scope.

**Occurrence sweep (docs/ + README.md + AGENTS.md + internal/core/embed_templates/):**
old role/mode name mentions went from 74 (baseline, `git show HEAD`) to 0
(one confirmed generic-English exception: `docs/contributor-guide.md:31`
"spec builder", unrelated to the persona system).

**Dependencies/blockers:** none — implementation complete.

**Waves:** 1 (mode core) → 2 (mode docs) → 3 (role core, largest/highest-risk)
→ 4 (role templates/agents) → 5 (role docs) → 6 (tests) → 7 (final
validation). All 7 waves complete.
