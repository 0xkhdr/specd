# Spec 01 — Product & Philosophy Core

> **Authoring order:** 1 / 12 · **Critical path:** yes (`01 → 10 → 02 → 05 → 03 → 08 → 09`)
> **Sources:** `fresh-start/01-product-philosophy-core.md`, `The_New_SDLC_With_Vibe_Coding.pdf` pp.26–34
> **ADRs:** ADR-0, ADR-7, ADR-8 (`fresh-start/00-decisions.md`)
> **Reference:** `reference/README.md`, `reference/docs/concepts.md`, `reference/docs/validation-gates.md`

This domain owns the keep/cut line for every other spec. It defines what `specd` *is* — a
harness (deterministic scaffold of tools + sandboxes + orchestration), not a framework — and
turns the eight principles (P1–P8) and the paper's seven harness components into a standing,
enforceable charter.

---

## 1. Purpose & principles

- **Principles owned:** P1 (The Foundational Split — agent creates; harness enforces), P8
  (Steering as Constitution). Serves all eight.
- **Paper concept:** the harness as the unit of engineering (pp.26–34). Thesis: *"Most agent
  failures, examined honestly, are configuration failures"* (p.30). specd makes the paper's
  "80% problem" safely delegable by making every state change evidence-gated and every report
  deterministic.
- **Charter test for every retained feature:** *does this feature enforce the plan, or try to
  author it?* If it authors, it is agent work, not harness work (P1) — it is CUT.

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| Eight principles as invariants | **KEEP** | They are the product. `reference/docs/concepts.md` |
| Zero-dep, single static binary, git-native default | **KEEP** | P1; auditable + portable. `reference/go.mod`, `reference/docs/contributor-guide.md` inv#1 |
| 29-command surface | **REDESIGN → 16 verbs** | `fresh-start/00-scope-triage.md`; ADR-0 corrects the count |
| Postgres/Redis backends | **CUT** (optional build tag only) | Contradict zero-dep value. ADR-8, ADR-9 |
| "harness not framework" positioning | **KEEP** + make it the acceptance test | Charter gate below |
| `conductor`/`orchestrator` distinction | **KEEP** as first-class `mode` (designed in Spec 02) | Paper p.31; ADR-7 |

**Minimal accurate surface (the whole product):** (a) a spec lifecycle on disk, (b) a
deterministic gate engine, (c) an evidence ledger, (d) a context engine, (e) an
agent-agnostic integration floor, (f) an *opt-in* orchestration tier. Nothing else is core.

## 3. Requirements (EARS)

- **R1.1** When a feature is proposed for the core binary, the system shall require it to map
  to exactly one of the seven harness components and at least one of the eight principles, or
  be rejected.
- **R1.2** The system shall build as a single statically linked binary with zero runtime Go
  module dependencies (`go.mod` has no `require` block).
- **R1.3** When built with default tags, the system shall use only the git-native state
  backend and shall not link any external database driver.
- **R1.4** The system shall treat all agent-authored artifacts (`requirements.md`,
  `design.md`, `tasks.md`) as untrusted input and enforce, never author, their content.
- **R1.5** When a user runs the binary with no arguments, the system shall print the 16-verb
  command surface and exit `0`.
- **R1.6** The system shall keep every human-facing report a pure projection of `state.json`
  with no model invocation in its code path.

## 4. Design

### Module boundaries
- `internal/core` — domain logic + on-disk contracts.
- `internal/context` — context engine, kept separate to avoid a `core→context→core` cycle
  (ADR-0).
- `internal/cli` — zero-dep arg parser.
- `internal/cmd` — one file per verb.
- `internal/integration` — host adapters.
- `internal/orchestration` — opt-in tier, compiled always, inert unless enabled.

### Key mechanism — the harness charter
- `docs/charter.md` maps every shipped verb to exactly one of the seven harness components
  (instructions / tools / sandboxes / orchestration / guardrails / observability / context)
  and at least one principle. Unmapped code cannot merge.
- The charter is **wired as a CI lint over the registry**, not a doc convention — this
  operationalizes the subtractive bias as a standing gate (mitigates the "rubber stamp" risk).
- No new types are introduced here; enforcement rides on the registry↔help single-source
  guard (`TestRegistryMatchesHelp`, Spec 10).

### External interfaces
- The 16-verb CLI surface; the MCP tool surface (Spec 07); the `.specd/` on-disk layout
  (Spec 02).

## 5. Invariants preserved (ADR-8)
Zero-dep; embedded templates via the single `go:embed` in `embed.go`; atomic writes; CAS on
`revision`; reentrant per-spec lock; `ParseTasks` byte round-trip; determinism (no LLM in any
decision/gate/render path).

## 6. Cross-domain dependencies
This domain constrains all others. Its `mode` field is designed in Spec 02; its component map
is exercised by Specs 03/05/08/09. Immediately unblocks Spec 10.

## 7. Risks & open questions
- **Risk:** the charter becomes a rubber stamp. → wire it as a registry lint in CI.
- **Open (resolved by ADR-7):** `conductor` (real-time) mode needs no dedicated code — it is
  "the same core with orchestration disabled" (`mode: simple`).
