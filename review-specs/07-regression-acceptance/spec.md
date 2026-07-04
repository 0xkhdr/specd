# Wave 7 — Regression & Acceptance

> **Order:** 8 / 8 · **Standing regression** — re-audits W0–W6 after they close, gates release alongside W6.
> **Findings:** none new — proves W0–W6 stayed closed and their domain contracts hold together.
> **Sources:** REGRESSION_REVIEW.md (audit method + gaps G1–G5), PROJECT.md §8 F1, review-specs/README.md finding matrix.
> **ADRs touched:** ADR-8 (evidence integrity), ADR-7 (mode enum), ADR-10 (scaffold surface).

REGRESSION_REVIEW.md re-verified Spec 13 by **running the built binary and executing each task's
literal `verify:` — never by trusting `progress.md`** — and still found five gaps a green `progress.md`
had hidden: a falsified 100% (G2/F1), a hollow verify that passed by hitting an absent spec (G4), two
verify commands pointed at deleted targets (G3/G4), and an unimplemented ADR-7 enum (G1/F5). Those are
not W13-specific accidents; they are the failure modes every wave can regress into. This wave makes that
audit **standing and repeatable** across all seven review waves: one harness re-runs every wave's verify
at HEAD, and one per-domain check re-asserts the best-practice invariant each wave owns — so a later wave
cannot silently break an earlier one, and no wave can be marked done ahead of live evidence again.

## 1. Purpose & principles

- **Principles owned:** P3 (evidence gates state change — applied to the whole program, not one wave), P7 (deterministic reporting: the regression verdict is computed, never asserted).
- **Harness components:** observability (a truthful cross-wave regression ledger), instructions (the audit method, recorded so it is reproducible).

## 2. Requirements (EARS)

- **R7.1** When the regression harness runs, the system shall execute **every** `verify:` command
  from W0–W6 (`review-specs/*/tasks.md`) literally at current HEAD and exit non-zero if any one fails —
  no wave's fix is allowed to have regressed another wave's evidence.
- **R7.2** When any wave's `verify:` is inspected, the system shall assert a **real side effect against
  real runtime content** (a spec created under `.specd/specs/`, an on-disk record, an exit that tracks
  findings) — a verify that passes only by hitting an absent/not-found path (the G4 hollow-verify smell)
  shall be flagged and shall fail the harness.
- **R7.3** When each wave's `verify:` and `files:` are checked, every path they name shall exist in the
  tree at HEAD (no stale target — the deleted `fresh-start/00-decisions.md` and the
  authoring-`specs/` vs runtime-`.specd/specs/` mismatch of G3/G4 shall not recur).
- **R7.4** While any wave's findings row (README matrix) is open, the system shall refuse to mark that
  wave complete; and `specd report`/`status` completion for a wave shall equal its evidence-ledger truth
  under `.specd/specs/` (no 100% without a passing record — the F1/G2 falsified-tracker guarantee, generalized).
- **R7.5** When the per-domain regression runs, the system shall re-assert the one best-practice invariant
  each wave owns (the §3 matrix) — honesty (W0), loop-closure + ADR-7 mode enum (W1), trust boundary (W2),
  record provenance (W3), content gates (W4), 16-verb surface + config correctness (W5), release identity +
  dogfood (W6) — and exit non-zero on the first violated invariant.

## 3. Design

- **Cross-wave harness (R7.1):** `scripts/regress-all.sh` — extends the `scripts/audit-progress.sh`
  pattern (run via `sh -c`, log exit codes, verdict from the log, never judgment) but scoped to
  `review-specs/*/tasks.md` instead of `specs/progress.md`. Zero new tooling; a shell loop over the
  markdown verify columns.
- **Verify-quality lint (R7.2/R7.3):** a static pass over the same verify columns — flag any command that
  targets `specs/` (authoring dir) where the runtime `check`/`verify` reads `.specd/specs/`, any bare
  `not-found`-style exit with no positive assertion, and any path failing `test -e`. This is exactly the
  G3/G4 class, turned into a lint so it cannot re-enter.
- **Per-domain matrix (R7.5):** `scripts/regress-domains.sh`, one concrete assertion per wave:

  | Wave | Domain | Regression assertion (best practice re-checked) |
  |---|---|---|
  | W0 | honesty / tracker | `sh scripts/audit-progress.sh` exits 0 **and** no `100%` in `specs/progress.md` unless every ✅ has a passing verify |
  | W1 | lifecycle + mode | `new` writes `mode:"simple"`; `new --mode orchestrated` → `orchestrated`; `new --mode bogus` fails loud; `task complete` writes a record carrying exit code + git HEAD (ADR-7 / G1 closed) |
  | W2 | trust boundary | MCP surface exposes no self-approval path; `brain start` fails closed unless `mode:"orchestrated"`; `pinky` registered |
  | W3 | record provenance | decision/midreq records carry provenance (author role + HEAD); no hollow record accepted |
  | W4 | content gates | the `ears`, `approval`, `contextbudget`, `sync` gates are registered and `check` runs them over real content, exit tracking findings |
  | W5 | CLI surface | the registry exposes **exactly 16 verbs**; config reads the correct key and fails loud on an unknown one; no dead flag |
  | W6 | release identity | `--version` prints the build stamp (`dev` unstamped); CI workflow present; `.specd/` dogfood ledger reports 100% with real evidence |

- **Determinism (P7):** every assertion above is a shell/grep/exit-code check over files or the built
  binary — no LLM, no network, reproducible byte-for-byte on any checkout.

## 4. Invariants preserved

- Never mark ahead of evidence (ADR-8), now enforced across the whole review-specs program, not per wave.
- No new packages or dependencies: two shell scripts reusing the `audit-progress.sh` pattern; the binary
  is the source of runtime truth, the markdown verify columns are the source of intent.
- Subtractive bias honored: W5's assertion re-checks the **cut** to 16 verbs (triage removed), it does not
  restore anything.
- ADR-7 mode enum (`simple`/`orchestrated`) and ADR-10 four-role scaffold are re-asserted, not redefined.
