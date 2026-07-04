# Wave 4 — Finish the Gate Engine, Wake the Constitution

> **Order:** 5 / 7 (parallel with W3) · **Depends:** W1 · **Unblocks:** W5 (memory-verb decision)
> **Findings:** F8 (missing content gates), F9 (steering/memory inert)
> **Sources:** PROJECT.md §8 Wave P4, BUILD_REVIEW.md §5 Wave 4, specs/03 + 06 + 08, ADR-1/4
> **Files:** `internal/core/gates/{core,ears,approval,sync}.go`, `internal/context/manifest.go`

Spec 03 promises seven core gates; the registry today holds only structural checks —
`requirements.md` content is never validated, nothing checks approvals before task
waves, ADR-1's Sync gate is absent, and the design gate reads nothing. Meanwhile the
steering constitution and memory flywheel (paper P8, the *first* harness component)
exist as files nothing reads. This wave delivers the content gates and wires the
constitution into the context manifest.

## 1. Purpose & principles

- **Principles owned:** P8 (steering as constitution), P6 (approval gate), P2/P3.
- **Harness components:** guardrails (gates), instructions/context (steering + memory in manifest).

## 2. Requirements (EARS)

- **R4.1** When `check` runs, an EARS gate shall warn on `requirements.md` lines lacking
  the `When …, the system shall …` shape and shall error when the file is still the
  unedited scaffold stub (compare against the embedded template — the placeholder must
  not pass because the template text is EARS-shaped).
- **R4.2** When `check` runs, an approval gate shall error when tasks show progress
  (any non-pending status in `state.json`) while requirements or design are unapproved;
  and a design gate shall error at `approve <slug> design` when `design.md` sections are
  empty or the file is the unedited stub. A Sync gate (ADR-1 Gate 6) shall error when
  `tasks.md` checkboxes disagree with `state.json` task status.
- **R4.3** When the context manifest is built (all three surfaces: `specd context`,
  `next --dispatch`, worker brief — one engine, they cannot drift), it shall include
  `.specd/steering/*.md` and the spec + steering `memory.md` as manifest items
  (steering = static instructions mode; memory = `reference-if-needed`), bounded and
  counted against `SPECD_MAX_CONTEXT_TOKENS`; the manifest carries references + modes,
  never inlined content.
- **R4.4** When all new gates are off or green, `check` output shall remain
  byte-identical to pre-W4 output (ADR-4 parity, extended in `parity_test.go`).

## 3. Design

- **Gates via the registry only (ADR-4):** each gate = one registration call, zero edits
  to `check.go`. Gate bodies pure over `CheckCtx` — the stub-detection template bytes
  and approval state enter through `CheckCtx`, not file reads inside the gate.
- **Stub detection:** byte-compare against the embedded template (single source,
  ADR-10) after trimming trailing whitespace; no heuristics.
- **Severity floors:** approval + sync gates pin `error` (they guard P6/P3); EARS shape
  check is `warn` by default, stub check `error`; config can raise, not lower.
- **Manifest wiring (R4.3):** items appended in `BuildContextManifest`
  (`internal/context` — import-cycle rule ADR-0); token estimate via the existing pure
  heuristic; when over budget, memory items drop before steering (constitution wins),
  deterministically, with a manifest note.

## 4. Invariants preserved

- ADR-4: pure gate bodies, ordered registry, byte-identical output when opt-ins off.
- ADR-1: `tasks.md` stays clean Markdown; Sync gate enforces agreement, never rewrites.
- Domain 08: references + modes, never inlined content; pure token heuristic, no tokenizer.
