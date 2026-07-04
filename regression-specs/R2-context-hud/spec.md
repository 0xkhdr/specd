# Spec R2 — Context HUD

> **Regression of:** implemented rebuild vs. MVP intent · **Kind:** missing MVP surface flag
> **Sources:** `fresh-start/08-context-engineering.md`, `fresh-start/11-reporting-observability.md`
> **ADRs:** ADR-8 (determinism in render path)
> **Reference:** `reference/internal/core/commands.go` (`context --hud`: "load files, byte/token
> cost, mode/tier")

A human-readable view of the *same* context manifest the harness already computes. The
estimator, manifest, and budget engine were rebuilt (`internal/context/`); only the operator
render (`--hud`) was dropped. This adds the render — a pure projection, no new computation.

---

## 1. Purpose & principles
- **Principle owned:** P3 (lean, deliberate context). The HUD makes the byte/token cost of a
  task's context *visible* so a human can see when a spec is overloading the window before
  dispatch.
- **Paper concept:** *context engineering* — you cannot economize what you cannot see; the HUD
  is the instrument.
- **Why it regressed:** `context` shipped `--json` (machine surface) but not `--hud` (operator
  surface); the underlying `manifest`/`estimate`/`budget` already produce every number the HUD
  needs, so the gap is render-only.

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| `context <task> --hud` operator render | **KEEP-lite** | Domain 08 surface; `reference` `context --hud` |
| Per-file byte + estimated-token cost + total | **REUSE** | `internal/context/{manifest,estimate,budget}.go` already compute these |
| Mode/tier line | **KEEP** | reads spec `mode` (Spec 02) — no new state |
| HUD numbers must equal `--json` numbers | **KEEP (hard)** | one engine, two renders; no divergent math |

**Minimal accurate surface:** one flag `--hud` on the existing `context` command; one module
`internal/context/hud.go` that formats an already-built `Manifest`. No new state, no config.

## 3. Requirements (EARS)
- **RH.1** When `context <task> --hud` is invoked, the system shall render a table of the
  manifest's load files with each file's byte size and estimated token cost, a total row, and
  the spec's `mode`/tier — computed from the same `Manifest` the `--json` surface serializes.
- **RH.2** The HUD render shall be a pure function of the built `Manifest` (no LLM, no second
  estimation pass), sharing the estimator with the gate/budget path.
- **RH.3** For a given task, the byte and token totals shown by `--hud` shall equal those
  emitted by `--json` (the two surfaces never diverge numerically).
- **RH.4** When `--hud` and `--json` are both supplied, the system shall reject the combination
  with a usage error (one render per invocation).

## 4. Design

### Module boundaries
- `internal/context/hud.go` — `RenderHUD(m Manifest) string`: pure formatter over the existing
  `Manifest` type from `manifest.go`. Reuses `estimate.go`'s token count already stored on the
  manifest; performs no new estimation.
- `internal/core/commands.go` — add the `--hud` flag to the `context` command meta.
- `internal/cmd/registry.go` — in the `context` handler, branch to `RenderHUD` when `--hud` is
  set; keep `--json` and default (phase-briefing) paths intact.

### On-disk contracts
None. The HUD reads the same inputs the manifest already reads; it writes nothing.

### External interfaces
- `RenderHUD(Manifest) string`. Consumed only by the `context` CLI handler.

## 5. Invariants preserved (ADR-8)
Render is a pure function of on-disk-derived `Manifest`; no LLM in the render path; zero new
dependencies; no state mutation.

## 6. Cross-domain dependencies
- Depends on: Spec 08 (`Manifest`, estimator, budget), Spec 02 (spec `mode`), Spec 10 (CLI
  flag plumbing).
- Consumed by: operators; optionally referenced by Spec 11 reporting for a task-context view.

## 7. Risks & open questions
- **Risk:** HUD estimation drifts from the gate's budget estimation. → resolved by RH.2/RH.3 —
  one estimator, and a test asserting `--hud` totals equal `--json` totals.
- **Open:** exact column set (path, bytes, tokens, %-of-budget?). Proposed: path · bytes ·
  tokens · total; budget-% only if `SPECD_MAX_CONTEXT_TOKENS`/config budget is set.
