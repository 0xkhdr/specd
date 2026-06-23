# Tasks — Extract internal/context (contextpkg)

<!--
Waves = commits. One commit per wave; build+test green before flipping to next.
Metadata keys: why, role, files, contract, acceptance, verify, depends, requirements.
-->

## Wave 1
- [ ] T1 — Move estimator to internal/context
  - why: zero-coupling leaf proves the package boundary first (Req 1)
  - role: builder
  - files: internal/core/context_estimate.go, internal/context/estimate.go, internal/context/estimate_test.go
  - contract: Create internal/context/estimate.go in package contextpkg holding EstimateTokens + EstimateTokensString verbatim. Move the estimator test alongside. Update the lone in-engine caller (context_manifest.go:120-122 EstimateTokensString) to use contextpkg once it moves in Wave 3 — for now keep EstimateTokens reachable from core via a thin alias if any core test needs it. Do NOT change the ceil(len/4) math or add a tokenizer.
  - acceptance: package contextpkg exists; EstimateTokens/EstimateTokensString produce identical output to pre-move; no LLM/tokenizer import
  - verify: go build ./... && go test ./internal/context/...
  - depends: —
  - requirements: 1, 6

## Wave 2
- [ ] T2 — Push shared state/domain types into internal/spec
  - why: contextpkg and core must both depend on these enums without a cycle (Req 2)
  - role: builder
  - files: internal/spec/status.go, internal/spec/phase.go, internal/spec/role.go, internal/core/state.go, internal/core/phases.go, internal/core/tasksparser.go
  - contract: Create package spec with SpecStatus(+Status* consts), Phase(+Phase* consts), PhaseForStatus, IsReadonlyRole(+role set, private sliceToSet copy). In core, replace the definitions with type aliases + const re-declarations (type SpecStatus = spec.SpecStatus; const StatusRequirements = spec.StatusRequirements; same for Phase; var PhaseForStatus = spec.PhaseForStatus or thin wrapper; func IsReadonlyRole = spec.IsReadonlyRole). spec must NOT import core. Leave all other phases.go logic (PhaseReadiness, PlanningAdvance, DesignGate) in core.
  - acceptance: core.SpecStatus/core.StatusRequirements/core.Phase/core.PhaseForStatus/core.IsReadonlyRole all still resolve; spec is a leaf (no core import); every existing core caller compiles unchanged
  - verify: go build ./... && go test ./...
  - depends: T1
  - requirements: 2

## Wave 3
- [ ] T3 — Move the pure engine + slicers + wire types into contextpkg
  - why: the engine and its slicers are part of the engine, not a core dependency (Req 3)
  - role: builder
  - files: internal/context/manifest.go, internal/context/slice.go, internal/context/manifest_test.go, internal/context/slice_test.go, internal/core/context_manifest.go, internal/core/context_slice.go, internal/core/pinky_context.go
  - contract: Move BuildContextManifest, ContextRequest, ContextMode(+consts), budget math, ctxHint* defaults (context_manifest.go) and TaskSlice/CoveredRequirements/DesignSection/RecentMemory + trimTrailingBlank + sliceH2Re/sliceHeadingRe (context_slice.go) into contextpkg. Move MissionContextManifest + MissionContextItem wire types and the version/ceiling constants out of pinky_context.go into contextpkg; export bounds as ManifestVersion/MinSoftCeiling/MaxSoftCeiling. Carry PRIVATE copies of splitLines, StripHTMLComments, taskRE, waveRE, reqHeaderNumRe into contextpkg; leave core/ears.go, core/md.go, core/tasksparser.go originals untouched. ContextRequest.Status uses spec.SpecStatus; engine calls spec.PhaseForStatus/spec.IsReadonlyRole. contextpkg imports spec + stdlib only — never core. Move the engine/slicer tests with their code.
  - acceptance: contextpkg holds the engine, wire types, slicers; no init() or global mutable state; contextpkg does not import core
  - verify: go build ./... && go test ./internal/context/...
  - depends: T2
  - requirements: 3, 6

- [ ] T4 — Keep adapter, IO, and validator in core; rewire to contextpkg
  - why: PinkyMission embeds ACPAuthority so the adapter cannot move without a cycle (Req 4)
  - role: builder
  - files: internal/core/pinky_context.go, internal/core/pinky.go
  - contract: Keep BuildMissionContextManifest, SpecArtifactReader/specArtifactReader, HostContextBudgetFromEnv, and validateMissionContextManifest (+ missionContextKindSet/missionContextModeSet) in core. Repoint PinkyMission.ContextManifest to contextpkg.MissionContextManifest. Rewrite the adapter to build contextpkg.ContextRequest and return contextpkg.MissionContextManifest. Rewrite validateMissionContextManifest to reference the moved type and contextpkg.ManifestVersion/MinSoftCeiling/MaxSoftCeiling; keep ACPMaxListItems from core/acp.go. Do NOT change any error message string or wire/JSON tag. core imports contextpkg (edge core→context only).
  - acceptance: PinkyMission JSON byte-identical; validator messages unchanged; no import cycle; nil-ReadArtifact adapter output matches pre-extraction bytes
  - verify: go build ./... && go test ./internal/core/...
  - depends: T3
  - requirements: 4, 6

## Wave 4
- [ ] T5 — Rewire external callers
  - why: five external call sites must compile against the new layout (Req 5)
  - role: builder
  - files: internal/cmd/context.go, internal/cmd/dispatch.go, internal/mcp/tools_test.go, internal/testharness/spec_builder.go, internal/testharness/harness_extra_test.go, internal/schema/schema_test.go
  - contract: Update imports/type refs so callers use contextpkg.MissionContextManifest (and contextpkg engine symbols) where they previously used core's. core.SpecStatus/core.Status* call sites should need no change thanks to the Wave 2 aliases. Confirm whether schema_test.go actually references a moved type; if not, leave it. Do NOT change call-site behavior.
  - acceptance: whole module builds and all tests pass
  - verify: go build ./... && go test ./...
  - depends: T4
  - requirements: 5

- [ ] T6 — Raise coverage thresholds on the new packages
  - why: match the bar set by the schema/runner/pack extractions (Req 5)
  - role: builder
  - files: (coverage config used by prior extractions — Makefile/CI/coverpkg list)
  - contract: Add internal/spec and internal/context to the coverage gate at or above the threshold used for internal/schema, internal/runner, internal/pack. Add tests only if needed to clear the bar; prefer moving existing tests. Include a nil-ReadArtifact golden assertion if one does not already exist.
  - acceptance: coverage gate enforces internal/spec and internal/context; CI green
  - verify: go test ./... -cover (and the project's coverage-gate command)
  - depends: T5
  - requirements: 5, 6
