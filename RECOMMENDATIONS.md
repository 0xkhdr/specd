# Recommendations — post-audit follow-ups

Written 2026-07-09 after closing `specs/AUDIT-FINDINGS.md`. The audit's findings are resolved
(code gaps closed, record reconciled, divergences recorded). These are the **next** issues —
structural risks and loose ends the audit did not cover or left as future work. Priority: P1 =
real gap worth scheduling; P2 = polish / hardening.

---

## P1 — Dogfood the harness on its own specs (audit §1.4, still open)

The nine gap-closure specs live in top-level `specs/` as hand-maintained markdown, **not** in
`.specd/specs/` under specd's own evidence-gated pipeline. No `state.json`, no verify record,
no CAS, no lock. The tracking drift this whole audit fixed (`progress.md` over-claiming) is the
predicted failure mode of manual tracking — exactly what the product exists to prevent.

**Recommendation.** Migrate the in-flight planning specs into `.specd/specs/` and drive them
through `specd verify` / `specd approve`. Then per-task evidence is captured automatically and
`progress.md` cannot drift, because it stops being hand-maintained. This is the single highest-
leverage change: it makes the product prove itself.

**Cost.** Non-trivial — the two schemas differ (top-level uses `wave`/`deps`; runtime expects
the on-disk `.specd/` layout). Do it once, as its own spec.

---

## P1 — `brain run` dispatches but nothing drives worker completion in-process

`runBrainRun` (`internal/cmd/brain_run.go`) now loops until the frontier drains, but a
dispatched mission is executed by an **external** `pinky-*` subagent, not by the controller. So
`brain run` dispatches every ready task, then stops — it does not wait for workers to report and
re-plan. It is "dispatch the current wave," not "run to completion."

**Recommendation.** Decide and document the intended autonomy boundary:
- If `run` should be one wave: rename the mental model in docs (`command-reference.md` calls it
  "run") so operators don't expect run-to-done.
- If `run` should drive to completion: it needs a worker round-trip inside the loop (block on
  evidence for the leased task, then re-sense). That is a larger design — keep the LLM out of
  the decision path (invariant), so the worker call is the only non-deterministic step.

Until decided, `run` is honest but easy to misread.

---

## P1 — Verify timeout defaults to unbounded, and there is no scaffolded `project.yml`

`verify.timeout_seconds` defaults to `0` (unbounded) — chosen to be non-breaking. But `specd
init` does not scaffold a `project.yml` at all (confirmed: no writer references it), so in a
fresh project the safety bound is not just off, it is invisible. The hang-the-pipeline gap the
audit named (4.2) is *closeable* now but not *closed by default*.

**Recommendation.** Scaffold a commented `project.yml` in `specd init` with a conservative
`verify.timeout_seconds` (e.g. `600`) and the other opt-in gates documented inline. Operators
then see the knob and start bounded. Low risk; high discoverability win.

---

## P2 — `ConfigPaths.Global` is dead in the CLI

Every CLI config load passes only `Project: <root>/project.yml`
(`brain_run.go`, `criteria.go`, `escalation.go`, `memory.go`, `registry.go`). `ConfigPaths.Global`
is honored by `LoadConfig` but never populated outside tests. The
`project-diagnostics-config` decision note claims a "global YAML layered under" project config —
supported by the loader, but no CLI path supplies it.

**Recommendation.** Either wire a real global path (e.g. `$XDG_CONFIG_HOME/specd/config.yml`) so
the layering is true, or drop the `Global` field and simplify `LoadConfig` to one source +
env. Do not leave a documented feature with no way to reach it (subtractive bias favors
dropping it unless a user needs machine-wide defaults).

---

## P2 — Lease-release assertion is indirect

`stress-orchestration.sh` now asserts no lease survives after the resume race — but that
exercises `brain resume`'s lease clearing, not the new completion-release in `runBrainStep`.
The completion path is covered only transitively by the brain suite.

**Recommendation.** Add one focused Go test: seed a session with a live lease on task `T1`,
mark `T1` complete in `tasks.md`, run one step, assert the lease is gone from `session.json`.
Small, deterministic, and pins the exact behavior the audit's gap 5.4 asked for.

---

## P2 — No regression coverage that a timed-out verify is *recorded*, only that it returns 124

`internal/core/verify/timeout_test.go` proves `Run` returns exit 124 on deadline. The wiring
that turns that into a **failing evidence record** (`runVerify` in `registry.go` →
`AppendEvidence`) has no end-to-end test.

**Recommendation.** Add a `cmd`-level test: a task with `verify: sleep 5`, `project.yml` with
`verify.timeout_seconds: 1`, run `specd verify`, assert the evidence record carries exit 124 and
the task does **not** complete. Closes the loop from config → exec → evidence.

---

## Standing invariants (unchanged, restated so follow-ups don't erode them)

Zero new runtime deps. Atomic writes + CAS on `state.json`. No LLM in any gate/DAG/decision
path. Docs-lint green (`command-reference.md` ↔ `CHEATSHEET.md`). `reference/` untouched.
Evidence integrity: no task completes without a passing verify record; **no bypass flag**.
