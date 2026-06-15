# spec.md — Replay & Spec Diff / Time-travel

**Status:** proposed
**Source:** specd-report.html §8 idea **D3** (impact: med · effort: med · moat: med)
**Date:** 2026-06-16
**Scope:** new `specd replay` + `specd diff` read-only commands over existing audit records.

---

## 1. Objective

`specd replay <slug>` reconstructs the full decision timeline (approvals,
verifies, midreqs, ADRs) and `specd diff` shows how requirements/design evolved
across phases. The records already exist — surface them as a narrative.
Post-mortems and onboarding need "how did we get here"; specd already has the
audit trail, so make it readable.

> **Hard invariant:** read-only, deterministic, stdlib-only, no LLM. Both
> commands derive output purely from on-disk records (state.json, ledgers,
> decision/midreq files, git history of artifacts) — they mutate nothing and
> compute no new judgments. Identical inputs ⇒ byte-identical output.

## 2. Context

- Audit sources already on disk: `state.json` (status history, verify records),
  decision ADRs (`internal/cmd/decision.go`), mid-requirement logs
  (`midreq.go`), append-only ledger writes (`io.go` `AppendFile`).
- Artifact evolution (requirements/design) is recoverable from git history of
  the spec files.
- `SPECD_JSON=1` structured-output convention applies.

## 3. Requirements (EARS)

- **R1 (H)** WHEN `specd replay <slug>` runs, THE SYSTEM SHALL emit a
  chronologically ordered timeline merging approvals, verify records, midreqs,
  and decisions from the existing records.
- **R2 (M)** WHERE `SPECD_JSON=1` is set, the timeline SHALL be emitted as an
  ordered JSON array of typed events (type, timestamp, ref, summary).
- **R3 (M)** WHEN `specd diff <slug> --from <phase> --to <phase>` runs, THE
  SYSTEM SHALL show how `requirements.md`/`design.md` changed between those
  phases, sourced from git history of the artifacts.
- **R4 (M)** THE commands SHALL be strictly read-only — no state mutation, no
  lock acquisition beyond shared reads.
- **R5 (M)** IF an expected record source is missing or partially corrupt, THE
  SYSTEM SHALL render what exists and note the gap, never panic.
- **R6 (L)** THE timeline SHALL be deterministically ordered (stable sort on
  timestamp then a tie-break key) so output is reproducible.

## 4. Design / approach

1. **Event collector** — `internal/core/replay.go`: read state status history +
   verify records, parse decision/midreq files, normalize to a common
   `TimelineEvent{Type, Time, Ref, Summary}`.
2. **Ordering** — stable sort by `(Time, Type, Ref)` for determinism.
3. **Diff** — `internal/cmd/diff.go`: resolve the commits where the spec entered
   `--from`/`--to` phases (status-transition timestamps mapped to git), shell to
   `git diff` on the artifact paths.
4. **Render** — text narrative + `SPECD_JSON=1` array.

## 5. Non-goals

- No LLM summarization — events are rendered from recorded fields verbatim.
- No mutation/repair of records; this only reads.
- No new audit fields (works off what's already recorded).

## 6. Acceptance criteria

- `specd replay` emits a correctly ordered timeline merging approvals/verifies/
  midreqs/decisions; `SPECD_JSON=1` gives a typed event array.
- `specd diff --from --to` shows artifact evolution from git history.
- Missing/partial records ⇒ graceful note, no panic.
- Output is deterministic for identical inputs; commands mutate nothing;
  `make ci` green; stdlib-only.
