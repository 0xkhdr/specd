# Spec — Extract `internal/context` (contextpkg) from `internal/core`

Branch: `restructure/core-split`. Source of truth for scope: `RESTRUCTURE_NEXT.md`.
This spec refines that plan against the actual code and resolves one import-cycle
hazard the plan did not account for (see Design § Cycle resolution).

---

## Requirements

### Requirement 1 — Pure leaf estimator has its own home
The token estimator is zero-coupling and must move first to prove the package
boundary.

- WHEN `context_estimate.go` is moved to `internal/context`, THEN `EstimateTokens`
  and `EstimateTokensString` SHALL live in package `contextpkg` and produce
  byte-identical results to the pre-move heuristic (`ceil(len/4)`).
- THE estimator SHALL remain a pure byte heuristic; the system SHALL NOT introduce
  a real tokenizer or any LLM dependency (No-LLM-in-context invariant).

### Requirement 2 — Cross-cutting state/domain types live in a shared leaf
The engine reads value types that today live in core; both `core` and `contextpkg`
must depend on them without an import cycle.

- THE shared leaf package `internal/spec` (package `spec`) SHALL own `SpecStatus`
  (+ `Status*` consts), `Phase` (+ `Phase*` consts), `PhaseForStatus`, and
  `IsReadonlyRole` (+ its role set).
- WHEN the types are pushed down, THEN `internal/core` SHALL re-export them via Go
  type aliases and const re-declaration (`type SpecStatus = spec.SpecStatus`, etc.)
  so existing `core.SpecStatus` / `core.StatusRequirements` call sites compile
  unchanged.
- THE `internal/spec` package SHALL NOT import `internal/core` (it is a leaf).

### Requirement 3 — The pure engine moves into contextpkg
The deterministic "what to load" engine and its slicers are part of the engine,
not a dependency of it.

- WHEN the engine is moved, THEN `BuildContextManifest`, `ContextRequest`,
  `ContextMode` (+ consts), the budget math, the `ctxHint*` defaults, the wire
  types `MissionContextManifest` / `MissionContextItem`, and the slicers
  (`TaskSlice`, `CoveredRequirements`, `DesignSection`, `RecentMemory`) SHALL live
  in `contextpkg`.
- WHERE the slicers need local Markdown helpers (`splitLines`, `StripHTMLComments`,
  `taskRE`, `waveRE`, `reqHeaderNumRe`, `sliceH2Re`, `sliceHeadingRe`), `contextpkg`
  SHALL carry private copies; the originals in `core` (`ears.go`, `md.go`,
  `tasksparser.go`) SHALL remain untouched (an `internal/mdutil` leaf is deferred
  to backlog).
- `contextpkg` SHALL import only `internal/spec` (and stdlib); it SHALL NOT import
  `internal/core`.

### Requirement 4 — Mission adapter, IO, and validation stay in core
The mission-mode glue is core→engine wiring and host-IO; it must not be dragged
into the pure engine.

- THE adapter `BuildMissionContextManifest(mission PinkyMission, …)` SHALL stay in
  `internal/core`, because `PinkyMission` embeds `ACPAuthority` and cannot move to a
  pure leaf without dragging the ACP cluster.
- THE IO/boundary functions `SpecArtifactReader` / `specArtifactReader` (filesystem
  IO) and `HostContextBudgetFromEnv` (environment read) SHALL stay in `core`,
  preserving the engine-purity invariant (the engine never touches env or disk).
- THE validator `validateMissionContextManifest` SHALL stay in `core` and reference
  the moved wire types; the soft-ceiling/version bounds it needs SHALL be exported
  from `contextpkg` (`ManifestVersion`, `MinSoftCeiling`, `MaxSoftCeiling`).

### Requirement 5 — External callers rewired, coverage raised
- THE five external call sites (`internal/cmd/context.go`,
  `internal/cmd/dispatch.go`, `internal/mcp/tools_test.go`,
  `internal/testharness/spec_builder.go`,
  `internal/testharness/harness_extra_test.go`; plus
  `internal/schema/schema_test.go` if it references a moved type) SHALL compile and
  pass against the new package layout.
- WHEN the new packages land, THEN coverage thresholds for `internal/spec` and
  `internal/context` SHALL be set at or above the bar used for the prior three
  extractions (`schema`, `runner`, `pack`).

### Requirement 6 — Behavior is byte-for-byte preserved
- WHEN `ReadArtifact` is `nil`, THEN `BuildContextManifest` SHALL reproduce the
  pre-extraction manifest exactly (default `ctxHint*` hints and whole-file fallback
  modes unchanged).
- THE engine SHALL remain total: no IO, no panics, deterministic; the move SHALL
  NOT introduce a package-`init` or global mutable state.

---

## Design

### Overview
Carve the context engine out of the `internal/core` god-package in five small,
mechanical commits, mirroring the proven `schema`/`runner`/`pack` extractions. The
engine is already pure (all IO injected via `ReadArtifact`), so the only real work
is breaking two couplings: shared value types (type-push) and the mission adapter
seam (interface/ownership split).

### Architecture (target package graph)

```
internal/spec        (leaf: SpecStatus, Phase, PhaseForStatus, IsReadonlyRole)
      ▲      ▲
      │      └──────────────┐
internal/context            │   (contextpkg: engine, wire types, slicers, estimator)
  (imports spec only)       │
      ▲                     │
      │                     │
internal/core ──────────────┘   (PinkyMission, adapter, IO, validator;
                                 re-exports spec types via aliases)
```

Edges point "imports". No cycle: `core → context → spec`, and `core → spec`.

### Cycle resolution (the key decision, diverges from RESTRUCTURE_NEXT.md step 4)
`RESTRUCTURE_NEXT.md` step 4 proposes moving `BuildMissionContextManifest`,
`SpecArtifactReader`, and `HostContextBudgetFromEnv` into `contextpkg`. That cannot
be done as written:

- `PinkyMission` (pinky.go) embeds `MissionContextManifest` **and** `ACPAuthority`.
  `ACPAuthority` is core's ACP cluster (not yet extracted).
- If the adapter `BuildMissionContextManifest(mission PinkyMission, …)` moved to
  `contextpkg`, then `contextpkg` would import `core` for `PinkyMission`, while
  `core` imports `contextpkg` for the `MissionContextManifest` field → **import
  cycle**.

Resolution: the **pure** pieces move (engine, wire types, slicers, estimator); the
**glue and IO** stay in core. `core.PinkyMission.ContextManifest` becomes type
`contextpkg.MissionContextManifest` (edge `core → context`, no back-edge). This also
keeps the engine-purity invariant honest — env/disk readers belong on core's
boundary, not in the pure engine.

### Components and interfaces

| Lands in | Symbols |
|----------|---------|
| `internal/spec` | `SpecStatus`+consts, `Phase`+consts, `PhaseForStatus`, `IsReadonlyRole`(+set, private `sliceToSet` copy) |
| `internal/context` (`contextpkg`) | `EstimateTokens`, `EstimateTokensString`; `ContextRequest`, `ContextMode`+consts, `BuildContextManifest`, `ctxHint*`, budget math; `MissionContextManifest`, `MissionContextItem`; `TaskSlice`, `CoveredRequirements`, `DesignSection`, `RecentMemory`; private md helpers; exported bounds `ManifestVersion`/`MinSoftCeiling`/`MaxSoftCeiling` |
| `internal/core` (stays) | `PinkyMission`; `BuildMissionContextManifest`; `SpecArtifactReader`/`specArtifactReader`; `HostContextBudgetFromEnv`; `validateMissionContextManifest` (+`missionContextKindSet`/`missionContextModeSet`); type-alias re-exports of spec types |

`ContextRequest.Status` is typed `SpecStatus` → resolves to `spec.SpecStatus` (via
the contextpkg import). The adapter maps `PinkyMission` fields into
`contextpkg.ContextRequest` and returns `contextpkg.MissionContextManifest`.

### Data models
Wire types are unchanged on the wire — only their package moves. JSON tags,
`omitempty` placement, and version (`1`) are preserved so `PinkyMission` JSON and
the manifest JSON are byte-identical. The additive `EstimatedTokens`/`Budget`
fields keep their `omitempty` semantics (zero ⇒ pre-feature bytes).

### Error handling
No new error paths. `validateMissionContextManifest` keeps every existing message
string verbatim (tests in `pinky_context_validate_cov_test.go` assert on bounds).
Exported bound constants must equal the current literals (`version=1`,
`min=1000`, `max=200000`).

### Verification strategy
- Per-commit: `go build ./... && go test ./...` green at each of the five steps.
- Move-tests-with-code: `context_*_test.go`, `pinky_context_validate_cov_test.go`
  split alongside their subjects; tests that assert wire bytes (e.g.
  `context_manifest_test.go`) act as the byte-for-byte guard for Req 6.
- Golden check: a `nil`-`ReadArtifact` manifest snapshot before vs. after must be
  identical (Req 6) — capture from an existing test or add one in step 1.
- Coverage: raise thresholds on `internal/spec` and `internal/context` (Req 5)
  using whatever mechanism the prior extractions used (`coverpkg`/Makefile/CI).

### Risks and open questions
- **Package name vs. stdlib `context`.** Directory `internal/context`, package name
  `contextpkg` (importers alias as needed). Confirm no lint rule forbids the dir
  name; if it does, fall back to `internal/contextengine`.
- **Leaf name `internal/spec` vs. existing `internal/schema`.** Risk of confusion.
  Alternative `internal/state`. Recommend `internal/spec`; flag at step 2 if the
  team prefers `state`.
- **Type-alias re-export longevity.** Aliases in `core` keep the blast radius tiny
  now; a later pass can migrate call sites to import `spec` directly and drop the
  aliases. Out of scope here.
- **`schema_test.go` coupling.** Grep flagged it as a possible caller; confirm
  whether it references a moved type before assuming a no-op (step 5).
