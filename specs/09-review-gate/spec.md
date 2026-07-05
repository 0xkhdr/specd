# 09-review-gate — Scaffolded review report + opt-in completion block

Wave 2. FINDINGS refs: B.5, D-tier2 item 11.

## Problem

v1 had a review workflow (`specd review`, `review checklist`, opt-in
`config.review.required` gate) blocking the verifying→complete transition
without a fresh approve-verdict review report — structured adversarial
review as a *gate*. Today the reviewer/auditor role prompt exists host-side
(`.claude/agents/pinky-*.md`) but the deterministic half is missing: no
scaffolded report format, no gate that refuses completion without it.
FINDINGS verdict: **port (staged)** — port the gate + scaffold; skip the
fancy checklist extraction initially.

## Requirements (EARS)

- R1: WHEN a user runs `specd review`, THE SYSTEM SHALL scaffold
  `.specd/specs/<slug>/review_report.md` from an embedded template with:
  spec slug, git HEAD under review, per-task section (id, files,
  acceptance), verdict field (`approve|reject|needs-changes`), findings
  section, reviewer identity field.
- R2: IF a review report already exists for the current git HEAD, THEN
  `review` SHALL refuse to overwrite it unless `--force` is given.
- R3: WHEN config enables `review.required` (opt-in, default off), THE
  approval gate for the completion transition SHALL refuse unless a review
  report exists whose verdict is `approve` AND whose recorded git HEAD
  matches the HEAD the evidence is pinned to (freshness — a stale approval
  from an older HEAD does not count).
- R4: WHEN the report's verdict is `reject` or `needs-changes`, THE gate
  SHALL refuse and surface the findings section in the gate output.
- R5: THE report parse SHALL be strict: missing verdict, unknown verdict
  value, or missing HEAD line fails the gate (fail closed), never treated
  as approve.
- R6: THE reviewer SHALL be role-guarded in docs and role prompts: the
  auditor role fills the report; craftsman completing its own review is a
  documented anti-pattern (harness cannot enforce identity — say so
  honestly in docs).

## Design notes / best practice

- Template via `go:embed` next to existing role/steering templates; scaffold
  through `core.AtomicWrite`.
- Parser: reuse the markdown-table/section machinery from the existing
  parsers where possible; keep it byte-tolerant (report is human-edited —
  do not require byte-stability, only field extraction; contrast with
  tasks parser and document why).
- HEAD-freshness (R3) is the load-bearing detail — it is what makes the
  review a *fact about this code*, mirroring evidence pinning.
- Gate joins the registry like `security`/`criteria.required`: opt-in,
  pure function of on-disk state, no LLM anywhere in the gate path (the
  LLM writes the report through the host agent; the harness only checks
  it — thesis preserved).
- Skip for now (record in ADR set): `review checklist` auto-extraction,
  multi-reviewer quorum.

## Out of scope

- Checklist extraction from design/tasks (staged later).
- Reviewer identity verification.

## Acceptance

- `review` scaffolds report; gate off ⇒ behavior unchanged; gate on ⇒
  completion refused until approve-verdict report at evidence HEAD;
  reject verdict surfaces findings; malformed report fails closed; stale-
  HEAD approval refused. Full suite green.
