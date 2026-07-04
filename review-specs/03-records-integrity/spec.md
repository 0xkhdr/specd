# Wave 3 — Make Records Mean Something

> **Order:** 4 / 7 (parallel with W4) · **Depends:** W1
> **Findings:** F6 (hollow decision/midreq/approval records), F14 (git_head "unknown", no timestamps)
> **Sources:** PROJECT.md §8 Wave P3, BUILD_REVIEW.md §5 Wave 3, specs/02 + 05, ADR-6/8
> **Files:** `internal/cmd/lifecycle.go`, `internal/core/{state,task_complete}.go`

A "recorded human decision" that records `{"kind":"decision"}` records nothing. The
paper's observability requirement ("audit exactly why a decision was made", p.30)
demands every record carry content, provenance, and a pin to a commit.

## 1. Purpose & principles

- **Principles owned:** P3 (evidence integrity), P7 (deterministic observability), P2.
- **Harness components:** observability (audit trail), guardrails (completion pinning).

## 2. Requirements (EARS)

- **R3.1** When `decision` or `midreq` runs, the system shall require `--text` (usage
  error without it) and accept optional `--scope`; and every record the system writes
  (approval, mode transition, decision, midreq, evidence, unverified-attestation) shall
  carry `timestamp` (RFC 3339 UTC from the injectable `Clock`), `git_head`, and `actor`
  (`$SPECD_ACTOR` if set, else OS user). Approval records shall name the gate approved
  *and* the artifact revision they approved.
- **R3.2** When a verify run cannot resolve git HEAD (`git_head: "unknown"`), the system
  shall warn at `verify` time and shall refuse at `task complete` time — an evidence
  record that cannot be pinned to a commit does not count toward completion.
- **R3.3** When existing records lacking the new fields are loaded (pre-W3 ledger
  entries), the system shall load them without error but shall not accept them as
  completion evidence (fail-loud on use, tolerant on read — append-only ledger is never
  rewritten).
- **R3.4** When `status --json` renders, decisions and midreq gates shall round-trip
  their text/scope/actor/timestamp verbatim (projection, never synthesis); high/critical
  midreq gates shall remain never-auto-cleared.

## 3. Design

- **One enrichment point:** a single `stampRecord(rec)` helper in core fills
  timestamp/git_head/actor for every record kind — not per-callsite copies. Uses the
  injectable `Clock` (determinism in tests).
- **Ledger compatibility (R3.3):** the ledger stays append-only; old records are
  distinguishable by absent fields, and `CompleteTask` rejects them with a message
  naming the re-verify remedy.
- **Actor:** `$SPECD_ACTOR` is in the scrubbed-env allowlist already (`SPECD_*`);
  host-reported, stored verbatim, never trusted as proof (guardrail §3).

## 4. Invariants preserved

- Append-only evidence ledger — no rewriting old records to add fields.
- CAS + lock on every state write; `SchemaVersion` stays 1 (fields are additive to
  record payloads, not core `State` — ADR-6 boundary respected).
- Determinism: timestamps only via `Clock`; renders remain pure projections.
