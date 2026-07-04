# ADR Log — Architecture Decision Records

> This is a condensed, single-file reference for all ADRs that govern the specd rebuild.
> The authoritative source is `PROJECT.md §4`; this file is the readable derivative for
> day-to-day reference. When PROJECT.md and this file conflict, PROJECT.md wins.
>
> **Format:** each ADR names its verdict (KEEP · SIMPLIFY · REDESIGN · CUT · DEFER),
> the problem it solved, and the binding constraints it imposes.

---

## ADR-0 — Preflight factual corrections

**Verdict:** Corrective (not a design choice — a fact-setting record)

**Problem:** Pre-rebuild analyses contained several false premises about v1.

**Binding corrections:**
- v1 had **29 registered commands**, not the number cited earlier.
- The parser entry point is **`ParseTasks`** (not `ParseTasksMd`).
- `BuildContextManifest` lives in **`internal/context`**, not `core`. Folding it into core
  creates a `core→context→core` import cycle. Core exposes only the adapter
  `BuildMissionContextManifest`. This split is enforced forever.
- v1's Postgres/Redis backends contradicted the zero-dep value and are CUT.

---

## ADR-1 — Parser & plan format

**Verdict:** KEEP agent-authored `tasks.md` · REDESIGN the annotation channel

**Problem:** v1 annotated task status inside `tasks.md`, mixing machine and human concerns.

**Decision:**
- `tasks.md` stays the source of truth for task authoring (P1/P2). It is **clean Markdown**;
  the only load-bearing content is the pipe-table rows with checkboxes.
- Machine state (status, verify-ref, telemetry) lives exclusively in **`state.json`** (machine truth).
- `Serialize(Parse(x)) == x` — the byte-round-trip invariant is property/fuzz tested.
- Single-line rewrite on status change; stable line numbers via length-preserving comment stripping.
- The Sync gate (gate 6) enforces checkbox ↔ `state.json` agreement.
- Never switch to JSON/YAML task files — that moves authorship away from agents/reviewers (P1/P2 violation).

---

## ADR-2 — Config

**Verdict:** KEEP hand-rolled YAML-subset loader · CUT legacy `config.json`

**Problem:** v1's config was complex, had two competing formats, and swallowed parse errors.

**Decision:**
- Config file: **`config.yml`** (was `project.yml` in early rebuild — an open finding, F10).
- Layered: global → project → env. Pure `LoadConfig(paths, env) → (Config, []Diagnostic)`.
- **Fail-loud**: parse error is a hard exit, never a silent default.
- `state.json` schema version resets to `SchemaVersion: 1`.
- CUT `config.json` handling and the `migrate` command.
- Migration reintroduced only when the v1 schema first evolves.

---

## ADR-3 — Orchestration surface

**Verdict:** REDESIGN (collapse v1's ~350K brain/pinky/conductor mass)

**Problem:** v1's orchestration was over-engineered: conductor, orchestrate, program tier,
model-tier routing — far beyond the paper's orchestrator concept.

**Decision:**
- Collapse to a **single `internal/orchestration` package** with a pure `Decide(Snapshot) → Decision` core.
- Ship only: `brain {start|step|run|status|approve|cancel|resume}`.
- Worker CLI (`pinky`): **DEFER and unship** — the verbs were dead surface; re-entry seam is
  one registration call when a real driver needs them.
- CUT `orchestrate` command.
- DEFER `conductor` (analytics), program (multi-spec) tier, model-tier routing.
- Compiled always; **inert unless `orchestration.enabled: true` in config**.
- **Fail-closed by default.** No policy can open the orchestration tier without explicit config.

---

## ADR-4 — Gate engine

**Verdict:** REDESIGN (pluggable registry)

**Problem:** v1's gate logic was hardcoded branches in `check.go`.

**Decision:**
- Pluggable interface: `Gate{ Name(); Run(CheckCtx) []Finding }` + ordered `Registry`.
- Uniform `off|warn|error` severity in one config block.
- **7 core gates** register unconditionally in `CoreRegistry()`.
- Opt-in gates (acceptance, scope, context-budget, security) register when configured or flagged.
- Gate bodies are **pure — no IO**. `CheckCtx` is the only input; no fs/net handles.
- Byte-identical `check` output when opt-ins are off is testable.
- Adding a gate = one `registry.Register` call, zero edits to the check runner.
- External custom gates: subprocess contract (stdin/stdout JSON, scrubbed env, bounded timeout).

---

## ADR-5 — Flywheel tiering (primary subtraction)

**Verdict:** CUT from v1 shipped surface · DEFER with documented re-entry seams

**Problem:** v1 accreted flywheel commands (eval, review, deploy, observe, ingest, harness)
that are valuable but premature given the minimal-core north star.

**Decision:**
- v1 ships **no flywheel commands**.
- **One exception**: the `security` gate module ships (stdlib-only, off by default).
- CUT: `submit`, `migrate`.
- DEFER: `eval`, `review`, `deploy`, `observe`, `ingest`, `harness`, the program tier.
- Re-entry is **only** via two seams already in the tree:
  1. The `Gate` interface (`internal/core/gates/registry.go`).
  2. `State.Records map[string]json.RawMessage` (`internal/core/state.go`).
- Restoring flywheel features to fix the verb-count finding (F7) is **explicitly out of bounds**.
- See `docs/deferred-flywheel.md` for the detailed evidence shapes and seam documentation.

---

## ADR-6 — State schema

**Verdict:** REDESIGN (minimal core + extension map)

**Problem:** v1's State had fields for every feature, creating coupling between core and plugins.

**Decision:**
- Core `State` holds only lifecycle fields: `SchemaVersion, Slug, Mode, Status, Phase, Revision`.
- Optional / plugin evidence lives in `State.Records map[string]json.RawMessage`.
- Core validates only that entries are valid JSON; each plugin owns its key's schema.
- `TaskStatus map[string]TaskRunStatus` in state is machine truth for per-task run status (ADR-1).
- `Extra map[string]json.RawMessage` for forward-compat extension.

---

## ADR-7 — Execution mode

**Verdict:** KEEP as first-class enum · REDESIGN definition

**Problem:** v1's mode was loosely typed and lacked a clean simple/orchestrated split.

**Decision:**
- Mode is a first-class enum with exactly **two states** aligned to the paper:
  - **`simple`** (conductor: human-in-the-loop, no worker delegation).
  - **`orchestrated`** (orchestrator: async delegation).
- Set at `new --mode`, default `simple`.
- Changeable only via an auditable `approve --mode` transition.
- Orchestration eligibility keys off `mode: orchestrated`.
- v1's analytics-flavored `Conductor` mode is dropped (deferred with the `conductor` command).

> **Open finding F5:** ADR-7 mode enum is partially unimplemented in the current tree
> (`ModeDefault`/`ModeAgent` exist but not `simple`/`orchestrated`; no `--mode` flag;
> orchestration eligibility does not key off mode). Wave P1 closes this.

---

## ADR-8 — Hard invariants

**Verdict:** ENFORCE verbatim — any change requires a new recorded ADR

These invariants are non-negotiable and must hold at every commit:

| Invariant | Mechanism |
|---|---|
| **Atomic writes** | `AtomicWrite`: MkdirAll → CreateTemp → write → fsync → chmod 0644 → rename |
| **CAS on revision** | `SaveStateCAS` inside `WithSpecLock`; test builds panic if `SaveState` runs unlocked |
| **Reentrant advisory lock** | `WithSpecLock`: cross-process `O_CREATE|O_EXCL` `.lock`, stale reclaim at 30s, in-process per-path mutex, goroutine-id reentrancy, 5s acquire timeout |
| **Parser byte round-trip** | `Serialize(Parse(x)) == x`; property/fuzz tested; single-line rewrite on status change |
| **Embedded templates** | Single `go:embed` in `embed_templates/`; no disk-relative reads at runtime |
| **Zero runtime dependencies** | `go.mod` has no `require`; git-native default backend; single static binary |
| **Evidence integrity** | No task completes without a passing verify record (exit code + real git HEAD) |
| **Determinism** | Gates, DAG, reports are pure functions of on-disk state; no LLM or network in any decision/render path |

---

## ADR-9 — Domain completeness

**Verdict:** 12 domains cover the retained surface

**Decision:**
- State backends fold into domain 10 (git-only; Postgres/Redis CUT to optional build tags at most).
- The program tier folds into domain 09 (DEFER wholesale).
- No domain is "owned" by a flywheel command.

---

## ADR-10 — Scaffold surface

**Verdict:** SIMPLIFY init

**Problem:** v1's `init` was 803 LOC + 15K `initplan` — massively over-built.

**Decision:**
- `init` scaffolds exactly:
  - `.specd/roles/{scout,craftsman,validator,auditor}.md` — **exactly four roles**.
  - `.specd/steering/{reasoning,workflow,product,tech,structure,memory}.md`.
  - Marker-merged `AGENTS.md` (only the marker-delimited section replaced).
- `scribe` role: removed (invented without provenance); `auditor` role: restored.
- `new` scaffolds: `requirements.md` + `design.md` + `tasks.md` + `state.json` + `memory.md`.
  `design.md` is required so the design gate is reachable.
- CUT: brain/pinky/reviewer as roles; the `skills/` SKILL tree; `config.json`; 7-key task
  schema (`why`/`contract` dropped — skeleton is 6-key: `id/role/files/depends-on/verify/acceptance`).
- DEFERRED: `config.yml` seeding by init (Wave P5); pinky subagent prompts + runtime gitignore.

---

## ADR-11 — Standing regression (Wave W7)

**Verdict:** Close the concept↔functionality gap with deterministic scripts, not a one-time audit

**Problem:** Periodic manual audits drift and miss regressions between releases.

**Decision:**
- Three deterministic scripts under `scripts/` run at HEAD and are re-runnable on every push:
  1. `regress-all.sh` — re-runs **every** `verify:` in `review-specs/00..06/tasks.md` literally
     via `sh -c`; verdict from exit-code log only (W7 wave excluded to avoid self-recursion).
  2. `regress-lint.sh` — static smell audit:
     - Authoring `specs/` read where runtime reads `.specd/specs/` → G1.
     - Hollow existence-only verify → G4.
     - `files:`/verify target failing `test -e` → G3.
  3. `regress-domains.sh` — re-asserts each wave's owned invariant black-box against a freshly
     built binary in a throwaway tree (W0 honesty … W6 release), exiting on first violation.
- No LLM, no network in any verdict path — same determinism guardrail as the gates.
- A wave stays open until its live evidence exists; no surface is unowned.
