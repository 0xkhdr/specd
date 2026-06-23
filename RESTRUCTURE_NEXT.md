# Restructure — Next Improvements

Status as of 2026-06-24, branch `restructure/core-split`.

## Where we are

`internal/core` was a single god-package. Three leaf packages have been carved out, each a clean extraction with its own tests and raised coverage thresholds:

| Commit    | Package              | What moved                                  |
|-----------|----------------------|---------------------------------------------|
| `8f4ca44` | `internal/schema`    | Schema definitions + validation             |
| `0278e70` | `internal/runner`    | Sandbox/process runner                      |
| `bc543f1` | `internal/pack`      | Pack resolve/apply                          |

`internal/core` still holds ~73 non-test `.go` files. The next coherent slice to lift out is the **context engine**.

---

## Next: `internal/context` (contextpkg)

The context engine is the deterministic "what to load" contract — `BuildContextManifest` and friends. It is a strong extraction candidate: it is pure (all IO injected via `ReadArtifact`), well-tested, and conceptually self-contained. But it cannot be moved as-is because it reaches back into core types and helpers. That reach is exactly the **type-push + interface-seam** work.

### Files in scope

| File                       | Lines | Coupling to core                                                                 |
|----------------------------|------:|----------------------------------------------------------------------------------|
| `context_estimate.go`      |    50 | **None.** Pure leaf (`EstimateTokens`, `EstimateTokensString`).                  |
| `context_slice.go`         |   178 | Local helpers: `splitLines`, `StripHTMLComments`, regexes (`taskRE`, `waveRE`, `reqHeaderNumRe`). |
| `context_manifest.go`      |   264 | Types `SpecStatus`, `Phase`, `PinkyMission`; funcs `PhaseForStatus`, `IsReadonlyRole`. |
| `pinky_context.go`         |   159 | `PinkyMission`, `ReadArtifact`; validators `validateACPText/Paths`, `ACPMaxListItems`, `sliceToSet`. |

### The two coupling problems

**1. Type-push** — the engine reads value types that live in core:
- `SpecStatus`, `Phase` (`state.go`), `PhaseForStatus` (`phases.go`)
- `PinkyMission` (`pinky.go`)
- `IsReadonlyRole` (`tasksparser.go`)

These are cross-cutting state/domain types. To let `contextpkg` and `core` both depend on them without an import cycle, push them down into a leaf types package (proposed `internal/spec` or `internal/state`) that both import. `MissionContextManifest` / `MissionContextItem` are wire types and should travel **with** the engine into `contextpkg`.

**2. Interface-seam** — the engine already injects its only IO (`ReadArtifact func(name) (string, ok)`), which is the seam done right. The remaining inward calls to split here:
- **Slicers** (`TaskSlice`, `CoveredRequirements`, `DesignSection`) move *with* the engine — they are part of it, not a dependency.
- **Validation** (`validateMissionContextManifest` in `pinky_context.go`) is *pinky/ACP* domain, not engine domain. It should **stay in core** and call the moved wire types, rather than dragging `validateACP*` into `contextpkg`. Split the file: pure builder + adapter → `contextpkg`; validation stays.
- **Local text helpers** (`splitLines`, `StripHTMLComments`, regexes) are tiny and unexported — copy them into `contextpkg` (or a `internal/mdutil` leaf if they are shared more widely; check `ears.go`/`tasksparser.go` reuse first).

### External callers (small blast radius)

Only five files outside core touch this API — confirms the surface is narrow:
- `internal/cmd/context.go`, `internal/cmd/dispatch.go`
- `internal/mcp/tools_test.go`
- `internal/testharness/spec_builder.go`, `internal/testharness/harness_extra_test.go`

### Proposed sequence

1. **Leaf first** — move `context_estimate.go` to `internal/context`. Zero coupling, proves the package boundary, gives `EstimateTokens` a home. (`EstimateTokensString` is the one engine call that's trivially relocatable.)
2. **Type-push** — extract `SpecStatus`, `Phase`, `PhaseForStatus`, `PinkyMission`, `IsReadonlyRole` into the shared leaf types package. Land this as its own commit; it touches many files but is mechanical.
3. **Engine move** — move `context_manifest.go` + `context_slice.go` (builder, source-artifact assembly, slicers, budget math) into `contextpkg`. Carry the local md helpers.
4. **Split the adapter** — `pinky_context.go`: `BuildMissionContextManifest` + `SpecArtifactReader` + `HostContextBudgetFromEnv` go to `contextpkg`; `validateMissionContextManifest` stays in core, now importing the moved wire types.
5. **Rewire callers** — update the five external callers + tests; raise coverage thresholds on the new package as the prior three extractions did.

### Invariants to preserve

- **No-LLM-in-core / no-LLM-in-context** — `EstimateTokens` stays a pure byte heuristic. Do not pull in a real tokenizer.
- **Byte-for-byte output** — a `nil` `ReadArtifact` request must reproduce the pre-extraction manifest exactly. The default token hints (`ctxHint*`) and whole-file fallbacks encode this; keep their values.
- **Engine totality** — no IO, no panics, deterministic. The move must not introduce a package-init or global-state dependency.

---

## After contextpkg (backlog, unordered)

- **`backend_*` family** — `backend_git.go`, `backend_redis.go`, `backend_postgres.go` look like a storage seam already; candidate for `internal/store` behind one interface.
- **`acp_*` cluster** — large, cohesive (archive/cursor/lease/store/validate). Likely the biggest remaining package and the highest-value split.
- **`orchestration_*` + `program_session.go`** — engine vs. authoring boundary worth tracing.

These are noted only so the direction is recorded; each needs the same coupling audit before committing to a move.
