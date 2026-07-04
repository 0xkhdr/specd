# Spec R1 — Steering Memory & Promotion

> **Regression of:** implemented rebuild vs. MVP intent · **Kind:** missing MVP-KEEP subsystem
> **Sources:** `fresh-start/00-scope-triage.md` (memory KEEP, P8, "add/promote; absorbs `promote`"),
> `fresh-start/08-context-engineering.md`, `fresh-start/12-flywheel-triage-tier.md`
> **ADRs:** ADR-6 (records extension), ADR-8 (evidence/lock/atomic)
> **Reference:** `reference/internal/cmd/memory.go` (`RunMemory`),
> `reference/internal/core/commands.go` (memory meta, `PromotionThreshold`)

The learning flywheel's write path. Specs accumulate reusable patterns in a per-spec
`memory.md`; once a pattern recurs across enough specs it is **promoted** into the shared
`.specd/steering/memory.md` that every future spec's context loads. The store already ships
(scaffolded by `init`); the command that feeds and promotes into it was never built.

---

## 1. Purpose & principles
- **Principle owned:** P8 (Learning Flywheel / steering memory). Serves P3 (lean context — the
  steering memory is what gets loaded, so it must stay curated) and P7.
- **Paper concept:** *context engineering* — durable, promoted context outlives any single
  session; the harness curates it deterministically rather than an LLM "remembering."
- **Why it regressed:** the rebuild scaffolded the steering **store**
  (`internal/core/embed_templates/steering/memory.md`) but omitted the `memory` verb, so the
  store can only ever stay empty. The parity guard did not catch it because `memory` is absent
  from `core.Commands` entirely (not a no-op handler — a missing verb).

## 2. Verdicts (with citations)

| Capability | Verdict | Why / reference |
|---|---|---|
| `memory <slug> add` → append `## <key>` block to spec `memory.md` | **KEEP** | P8; `reference/internal/cmd/memory.go:66-100` |
| `memory <slug> promote --key` → lift block into `steering/memory.md` past threshold | **KEEP** | P8 flywheel; `reference/internal/cmd/memory.go:102-138` |
| Promotion threshold as a **pure count** of specs containing the block | **KEEP (hard)** | Determinism guardrail; `reference/…memory.go:114-124` — no LLM |
| `PromotionThreshold` config knob | **KEEP** → default `3` | `reference` `cfg.PromotionThreshold` |
| Reference standalone `promote` verb | **CUT** (absorbed) | `00-scope-triage`: "absorbs `promote`" → `memory … promote` |
| `--related a,b` → `[[a]], [[b]]` wikilinks | **SIMPLIFY-KEEP** | ties patterns into the reasoning graph; `reference/…memory.go:78-86` |
| Reference `--force` bypass of threshold | **KEEP** | audited override; `reference/…memory.go:122` |

**Minimal accurate surface:** one verb `memory <slug> <add|promote>`; module
`internal/core/memory.go` (pure block extract/append/count) + `internal/cmd/memory.go`
(handler); config field `PromotionThreshold`; on-disk `.specd/specs/<slug>/memory.md` and the
existing `.specd/steering/memory.md`.

## 3. Requirements (EARS)
- **RM.1** When `memory <slug> add` is invoked with `--key`, `--pattern`, `--body`, `--source`,
  and `--criticality`, the system shall append a `## <key>` block (pattern/detail/source/
  criticality/related fields) to `.specd/specs/<slug>/memory.md`, holding the per-spec lock for
  the read-modify-write.
- **RM.2** When any required `add` flag is missing, or `--criticality` is not one of
  `minor|important|critical`, the system shall refuse with a usage error and exit non-zero
  (no partial write).
- **RM.3** When `memory <slug> promote --key <k>` is invoked, the system shall count the specs
  whose `memory.md` contains a `## <k>` block and, only if that count ≥ `PromotionThreshold`
  (or `--force` is set), append the block plus a deterministic provenance line to
  `.specd/steering/memory.md`; otherwise it shall refuse and report the observed count and the
  threshold.
- **RM.4** The promotion decision shall be a pure function of on-disk `memory.md` files and
  config — no LLM, no network — satisfying the determinism guardrail.
- **RM.5** When a `promote --key` names a block absent from the spec's `memory.md`, the system
  shall fail loud with a gate error rather than promote an empty block.
- **RM.6** When `--related a,b` is supplied, the system shall render `[[a]], [[b]]`; when
  absent, the related field shall be `—`.
- **RM.7** The provenance line shall render its date from the injectable `Clock` (UTC), so
  promotion output is byte-deterministic under test.
- **RM.8** The system shall register `memory` in `core.Commands` with a non-nil handler and
  fail closed on an unknown subcommand, keeping `TestEveryCommandHasHandler` green.
- **RM.9** When `new <slug>` scaffolds a spec, the system shall create an empty `memory.md`
  artifact so `add`/`promote` have a stable target.

## 4. Design

### Module boundaries
- `internal/core/memory.go` — pure helpers: `ExtractMemBlock(text, key) string`,
  `RenderMemBlock(fields) string`, `CountSpecsWithBlock(root, key) int`. No I/O side effects
  beyond reads; deterministic.
- `internal/cmd/memory.go` — `runMemory(root, args, flags)` handler: arg parse → lock →
  append/promote → human/`--json` output. Mirrors `reference/internal/cmd/memory.go` structure,
  re-cased to the new codebase's handler signature (`registry.go`).
- `internal/core/paths.go` — add `SpecMemoryPath(root, slug)`, `SteeringMemoryPath(root)`,
  and `ListSpecs(root)` (enumerate `.specd/specs/*/`). The reference names (`ArtifactPath`,
  `SteeringDir`, `ListSpecs`) do not exist post-consolidation; add the thin equivalents here.
- `internal/core/config_loader.go` / `config_validate.go` — add `PromotionThreshold int`
  (default `3`, validated `>= 1`).

### On-disk contracts
- `.specd/specs/<slug>/memory.md` — append-only `## <key>` blocks; created empty by `new`.
- `.specd/steering/memory.md` — existing shared store; `promote` appends a block + a
  `**Promoted:** from spec '<slug>' on <date> (seen in <n> spec(s))` provenance line.
- Block grammar matches reference: `## <key>` then `**Pattern:** / **Detail:** / **Source:** /
  **Criticality:** / **Related:**` lines; `ExtractMemBlock` reads to the next `## ` heading.

### External interfaces
- `runMemory` (CLI). `CountSpecsWithBlock`, `ExtractMemBlock` (reused by Spec 12 flywheel and
  by `check` if a future gate wants a "promotable pattern" signal).

## 5. Invariants preserved (ADR-8)
Per-spec advisory lock wraps every `add`/`promote` read-modify-write; append via the atomic
file primitive; threshold is a pure count (no LLM); injectable `Clock` keeps output
deterministic; zero new dependencies.

## 6. Cross-domain dependencies
- Depends on: Spec 10 (config, paths, `Clock`, lock), Spec 02 (spec dirs, `new` scaffold),
  Spec 08 (steering store), Spec 12 (defines the flywheel this feeds).
- Consumed by: Spec 12 (promotion is the flywheel tier's core move), Spec 08 (promoted memory
  is loaded context).

## 7. Risks & open questions
- **Risk:** `memory.md` becomes an unbounded junk drawer. → `add` is human-invoked and
  criticality-tagged; promotion is threshold-gated; no auto-write.
- **Risk:** provenance non-determinism from wall-clock. → resolved via injectable `Clock`
  (RM.7), matching the rest of the codebase.
- **Open:** default `PromotionThreshold` (reference implies a small integer). Proposed `3`;
  reviewer may set `2`.
