# tasks.md — Context-Engineering Level Up (Pinky ⇄ Brain ⇄ MCP)

Implementation DAG for `spec.md`. Tasks are grouped into dependency **waves**; tasks in
the same wave are parallelizable. Each task lists `role`, `files`, `contract`,
`acceptance`, `verify`, `depends`, `requirements`.

Conventions: Go 1.x, no new deps, no LLM in core, all output deterministic. Run
`make test` and `make lint` (fmt-check + shellcheck + test-lint) before completing any
task. Keep manifest wire version at `1`; all additions additive + `omitempty`. Mirror
existing test style in `internal/core/*_test.go` and `internal/cmd/*_test.go`.

---

## Wave 0 — Foundations (no dependencies)

### T1 — Token estimator
- **role:** builder
- **files:** `internal/core/context_estimate.go`, `internal/core/context_estimate_test.go`
- **contract:** Add `func EstimateTokens(b []byte) int` and `func EstimateTokensString(s string) int`: deterministic heuristic (`ceil(len/4)` baseline with a Markdown-aware adjustment for code fences/tables). No tokenizer dependency, no IO. Pure and total.
- **acceptance:** Same input → same output; empty → 0; monotonic in length; documented heuristic.
- **verify:** `go test ./internal/core/ -run EstimateTokens`
- **depends:** —
- **requirements:** AC-2, AC-8

### T2 — Deterministic slicers
- **role:** builder
- **files:** `internal/core/context_slice.go`, `internal/core/context_slice_test.go`
- **contract:** Add pure slicers (no IO; take content + selector, return slice string + found bool): `TaskSlice(tasksMd, taskID)`; `CoveredRequirements(reqMd, ids []int)`; `DesignSection(designMd, headings []string)`; `RecentMemory(memoryMd string, n int)`. Each returns the whole input only when no selector matches AND caller opts into fallback (return found=false so caller decides). Markdown-heading / checkbox aware, deterministic ordering.
- **acceptance:** Slices contain only the requested block; unknown selector → found=false, empty slice; ordering stable.
- **verify:** `go test ./internal/core/ -run Slice`
- **depends:** —
- **requirements:** AC-3

---

## Wave 1 — Shared context engine

### T3 — Unified context manifest engine
- **role:** builder
- **files:** `internal/core/context_manifest.go`, `internal/core/context_manifest_test.go`
- **contract:** Add `ContextRequest{ Slug, Status, TaskID, Role, Files, Mode (briefing|dispatch|mission), HostBudget int }` and `func BuildContextManifest(req ContextRequest) MissionContextManifest`. Reuse existing `MissionContextManifest`/`MissionContextItem` types from `pinky_context.go` (do not fork them). Logic: role → skill → phase-skill → spec-context → scoped files → **phase-filtered** source artifacts. Wire `EstimateTokens` (T1) for `TokenHint` and the slicers (T2) for `read-targeted` items. Keep `BuildMissionContextManifest` as a thin adapter that calls this engine (mission mode) so Surface C output stays equivalent except for measured hints.
- **acceptance:** Engine reproduces today's mission item ordering/kinds; `TokenHint` now measured; source artifacts filtered by phase relevance; pure (no IO except reading the named artifacts via injected reader).
- **verify:** `go test ./internal/core/ -run ContextManifest`
- **depends:** T1, T2
- **requirements:** AC-1, AC-2, AC-3, AC-7

### T4 — Budget + accounting fields
- **role:** builder
- **files:** `internal/core/pinky_context.go`, `internal/core/context_manifest.go`, plus `_test.go`
- **contract:** Add additive `omitempty` fields to `MissionContextManifest`: `EstimatedTokens int`, `Budget int`. Compute `EstimatedTokens` = sum of required-item hints. Compute `Budget` from `(phase, role, file count, HostBudget)` clamped to `[minMissionContextSoftCeiling, maxMissionContextSoftCeiling]`, defaulting to `missionContextSoftCeiling` (12000). Extend `validateMissionContextManifest` to accept (not require) the new fields and keep version `1`.
- **acceptance:** Absent/zero new fields → byte-identical JSON to today (AC-7); validator still passes existing fixtures; budget within bounds.
- **verify:** `go test ./internal/core/ -run 'ContextManifest|Validate'`
- **depends:** T3
- **requirements:** AC-4, AC-6, AC-7

---

## Wave 2 — Surface integration

### T5 — `specd context` emits the manifest (Surface A)
- **role:** builder
- **files:** `internal/cmd/context.go`, `internal/cmd/context_skill_test.go` (+ new test)
- **contract:** Replace the ad-hoc `buildBrief().load` flat list with a call to `BuildContextManifest(mode=briefing)`. Human output gains a `LOAD NOW` table (item, mode, ~tokens, why) and a budget line (`est X / budget Y`). `--json` output gains the `contextManifest` block (with `estimatedTokens`, `budget`). Preserve `phaseLabel/purpose/focus/next/signals/gate` exactly. In `executing`, scope files to the next runnable task; use `RecentMemory` instead of whole `memory.md`.
- **acceptance:** Existing context fields unchanged; new manifest present; gated/blocked/uncovered paths still print; JSON keys additive.
- **verify:** `go test ./internal/cmd/ -run Context`
- **depends:** T3, T4
- **requirements:** AC-1, AC-8

### T6 — `specd dispatch` dedupes role assets (Surface B)
- **role:** builder
- **files:** `internal/cmd/dispatch.go`, `internal/cmd/dispatch_test.go` (add if absent)
- **contract:** Stop inlining `RolePrompt` per packet. Add a top-level `assets` map (`role/<name> → path`) to `frontierOut`; each packet references its role by name + path. Attach a per-packet context manifest via `BuildContextManifest(mode=dispatch, taskID=f.ID)`. Add `--inline-roles` flag to restore full-text inlining for hosts that need it (back-compat).
- **acceptance:** A multi-task same-role wave contains the role prompt bytes once; `--inline-roles` reproduces old shape; human output still lists frontier.
- **verify:** `go test ./internal/cmd/ -run Dispatch`
- **depends:** T3
- **requirements:** AC-5, AC-7

### T7 — `context-budget` validation gate
- **role:** builder
- **files:** `internal/core/gates.go`, `internal/core/gates_test.go`
- **contract:** Add an opt-in gate `context-budget`: for the active spec build the manifest and warn/fail when required-item `EstimatedTokens > Budget`, naming the top heaviest items. Register alongside the existing core/opt-in gates following the file's current registration pattern. Opt-in (off by default) so the 7 core gates' behaviour is unchanged.
- **acceptance:** Off by default (no change to `specd check` defaults); when enabled, over-budget fixture reports the offenders; under-budget passes.
- **verify:** `go test ./internal/core/ -run Gate`
- **depends:** T4
- **requirements:** AC-4

---

## Wave 3 — MCP negotiation

### T8 — MCP `maxContextTokens` negotiation
- **role:** builder
- **files:** `internal/mcp/negotiation.go`, `internal/mcp/negotiation_test.go` (or `host_compat_test.go`)
- **contract:** Parse optional `capabilities.specd.maxContextTokens` in `initialize`, mirroring the existing `maxTools` contract (ignore ≤0 / garbage safely, persist for session). Thread the value into `ContextRequest.HostBudget` for manifests produced during the session.
- **acceptance:** Omitting the hint → byte-identical to today (AC-6); a valid hint caps `Budget`; garbage ignored without tearing down `initialize`.
- **verify:** `go test ./internal/mcp/ -run 'Negotiat|Capab'`
- **depends:** T4
- **requirements:** AC-6, AC-8

---

## Wave 4 — Skills, docs, coverage

### T9 — Update Pinky/Brain skills + steering for budgeted context
- **role:** builder
- **files:** `internal/core/embed_templates/skills/specd-pinky/SKILL.md`, `internal/core/embed_templates/skills/specd-brain/SKILL.md`, `internal/core/embed_templates/steering/reasoning.md`
- **contract:** Document the budget/accounting fields and the targeted-slice modes: workers load required items in order, expand `reference-if-needed`/optional only when contract demands, and stop before `Budget`. Note that `read-targeted` items are slices, not whole files. Keep edits terse; no behavioural promises beyond what core enforces.
- **acceptance:** Skill text matches shipped field names/modes; embed builds; no template-drift test breakage.
- **verify:** `go test ./internal/core/ -run 'Skill|Template|Embed'`
- **depends:** T3, T4
- **requirements:** AC-1, AC-3

### T10 — Docs: agent-integration, mcp-guide, command-reference
- **role:** builder
- **files:** `docs/agent-integration.md`, `docs/mcp-guide.md`, `docs/command-reference.md`
- **contract:** Document the unified context engine, the manifest accounting fields (`estimatedTokens`, `budget`), targeted slicing, the `context-budget` gate, dispatch role-asset dedupe + `--inline-roles`, and `capabilities.specd.maxContextTokens`. Update the MCP guide's stale "No resources or prompts" / "Static tool list" limitation rows if contradicted by current code.
- **acceptance:** Docs reference real field/flag names; examples valid; links resolve.
- **verify:** `./scripts/test-lint.sh`
- **depends:** T5, T6, T7, T8
- **requirements:** AC-1, AC-4, AC-5, AC-6

### T11 — Coverage floor + integration test for unified context
- **role:** builder
- **files:** `internal/mcp/integration_test.go` (or `internal/core/context_manifest_test.go`), `scripts/coverage-check.sh` if floors need raising
- **contract:** Add an end-to-end test: build manifests for the same task via Surface A (`context`), B (`dispatch`), and C (mission) and assert they reference the same engine output (same items/kinds/order for shared inputs) and respect a small `HostBudget`. Keep/raise coverage floors per existing `coverage-check.sh` policy.
- **acceptance:** Test proves single-engine parity across surfaces and budget capping; coverage floors hold.
- **verify:** `make test`
- **depends:** T5, T6, T8
- **requirements:** AC-1, AC-6, AC-8

---

## Wave Summary

```
Wave 0:  T1  T2
Wave 1:  T3(T1,T2)  T4(T3)
Wave 2:  T5(T3,T4)  T6(T3)  T7(T4)
Wave 3:  T8(T4)
Wave 4:  T9(T3,T4)  T10(T5,T6,T7,T8)  T11(T5,T6,T8)
```

## Definition of Done (whole spec)

- All AC-1…AC-8 satisfied with tests.
- `make test && make lint` green; coverage floors held.
- Manifest wire stays version `1`; omitting new fields reproduces pre-feature bytes.
- One shared `BuildContextManifest` feeds all three surfaces — no duplicate load logic.
- No LLM in core; no new dependencies; estimates deterministic.
