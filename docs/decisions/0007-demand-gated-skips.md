# 0007 Demand-Gated v1 Capability Skips

Status: accepted

Context:
The subtractive-bias invariant requires that every deliberate skip be *recorded
with a decision*, not silently dropped. `00-hygiene` R5 makes this concrete: one
decision record per v1 capability left unported, each with reasoning and an
explicit revisit trigger. FINDINGS (§B, §C) evaluated the full v1 surface and
marked nine capabilities Skip/Defer for Wave 3 — "decision records only, no
implementation." This ADR is that record. Each capability below is Wave 3:
unimplemented by design, revived only on demonstrated demand, and a revival
means a *new spec*, never an edit to this file.

The revisit trigger for each is the concrete signal that flips the cost/benefit —
until it fires, building the capability is inverted priority (surface before
users) or duplicated by the host harness.

Decision:

## triage (C.2, FINDINGS §2)
- **Status:** skipped (verb registered, deferred at dispatch).
- **Context:** The extended-loop triage tier ranks/routes incoming work. A
  `triage` verb is registered but permanently deferred with, until now, no
  recorded decision — the exact gap R5 exists to close.
- **Decision:** Do not implement. Routing across specs is program-frontier work
  already served by `status --program` (spec 12); a ranking tier adds LLM-shaped
  judgment the harness deliberately keeps out of its deterministic paths.
- **Revisit-trigger:** A user runs enough concurrent specs that manual frontier
  selection is a measured bottleneck, AND a *deterministic* ranking rule (not an
  LLM heuristic) can be stated. Then: new spec, not a verb un-defer.

## conductor (B.12)
- **Status:** skipped.
- **Context:** Interactive micro-task sessions with an append-only ledger and
  mandatory rejection reasons.
- **Decision:** Do not implement. Fully overlapped by interactive agent harnesses
  (Claude Code itself). The one durable idea — mandatory rejection reasons as a
  training signal — is capturable through `midreq` without a session runtime.
- **Revisit-trigger:** A non-interactive host needs micro-task arbitration that no
  agent harness provides. Otherwise absorb the rejection-reason idea into `midreq`.

## dashboard (B.17)
- **Status:** skipped (explicit non-goal).
- **Context:** Unified loopback web dashboard.
- **Decision:** Do not implement. Large UI surface, zero enforcement value.
  `status --json` plus external tooling covers observation without adding a server,
  a template stack, or a live-update channel to a zero-dependency binary.
- **Revisit-trigger:** None internal. If wanted, it is an out-of-tree consumer of
  `status --json` / `report --format prometheus`, never code inside the binary.

## packs / registry (B.24)
- **Status:** skipped.
- **Context:** Spec packs (`init --pack`, `--registry`, SHA256-pinned `pack.lock`,
  declarative-only manifests).
- **Decision:** Do not implement. A distribution ecosystem before there is a user
  base is inverted priority. The v1 security design (SHA-256 pinning, no executable
  hooks) is sound and to be reused verbatim when built.
- **Revisit-trigger:** Multiple teams request shared, versioned spec templates.
  Then: new spec porting the pinning/quarantine design.

## harness sharing (B.25)
- **Status:** skipped.
- **Context:** `harness push/pull/list/enable` — team policy sharing with
  quarantine of executable artifacts.
- **Decision:** Do not implement. Same reasoning as packs; the quarantine pattern
  for executable artifacts is the part to remember.
- **Revisit-trigger:** A team needs to distribute shared policy/steering with
  integrity guarantees. New spec; reuse the quarantine design.

## ingest (B.11)
- **Status:** skipped.
- **Context:** Legacy-codebase inventory (`inventory.json`, countable facts only)
  plus an opt-in coverage gate.
- **Decision:** Do not implement. Brownfield adoption is a separable product
  surface, not a core-enforcement gap.
- **Revisit-trigger:** A brownfield adopter needs coverage attribution against an
  existing codebase. New spec: countable-facts inventory + opt-in gate.

## deploy (B.8)
- **Status:** deferred.
- **Context:** `deploy` / `deploy rollback` — evidence-gated deploy driver over
  `.specd/deploy/<env>.json`, append-only `deploy.jsonl`, production approval.
- **Decision:** Do not implement. Well-designed but a large hostile-input surface
  (step schemas, timeouts, rollback chains) with no current consumer.
- **Revisit-trigger:** A real user needs post-merge deploy enforcement. Then port
  the driver behind the same evidence/approval discipline as task verify.

## observe (B.9)
- **Status:** deferred.
- **Context:** `observe correlate` (+ optional loopback listener) — production
  error → spec attribution → evidenced mid-requirement.
- **Decision:** Do not implement. The flywheel is elegant but presumes the deploy +
  ledger infrastructure of `deploy` (B.8); it cannot precede it.
- **Revisit-trigger:** Same as deploy — fires only after deploy ships and a user
  needs production-error → spec attribution.

## eval / prototype (B.10)
- **Status:** deferred (partial-port candidate).
- **Context:** `eval` / `promote` / `--prototype` lifecycle — deterministic rubric
  engine (`artifact_present`, `regex`, `trajectory`, sandboxed `command`), `eval
  init` from approved requirements, `eval trend`, prototype specs that cannot
  complete until promoted with evidence.
- **Decision:** Do not implement now. FINDINGS marks this the one adapt-worthy item
  in the set: the rubric engine minus `trajectory` is small and reuses the existing
  sandboxed exec path, and prototype/promote gives spike work a home inside specd
  rather than faking full planning or living outside it. `trend` analytics stay
  dropped regardless.
- **Revisit-trigger:** Spike/exploratory work recurs often enough to warrant a
  first-class prototype phase. Then: new spec porting prototype + a minimal rubric
  (`artifact_present`, `regex`, sandboxed `command`), no `trajectory`, no `trend`.
