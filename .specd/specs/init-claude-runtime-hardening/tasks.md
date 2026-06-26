# Tasks — init-claude-runtime-hardening

Task records carry the 7 mandatory keys (`why`, `role`, `files`, `contract`,
`acceptance`, `verify`, `depends`) per the Task-schema gate. `acceptance` maps to
requirement IDs in `spec.md` §3.

---

- [x] **T1 — Runtime gitignore template + manifest entry**
  - role: builder
  - why: `.specd/runtime/` is NOT git-ignored today (`git check-ignore` confirms); runtime orchestration state must never be staged/pushed.
  - files: `internal/core/embed_templates/runtime.gitignore`, `internal/core/scaffold.go`
  - contract: Add embed template `runtime.gitignore` containing `*` + `!.gitignore` with a header comment. Append `ScaffoldAsset{Template:"runtime.gitignore", Target:".specd/runtime/.gitignore", Policy:ScaffoldCreate, Required:true, Refresh:true}` to `DefaultScaffoldManifest()`.
  - acceptance: 2.1, 2.2, 2.3
  - verify: `go test ./internal/core/... -run 'Scaffold|Init|Manifest'`
  - depends: none

- [x] **T2 — Verify gitignore via fresh init (integration)**
  - role: verifier
  - why: Prove a fresh `specd init` produces a `.specd/runtime/` whose contents are git-ignored.
  - files: `internal/cmd/init_test.go`
  - contract: Add/extend a test that runs init into a temp git repo and asserts `.specd/runtime/.gitignore` exists and `git check-ignore .specd/runtime/<file>` reports ignored while `.gitignore` itself stays tracked.
  - acceptance: 2.1, 2.2
  - verify: `go test ./internal/cmd/... -run Init`
  - depends: T1

- [x] **T3 — CLAUDE.md embed template**
  - role: builder
  - why: Claude Code loads `CLAUDE.md`, not `AGENTS.md`; without it the harness contract is invisible to the agent.
  - files: `internal/core/embed_templates/CLAUDE.md`
  - contract: Author `CLAUDE.md` template wrapping `@AGENTS.md` import inside the same managed-marker tokens the AGENTS.md merge recognizes (confirm tokens in `internal/core` first). No prompt content duplicated.
  - acceptance: 1.2
  - verify: `go test ./internal/core/... -run Template`
  - depends: none

- [x] **T4 — Confirm/generalize marker-merge for CLAUDE.md**
  - role: investigator
  - why: `marker-merge` is exercised only by `AGENTS.md`; must confirm `MergeAgentsMD`/`ValidateAgentsMD` are filename-agnostic before reuse.
  - files: `internal/core/`
  - contract: Locate the marker-merge implementation; report whether it hardcodes the `AGENTS.md` filename or operates on any marker file. Return file:line findings and a yes/no on whether a refactor is needed for `CLAUDE.md`. Read-only.
  - acceptance: 1.3
  - verify: N/A
  - depends: T3

- [x] **T5 — Conditional CLAUDE.md asset when claude-code selected**
  - role: builder
  - why: CLAUDE.md must be written only when claude-code is selected/detected (R1.4), so non-Claude repos stay clean.
  - files: `internal/cmd/init.go`
  - contract: After selection resolves (`selected.Selected`), if `claude-code` is present, splice a `ScaffoldAsset`/`InitAction{Target:"CLAUDE.md", Policy:marker-merge, Kind:"merge"}` into `plan.Actions` before `ExecuteInitPlan`, mirroring the orchestration post-plan mutation pattern (`init.go:294-323`). Do nothing when claude-code is absent.
  - acceptance: 1.1, 1.4
  - verify: `go test ./internal/cmd/... -run Init`
  - depends: T3, T4

- [x] **T6 — Init tests for CLAUDE.md presence/absence**
  - role: verifier
  - why: Lock R1.1/R1.4 — CLAUDE.md appears iff claude-code is selected.
  - files: `internal/cmd/init_test.go`
  - contract: Add cases: (a) `--agent claude-code` → root `CLAUDE.md` written importing AGENTS.md; (b) `--agent none`/non-Claude host → no `CLAUDE.md`; (c) re-run is idempotent and preserves user content outside markers.
  - acceptance: 1.1, 1.3, 1.4
  - verify: `go test ./internal/cmd/... -run Init`
  - depends: T5

- [x] **T7 — Task-ID segment validation for mission filenames**
  - role: investigator
  - why: `validateACPRuntimeSegment` regex is `^[a-z0-9][a-z0-9-]*$` (lowercase); real task IDs like `T1` are uppercase and would be rejected.
  - files: `internal/core/runtime_paths.go`
  - contract: Determine the actual task-ID charset used across the codebase and whether a dedicated validator/normalizer is needed for `MissionPath`. Return findings + recommendation (new validator vs. normalize-to-lower). Read-only.
  - acceptance: 3.2
  - verify: N/A
  - depends: none

- [x] **T8 — Add MissionsDir/MissionPath to ACPRuntimePaths**
  - role: builder
  - why: No canonical runtime mission path exists; missions need a deterministic spec-scoped home so tasks/specs/attempts never collide on disk.
  - files: `internal/core/runtime_paths.go`
  - contract: Add `MissionsDir()` and `MissionPath(slug, taskID string, attempt int)` returning `missions/<slug>-<taskID>-<attempt>.json` under the validated runtime root. Validate slug (`ValidateSlug`), task ID (per T7 finding), and `attempt >= 1`; reject traversal/symlinks via existing `join`/`validate`.
  - acceptance: 3.1, 3.2, 3.4
  - verify: `go test ./internal/core/... -run 'Runtime|MissionPath'`
  - depends: T7

- [x] **T9 — Persist missions to spec-scoped runtime path**
  - role: builder
  - why: Today missions go only to an ephemeral spec-agnostic temp file; persist a durable, inspectable, deterministic record so re-issued attempts overwrite rather than duplicate.
  - files: `internal/worker/shell_runner.go`
  - contract: Write the mission JSON via `AtomicWrite` to `ACPRuntimePaths.MissionPath(slug, taskID, attempt)`. Keep the worker subprocess hand-off contract unchanged (temp file may remain as the transport). Derive slug/taskID/attempt from the mission payload.
  - acceptance: 3.3, 3.4
  - verify: `go test ./internal/worker/... -run Mission`
  - depends: T1, T8

- [x] **T10 — Update manifest parity/golden tests**
  - role: verifier
  - why: New scaffold targets must not break `SortedScaffoldTargets`/parity goldens.
  - files: `internal/core/initplan_test.go`, `internal/cmd/init_test.go`
  - contract: Update any golden/parity assertions to include `.specd/runtime/.gitignore` and the conditional `CLAUDE.md`. Run full init/scaffold suites green.
  - acceptance: 2.1, 1.1
  - verify: `go test ./internal/core/... ./internal/cmd/...`
  - depends: T1, T5, T9
