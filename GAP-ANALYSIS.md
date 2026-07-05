# Gap Analysis — specs/progress.md finalization

**Date:** 2026-07-05
**Scope:** Review the FINDINGS remediation program (`specs/progress.md`) against
implementation reality to close the last gaps before declaring the program done.
**Method:** built the binary, ran the full suite + every lint/regression harness,
and cross-checked each spec's stated acceptance against on-disk code, scripts,
and decision records.

---

## TL;DR

The **code is done and healthy** — all 14 specs are functionally implemented and
every gate passes. The remaining gaps are **tracking + governance drift**, not
missing features:

1. `progress.md` Wave 0 status is **stale** (marked `pending`; actually `done`) —
   and internally contradicts its own wave-ordering invariant.
2. The **Wave 3 demand-gated ADRs are missing** — `00-hygiene` R5 requires one
   decision record per skipped v1 capability (9 of them); none exist.
3. `scripts/audit-progress.sh` is **orphaned** — it audits a task-row format
   `progress.md` no longer uses, so it silently passes on zero matches
   (false-green infrastructure).

Close these three and the program is genuinely finalized.

---

## Health check (all green)

| Gate | Result |
|---|---|
| `go build .` | ✅ success |
| `go test ./...` | ✅ 247 passed, 13 packages |
| `gofmt -l .` | ✅ clean |
| `go vet ./...` | ✅ no issues |
| `scripts/test-lint.sh` | ✅ ok |
| `scripts/docs-lint.sh` | ✅ ok (CHEATSHEET mirrors command-reference) |
| `scripts/regress-lint.sh` | ✅ clean — no smells |
| `scripts/verify-progress.sh` | ✅ exit 0 |
| FINDINGS coverage | ✅ every B/C ref maps to a spec |

**Wave 0 is functionally complete** despite the `pending` label — verified present:
`internal/version/version.go` (+ `specd version` verb), `schema_check.go` +
state.json migration, `config_validate.go:93` unknown-key rejection,
`.goreleaser.yml` + `.github/workflows/release.yml`, `docs/CHEATSHEET.md`, and all
eight promised `scripts/*.sh`.

---

## Gaps (ranked)

### GAP-1 — `progress.md` Wave 0 status is stale and self-contradictory  `[high]`

Wave 0 (`00-hygiene`, `01-version-release`, `02-state-schema-version`) is marked
`pending`, but all three are fully implemented and passing. The document also
states its own invariant: *"a wave may start only when every spec in the prior
wave is `done`."* Waves 1–2 are all `done` while Wave 0 reads `pending` — an
impossible state under that rule. The tracking doc currently lies about program
status.

- **Fix:** set Wave 0's three rows to `done` with completion notes matching the
  verified artifacts. Add a one-line completeness footer (e.g. "all 14 specs
  done; suite green 2026-07-05").
- **Effort:** trivial (edit `progress.md`).

### GAP-2 — Wave 3 demand-gated decision records are missing  `[high]`

`specs/00-hygiene/spec.md` R5 requires **one ADR per deliberately skipped v1
capability**, each with reasoning + an explicit revisit trigger:
`triage`, `conductor`, `dashboard`, `packs`, `harness-sharing`, `ingest`,
`deploy`, `observe`, `eval/prototype` (9 total, per FINDINGS B.8–B.12, B.17,
B.24, B.25, C.2). `progress.md` §Wave 3 asserts *"Wave 0's `00-hygiene` records
the skip/defer decision for each."*

Reality: `docs/decisions/` holds 6 ADRs (0001–0006), and **none** are these skip
records. `0001-wave0-hygiene.md` documents only the Makefile-absence decision.
The claimed skip records do not exist anywhere in the repo. This is an unmet
acceptance criterion, not just a doc nit — the program's "subtractive bias /
record the decision" invariant is unbacked for every Wave 3 item.

- **Fix (choose one):**
  - (a) One consolidated ADR `docs/decisions/0007-demand-gated-skips.md` with a
    subsection per capability (Status / Context / Decision / Revisit-trigger).
    Lightest; matches the "decision records only" framing.
  - (b) Nine separate ADRs (0007–0015) — literal R5 reading, heavier.
  - **Recommended: (a)** — one record, nine subsections. Then repoint
    `progress.md` §Wave 3 at it.
- **Effort:** ~1 focused authoring pass (content already argued in FINDINGS + the
  §Explicit non-goals block; needs formalizing into an ADR with revisit triggers).

### GAP-3 — `scripts/audit-progress.sh` is orphaned (false-green)  `[medium]`

The script parses `progress.md` for task-level rows
(`| T13.13 | … | ✅ | <verify-cmd> |`) and re-runs each verify command. But the
current `progress.md` is **program/wave-level** (`| spec | scope | status | notes |`)
and the per-spec `tasks.md` files have **no status column and no `✅` markers**.
So the script matches zero rows and exits 0 — it audits nothing while appearing
to pass. `00-hygiene` R1 promised it as working progress-verification
infrastructure.

- **Fix (choose one):**
  - (a) Repoint it to walk each `specs/*/tasks.md` and re-run their `verify:`
    columns (this is what `regress-all.sh` already does against `.specd` tasks —
    confirm overlap first to avoid duplicating).
  - (b) If `regress-all.sh` + `verify-progress.sh` already cover the intent,
    **delete** `audit-progress.sh` and drop it from docs/spec R1 (subtractive
    bias — remove dead infrastructure rather than maintain a no-op).
  - **Recommended: (b)** unless a distinct audit need is identified. Verify no CI
    job or doc references it before removing.
- **Effort:** small; decision-gated on whether it duplicates `regress-all.sh`.

### GAP-4 — Minor FINDINGS ref-scheme drift  `[low / cosmetic]`

`progress.md` cites `B.20` and `C.7`, which don't appear in `FINDINGS.md`'s
ref enumeration (FINDINGS runs B.1–B.19, B.21–B.26, C.1–C.6, C.8). The `D.1–D.15`
refs are `progress.md`-local doc-task labels with no FINDINGS counterpart. No
functional impact — traceability cosmetic.

- **Fix:** either add the missing `B.20`/`C.7` anchors to `FINDINGS.md` or drop
  them from `progress.md`; note that `D.*` are progress-local. Optional.
- **Effort:** trivial; do only if strict traceability is wanted.

---

## Action plan

| # | Action | Gap | Priority | Effort |
|---|---|---|---|---|
| 1 | Flip Wave 0 rows to `done` + add completeness footer in `progress.md` | GAP-1 | High | Trivial |
| 2 | Author `docs/decisions/0007-demand-gated-skips.md` (9 subsections, each with revisit trigger); repoint `progress.md` §Wave 3 | GAP-2 | High | ~1 pass |
| 3 | Decide `audit-progress.sh` fate: repoint to `tasks.md` verifies **or** delete as dead no-op (check no CI/doc refs first) | GAP-3 | Medium | Small |
| 4 | (Optional) Reconcile `B.20`/`C.7`/`D.*` ref drift between FINDINGS and progress | GAP-4 | Low | Trivial |

**Suggested order:** 2 → 3 → 1 → 4. Do GAP-2 first (the real acceptance gap),
then resolve GAP-3's script fate, then flip the status in one pass once the
governance artifacts back it, then optional cleanup.

**Definition of done for the program:** all 14 rows `done`, one ADR per Wave 3
skip with revisit triggers, no orphaned/false-green scripts, suite + all lints
green. After actions 1–3, the FINDINGS remediation program is fully finalized.
