---
name: specd-eval-author
description: Refine a specd eval rubric. Load after `specd eval <slug> init` compiles a stub rubric from approved requirements, or when hardening a suite. Covers the four check kinds, scoring/minScore, the trajectory predicates, and the sandbox contract for `command` checks.
---

# specd eval author

`specd eval <slug> init` compiles `requirements.md` into a rubric skeleton at
`.specd/specs/<slug>/eval-rubric.json` — one stub `regex` check per acceptance
criterion (`crit-<r>-<n>`). That transform is deterministic and
interpretation-free; **your** job is to turn each stub into a real check that
proves the criterion, then run `specd eval <slug>`.

The binary never reasons about quality — it only runs the checks you write and
sums their weights. Keep every check mechanical and reproducible.

## Check kinds

Each check has an `id` (`^[a-z0-9][a-z0-9-]*$`), a `kind`, a `weight` (points,
default 1), and kind-specific fields.

- **`regex`** — RE2 `pattern` over the file at `path` (spec-relative). Use for
  "the design records X" or "the artifact mentions Y". `artifact_present` is
  just a permissive pattern (e.g. `.`) over the file you expect to exist.
- **`command`** — runs `command` through the shared sandboxed exec path (env
  scrub, `timeoutMs`, `sandbox: none|bwrap|container`, optional `workingDir`).
  Exit 0 = pass. This is where an LM judge plugs in: point it at a user script;
  its exit code plus a stdout digest are recorded as evidence.
- **`trajectory`** — a `predicate` over the V3 trajectory ledger:
  - `exists` — the ledger has any events.
  - `max-events` — event count `<= max`.
  - `min-events` — event count `>= min`.
  - `pattern` — RE2 `pattern` matches somewhere in the ledger (e.g. a tool name).
  Proves *how* the work was done, not just the artifact.

## Scoring

`score = sum(weight of passing checks) / sum(all weights)`. The suite passes
when `score >= minScore` (0..1). `specd eval` exits 1 below `minScore` and
records the run to `state.json.evals` plus a sequenced result file under
`.specd/specs/<slug>/evals/`.

## Rules

- **Fail closed.** Unknown kinds, absolute/`..` paths, NUL bytes, and bad
  regexes are rejected at load — the validator will tell you the offending line.
- **Deterministic.** No timestamps, no network, no host-specific paths in
  patterns. The same rubric + artifacts must always yield the same score.
- **`command` is hostile-input territory.** Never interpolate untrusted text
  into a command; keep the script in-repo and reviewed.
- Refine, run, read the failures, tighten. Result files are evidence — prune
  them via VCS, not by hand.

## Loop

```
specd eval <slug> init          # compile stubs from approved requirements
# edit .specd/specs/<slug>/eval-rubric.json — real checks, weights, minScore
specd eval <slug>               # score; exit 1 below minScore
specd eval <slug> trend         # score deltas + failure clustering over history
```
