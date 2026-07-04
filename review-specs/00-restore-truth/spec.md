# Wave 0 — Restore Truth

> **Order:** 1 / 7 · **Blocks:** everything (W1–W6)
> **Findings:** F1 (falsified tracker), F12 (repo `.specd/` contradicts scaffold), F13 (wrong `files:`)
> **Sources:** PROJECT.md §8 Wave P0, BUILD_REVIEW.md §3 F1/F12/F13 + §5 Wave 0
> **ADRs touched:** ADR-8 (evidence integrity), ADR-10 (scaffold surface)

The tracker of a spec-discipline tool must be a projection of reality. Today
`specs/progress.md` marks all 76 tasks ✅ while several verify commands fail, no
evidence records exist for any rebuild task, and the repo's own `.specd/` violates the
scaffold the binary ships. Nothing downstream can be trusted or dogfooded until this
wave closes. This is honesty, not features.

## 1. Purpose & principles

- **Principles owned:** P3 (evidence gates state change — restored to the tracker itself), P2 (specs as source of truth).
- **Harness components:** observability (truthful tracker), instructions (missing docs).

## 2. Requirements (EARS)

- **R0.1** When `specs/progress.md` is re-audited, the system's tracker shall show ✅
  only for tasks whose `verify:` command passes when literally executed at current HEAD;
  every task whose verify fails (at minimum T1.1, T2.4, T2.6, T5.4, T8.6, T12.3) shall
  be flipped back to ⬜, and every task's `files:` shall name files that actually exist
  in the tree (F13 — work consolidated into `lifecycle.go`/`registry.go`/`report.go`
  must be reflected, not the never-created per-feature filenames).
- **R0.2** When the repo's own `.specd/` is inspected after reset, it shall contain
  exactly the four ADR-10 roles (`scout,craftsman,validator,auditor` — no `scribe.md`),
  the six steering files, and no junk specs (`specs/demo/` with its invalid `builder`
  role deleted).
- **R0.3** When the original T1.1 / T8.6 / T12.3 verify commands run, they shall pass:
  `docs/charter.md` exists and maps every registered verb to one harness component +
  one principle; `docs/context.md` documents the manifest engine contract (four item
  modes, budget, heuristic estimator); `docs/deferred-flywheel.md` documents the
  deferred evidence shapes (`DeployApproval`, `EvalSummary`, inventory waivers) and the
  two re-entry seams (Gate interface + `state.records`).
- **R0.4** While any W0 requirement is open, no W1–W6 task shall be marked complete
  (audit precedes rebuild-completion claims).

## 3. Design

- **Audit method (R0.1):** run every `verify:` in `specs/progress.md` literally via
  `sh -c`, record exit codes to a scratch log, flip status from the log — never by
  judgment. Correct `files:` by checking each path with `test -e`; where consolidated,
  point at the real file and append `(consolidated)`.
- **Scaffold reset (R0.2):** delete `roles/scribe.md` + `specs/demo/`, run the built
  `specd init` in place (marker-merge preserves AGENTS.md user content), add
  `roles/auditor.md` via the embedded template — the binary, not hand-writing, is the
  source of scaffold truth.
- **Docs (R0.3):** `docs/charter.md` content comes from PROJECT.md §1 (seven components,
  eight principles) + §5 verb table; charter is the future lint source (spec 01), so
  the verb list must match the registry exactly. `docs/context.md` from PROJECT.md
  domain 08; `docs/deferred-flywheel.md` from domain 12 / ADR-5.

## 4. Invariants preserved

- Never mark ahead of evidence (ADR-8 evidence integrity, applied to the tracker).
- ADR-10: exactly four roles; embedded templates are the single scaffold source.
