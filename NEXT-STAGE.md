# Next-Stage Recommendations

Written 2026-07-09, after landing R2–R6 from `IMPLEMENTATION-PLAN.md`. This file names
what remains and what the just-shipped work opened up. Standing invariants apply to
everything below: no new runtime deps, atomic writes + CAS on `state.json`, no LLM in any
gate/DAG/decision path, `reference/` untouched, `docs/command-reference.md` ↔
`docs/CHEATSHEET.md` kept in sync.

## Status of this session

| Item | What shipped | Test |
|---|---|---|
| R3 | `specd init` scaffolds a commented, idempotent `project.yml` (`verify.timeout_seconds: 600` active) | `TestInitWritesBoundedProjectConfig` |
| R6 | config→exec→evidence records a timed-out verify as exit 124; task can't complete | `TestVerifyTimeout` |
| R5 | in-step lease release for completed tasks | `TestBrainStepReleasesCompletedLease` |
| R4 | dropped dead `ConfigPaths.Global`; one `configPaths(root)` helper routes all callers | `TestConfigCascade` (rewritten) |
| R2 | doc-only: `brain run` = one wave, not run-to-completion | `docs-lint.sh` |

Gates green: `gofmt -l .` empty, `go vet`, `docs-lint`, `test-lint`, `-race -count=1`,
`-count=2`, `regress-{all,domains,lint}`.

---

## Stage 1 (P1) — R1: dogfood the top-level `specs/` into `.specd/specs/`

The one plan item still open, and the highest-leverage one: the nine gap-closure specs live
in top-level `specs/` as hand-maintained markdown with no `state.json`, verify record, CAS,
or lock. Manual tracking drift is exactly the failure mode the product exists to prevent.

**Why it's staged separately.** It's not a code diff — it's an operational migration that
rewrites live planning artifacts and needs a schema translation between two formats. Do it as
its own spec, driven through the pipeline it migrates, so a bad intermediate state can't
half-land.

**Recommended path: pilot one slug first.** Migrate a single spec (`cli-contracts` is the
smallest surface) end-to-end, get the schema translation reviewed, then batch the rest.

1. **Schema map (do once).** Top-level uses `wave`/`deps` and free-form `spec.md` sections.
   Runtime expects `.specd/specs/<slug>/{requirements.md,design.md,tasks.md,state.json,.lock}`
   with EARS requirements, the design sections, and the tasks table schema the gates in
   `internal/core/gates/` enforce. Write the `wave`→DAG-`deps` and section→schema translation
   as a repeatable recipe (a throwaway script or a documented manual procedure), not ad hoc.
2. **Migrate per slug**, then drive each through `specd check` → `specd verify` →
   `specd approve` so per-task evidence is captured and `progress.md` becomes generated from
   `state.json` instead of hand-maintained.
3. **Retire** top-level hand-tracking once the pipeline is authoritative; keep
   `AUDIT-FINDINGS.md` as a historical record if useful.
4. **Watch `regress-lint.sh` smell "A"** — it guards the split between repo-planning `specs/`
   and runtime `.specd/specs/`. The migration must not blur verify lines across the two trees.

**Done when:** each migrated task carries a passing verify record pinned to git HEAD, `specd
status` reflects real state, and `regress-all.sh` / `regress-domains.sh` stay green.

---

## Stage 2 (P2) — follow-ups this session opened

- **`brain run` run-to-completion (R2 option B), if operators want it.** R2 shipped as
  doc-only ("run = one wave"). If unattended run-to-done is a real need, it's its own design:
  a worker round-trip inside the loop (block on evidence for the leased task, re-sense the
  frontier), with the worker call as the *only* non-deterministic step — the LLM must stay out
  of Decide/Sense. Needs timeout and crash-recovery-of-a-mid-flight-wait design.

- **Machine-wide config (R4 option B), if asked.** R4 dropped the unreachable global layer. If
  machine-wide defaults become a real user need, wire `Global` from
  `$XDG_CONFIG_HOME/specd/config.yml` (fallback `~/.config/specd/config.yml`) through the new
  `configPaths(root)` helper (now the single source of truth, so the change lands in one
  place), and add a loader test that global layers *under* project and env wins over both.

- **`init --dry-run` coverage.** `previewManaged` now reports `+ project.yml (new operator
  config)`; the write path is tested but the dry-run print line is not. Low priority — add a
  cmd-level assertion if the preview surface grows.

---

## Definition of done (any item)

- `gofmt -l .` empty, `go vet ./...` clean, `go mod tidy` no diff (zero new deps).
- `go test ./... -race -count=1` and `-count=2` green.
- `docs-lint.sh` green for any CLI/flag/doc change.
- `regress-all.sh`, `regress-domains.sh`, `regress-lint.sh` green.
- Evidence integrity preserved: no bypass flag anywhere.
