# AGENTS.md — Operating brief for specd

> **Read `PROJECT.md` first.** It is the single authoritative context document for this
> repository: the product philosophy (Agent = Model + Harness, the eight principles),
> the binding ADRs, the scope triage (29 → 16 verbs), per-domain decisions and
> invariants, the roadmap, and the audited current position with the remaining
> production waves (P0–P6).

Quick orientation:

- `specs/` — the authored per-domain specs (`spec.md` + `tasks.md`) and `progress.md`.
  **Do not trust `progress.md` until it is re-audited (PROJECT.md §8, finding F1).**
- `internal/` + `main.go` — the rebuilt zero-dependency Go binary.
- `reference/` — the frozen v1 implementation. Read-only museum: never import, build,
  or copy from it.
- `The_New_SDLC_With_Vibe_Coding.pdf` — the philosophical anchor.

Non-negotiable guardrails (full detail in PROJECT.md §3): determinism first (no LLM in
any decision/gate/render path), evidence integrity absolute (no completion without a
passing verify record), the ADR-8 hard invariants (atomic writes, CAS on revision,
reentrant lock, parser byte round-trip, embedded templates, zero runtime deps), and
subtractive bias (unsure = CUT/DEFER, recorded).
