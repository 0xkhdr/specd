# 00 — Cross-cutting Decisions (ADRs)

Architecture decisions that span domains, plus factual corrections to Part A / the
brief discovered during the preflight sweep. Each ADR: context → decision →
consequences.

---

## ADR-0. Preflight corrections to Part A (facts, not choices)
- **Command count:** the real registered surface is **29 commands** (`internal/cmd/
  registry.go`), not the looser list in Part A. `security`, `dispatch`, `program`,
  `program_schedule`, `watch_webhook` are **flags/subcommands**, not top-level
  commands; `doc.go` implements no command; `promote` is registered but implemented in
  `eval.go` (`RunPromote`).
- **Parser name:** the tasks parser entry point is **`ParseTasks`**, not `ParseTasksMd`.
  The byte-round-trip invariant is real and preserved (`encodeAnnotationField`,
  `ApplyTaskAnnotation` single-line rewrite, `StripHTMLComments` line-number stability).
- **Context engine placement:** `BuildContextManifest` lives in **`internal/context`**,
  not `internal/core`. Core exposes only the adapter `BuildMissionContextManifest`
  (`pinky_context.go`). Moving the builder into core creates a documented
  `core→context→core` import cycle. → **Consequence:** domain 08's "make context
  first-class/central" means *its own package*, not folding into core.
- **State backends:** Postgres/Redis backends exist behind build tags
  (`specd_postgres`, `specd_redis`), contradicting the stated zero-dep/git-native value.

## ADR-1. Parser & on-disk plan format — keep bespoke Markdown, move annotations to state
- **Context:** `tasks.md` is agent-authored, human-reviewed, git-diffable (P2), but today
  carries machine annotations inline via a subtle lossless encoding.
- **Decision:** **KEEP** agent-authored Markdown as the source of truth and the hard
  byte-round-trip invariant; **REDESIGN** the annotation channel — machine state
  (status, verify-ref, telemetry) lives in `state.json` (already machine truth), leaving
  `tasks.md` as clean Markdown whose only load-bearing content is checkboxes + metadata
  keys. Do **not** switch to JSON/YAML task files (would move authorship away from the
  agent, violating P1/P2).
- **Consequences:** smaller escape/encoding surface; the Sync gate (Gate 6) still enforces
  checkbox↔state agreement; round-trip becomes easier to prove (property test). (Domain 04.)

## ADR-2. Config format — YAML subset, no legacy JSON, no migrate
- **Context:** config is YAML-only as of v0.2.0 (`parseSimpleYAML`, fail-loud), with a
  `config_migrate.go` path for legacy `.json`.
- **Decision:** **KEEP** the hand-rolled YAML subset loader (zero-dep, deterministic,
  fail-loud). **CUT** legacy `config.json` runtime handling and the `migrate` command from
  the MVP — a fresh tree has no legacy schema.
- **Consequences:** new config fields are added in loader + validator only (no migration
  renderer, one fewer of the current four edit sites). `state.json` schema resets to
  `SchemaVersion: 1`. Reintroduce migration only when the v1 schema first evolves. (Domains 02, 10.)

## ADR-3. Orchestration surface — one control plane, opt-in, aggressively minimal
- **Context:** orchestration/ACP/program/pinky/support ≈ **350K of `internal/core`**
  across brain (4 cmd files) + pinky + conductor + orchestrate + a full program tier.
- **Decision:** collapse into a **single `internal/orchestration` package** with a **pure
  `Decide(Snapshot) → Decision`** core and thin file-backed IO. Ship `brain
  {start|step|run|status|approve|cancel|resume}` + `pinky {claim|heartbeat|report|inbox|
  checkpoint}` only. **CUT** `orchestrate`; **DEFER** `conductor` (analytics), the program
  (multi-spec) tier, and model-tier `routing`. Compiled always, inert unless
  `orchestration.enabled` (fail-closed).
- **Consequences:** the biggest subtraction in the tree. Determinism is *provable* (pure
  Decide, tested no-LLM). Multi-spec is a documented later lift, not a parallel plane. (Domain 09.)

## ADR-4. Gate engine — pluggable interface, not hardcoded branches
- **Context:** opt-in gates 8–13 are wired as config-keyed conditional branches; adding a
  gate touches four files.
- **Decision:** define `Gate{ Name(); Run(CheckCtx) []Finding }` + an ordered `Registry`;
  uniform `off|warn|error` severity in one config block. Core 7 gates register
  unconditionally; opt-in gates register when configured. Gate bodies stay pure (no IO).
- **Consequences:** one registration point; byte-identical output when opt-ins off is
  preserved and testable; the deferred flywheel (eval/review/security/context-budget)
  re-enters *only* here. (Domains 03, 08, 12.)

## ADR-5. Flywheel tiering — subtract with a re-entry contract
- **Context:** 8 flywheel commands (~90K core), mostly off by default, none on the MVP
  critical path.
- **Decision:** v1 ships **no flywheel commands**; only the `security` **gate module**
  ships (via ADR-4). All deferred features re-enter through exactly two seams: the `Gate`
  interface (ADR-4) and the `state.records` extension map (ADR-6). Evidence shapes
  (`DeployApproval`, `EvalSummary`, inventory waivers) are documented, not coded. **CUT**
  `submit`, `migrate`.
- **Consequences:** dramatic surface reduction with a stable contract for v2; no flywheel
  feature may add a core schema field or a `check` branch. (Domain 12.)

## ADR-6. State schema — thin core + `records` extension map
- **Context:** `State` carries a record struct for every flywheel feature, bloating the
  core schema even when those features are absent.
- **Decision:** core `State` holds only lifecycle fields; optional/plugin evidence lives in
  `State.Records map[string]json.RawMessage`. Core validates only that entries are valid
  JSON; each plugin owns its key's schema.
- **Consequences:** deferring the flywheel removes ~9 record structs from core; plugins
  attach evidence without a schema bump. (Domains 02, 12.)

## ADR-7. Execution mode — two states aligned to the paper (conductor/orchestrator)
- **Context:** three modes exist (`Simple/Orchestrated/Conductor`) but `Conductor` names a
  rejection-analytics mode, colliding with the paper's conductor/orchestrator axis (p.31).
- **Decision:** `mode` is a first-class enum with two real states: **`simple`** (paper's
  *conductor*: human-in-the-loop, no worker delegation) and **`orchestrated`** (paper's
  *orchestrator*: async delegation). Set at `new --mode`, default `simple`, changeable only
  via an auditable `approve --mode` transition. Drop the analytics `Conductor` mode (defers
  with the `conductor` command).
- **Consequences:** one binary serves the real-time IDE user and the async delegator;
  orchestration eligibility keys off `mode: orchestrated`. (Domains 01, 02, 09.)

## ADR-8. Hard invariants carried forward unchanged
The following are **preserved verbatim** (no ADR changes them); any future change requires a
new ADR:
- **Atomic writes** — temp → fsync → chmod 0644 → rename; partial write never replaces.
- **CAS on `revision`** — inside `WithSpecLock`; test-panics if unlocked.
- **Reentrant per-spec advisory lock** — stale reclaim, goroutine-id reentrancy, timeout.
- **`ParseTasks` byte round-trip** — `Serialize(Parse(x)) == x` (now property-tested).
- **Embedded templates** — the single `go:embed` in `embed.go`; no disk-relative reads.
- **Zero runtime dependencies** — `go.mod` has no `require`; git-native default backend.
- **Evidence integrity** — no task/spec completes without a passing verify record; worker
  reports validated against records.
- **Determinism** — no LLM/network in any harness decision, gate, estimate, or render path.

## ADR-9. Missing-domain check
The B.3 table's 12 domains cover the retained surface completely. No new domain was added.
Two items that might look like missing domains are deliberately *folded*, not omitted:
- **State backends** — covered inside domain 10 (foundations); decision = git-only,
  Postgres/Redis **CUT** to optional build tags.
- **Program (multi-spec) tier** — covered inside domain 09; decision = **DEFER** wholesale.
