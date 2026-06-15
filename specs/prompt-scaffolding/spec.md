# spec.md — One-shot Scaffolding From a Prompt (`specd new --from`)

**Status:** proposed
**Source:** specd-report.html §8 idea **A2** (impact: high · effort: low · moat: low) · §9 north-star item **#4**
**Date:** 2026-06-16
**Scope:** extend `internal/cmd/new.go`; new gate-driven authoring brief. No LLM call inside the binary.

---

## 1. Objective

Cut time-to-first-green-check from ~20 min to ~2 by letting an agent generate a
draft `requirements.md` + `design.md` + `tasks.md` DAG from a natural-language
feature description in one pass, then iterate against the gates. The blank-stub
cold start is the highest-friction moment; the gates already define "valid", so
the binary should hand the agent a **gate-shaped authoring brief** seeded with
the prompt — and the agent fills it.

> **Hard invariant:** the binary does **not** call an LLM. `--from` does not
> generate prose itself; it (a) records the feature prompt into the spec, and
> (b) emits a deterministic, gate-aware authoring brief the agent fills. The
> "generation" is the agent's job; specd's job is to scaffold toward the gates
> and validate the result.

## 2. Context

- `specd new <slug>` (`internal/cmd/new.go`) scaffolds 6 artifact stubs via
  `internal/core/specfiles.go` (`Artifacts`).
- Validity is fully specified by the 7 gates (`internal/core/gates.go`): EARS
  forms, 7 design headers, 7-key task schema, acyclic DAG.
- `specd context` already builds an agent brief (`internal/cmd/context.go`,
  `buildBrief`) — the same idiom extends to an authoring brief.

## 3. Requirements (EARS)

- **R1 (H)** WHEN `specd new <slug> --from "<description>"` runs, the system
  SHALL scaffold the spec as today AND persist the verbatim description into the
  spec (e.g. `requirements.md` intro + a `prompt` field in `state.json`).
- **R2 (H)** THE SYSTEM SHALL emit a deterministic **authoring brief** that
  states, for each artifact, exactly what each gate requires: the 5 EARS forms
  with a worked template, the 7 mandatory design headers, and the 7-key task
  schema with a DAG example.
- **R3 (M)** WHERE `SPECD_JSON=1` is set, the authoring brief SHALL be emitted as
  structured JSON (one object per target artifact with its gate constraints) so
  an agent or MCP host consumes it programmatically.
- **R4 (M)** THE SYSTEM SHALL NOT generate requirement/design/task prose itself
  and SHALL NOT call any network or LLM endpoint.
- **R5 (M)** WHEN the agent has filled the artifacts, `specd check <slug>` SHALL
  validate them with the unmodified existing gate pipeline (the brief is
  generated *toward* those gates, so a faithfully-filled draft passes).
- **R6 (L)** WHERE `--from` is omitted, `specd new` SHALL behave exactly as
  today (backward compatible).

## 4. Design / approach

1. **Persist the prompt** — add an optional `Prompt string` to the spec's
   `state.json` (`internal/core/state.go`) and inject it into the
   `requirements.md` stub header.
2. **Brief generator** — `internal/core/authoring.go`: pure function returning a
   struct describing, per artifact, the gate constraints. Derive EARS forms from
   `internal/core/ears.go`, design headers from the design gate, task keys from
   the task-schema gate — single source of truth, no duplicated strings.
3. **Render** — text brief for humans, JSON under `SPECD_JSON=1`.
4. **Iterate** — the agent writes artifacts, runs `specd check`, fixes
   violations. No new gate, no behavior change to `check`.

## 5. Non-goals

- No LLM/codegen inside the binary; no network.
- No new validation gate (this rides the existing 7).
- No auto-approve — the human gate (`specd approve`) is unchanged.

## 6. Acceptance criteria

- `specd new x --from "..."` scaffolds, persists the prompt, and prints/streams
  the authoring brief; `--from` omitted ⇒ identical to today.
- The brief's stated constraints are derived from (not copied alongside) the
  real gate definitions — a test asserts they stay in sync.
- A draft authored faithfully against the brief passes `specd check`.
- No network/LLM; `make ci` green; binary stdlib-only.
