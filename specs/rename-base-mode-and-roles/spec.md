# Spec: Rename Base Mode to Simple Mode and Rename Roles

**Source documents:** `rename-base-mode-and-roles-analysis-plan.md`, `rename-base-mode-and-roles-action-prompt.md`
**Status:** Draft — awaiting user approval before execution
**Confirmed role names (user-selected, G1 ✅):** `investigator`→`scout`, `builder`→`craftsman`, `reviewer`→`auditor`, `verifier`→`validator`. `brain` and `pinky` are unchanged (proper nouns, not personas).

> This spec supersedes the analysis plan's file inventory where live repository
> inspection disagreed with it. Every deviation is called out explicitly in
> §9. The analysis plan is intent guidance; this document is the verified
> implementation contract.

---

## 1. Purpose and Requirement Coverage

Rename the `base` execution mode to `simple`, and rename four of the task-role
personas (`investigator`→`scout`, `builder`→`craftsman`, `reviewer`→`auditor`,
`verifier`→`validator`), across the `specd` Go source, embedded templates,
`.claude/agents/` subagent definitions, JSON Schema, and documentation —
while preserving all existing behavior, CLI contracts, and on-disk
compatibility for specs created under the old names.

| Analysis-plan ID | Requirement | Status here |
|---|---|---|
| R1 | Rename "base" mode in Go source | Covered — §4, Wave 1 (scope corrected: 5 files, not the plan's assumed set) |
| R2 | Update embedded templates for mode | Covered — scope corrected to near-zero; see §9 D1 |
| R3 | Update all documentation | Covered — Wave 2 (mode) + Wave 5 (roles); scope corrected against live grep |
| R4 | Update `.claude/agents/` subagent defs | Covered — Wave 4 |
| R5 | Update CLI command metadata/enums | Covered — Wave 1 |
| R6 | Rename role personas | Covered — Wave 3 (Go registry/logic), Wave 4 (templates/agents) |
| R7 | Update dispatch/context-manifest logic | Covered — Wave 3, expanded to 3 additional files the plan missed |
| R8 | Update tests/fixtures | Covered — Wave 6 |
| R9 | Backward compatibility for `state.json` | Covered — §6, resolved via `EffectiveMode()` normalization, not a schema migration (see §9 D2) |
| R10 | Update validation gates/task schema | Covered — Wave 3 (schema `v1.json` has three role enums, two of them stale even relative to the *current* registry — see §2 F-new-3) |

New requirements discovered during live verification (not present in the
analysis plan, now in scope because they hold hardcoded old-name strings):

| New ID | Requirement | Why in scope |
|---|---|---|
| R11 | Finish the partial `investigator`→`scout` rename already started in `internal/spec/role.go` and `internal/mcp/prompts.go` (registry/prompts already have `scout`, everything else — templates, agents, docs, scaffolding, health checks — still says `investigator`) | Repo is mid-migration; leaving it half-done would ship a third, inconsistent state |
| R12 | Update `internal/core/scaffold.go` `DefaultScaffoldManifest()` — the two hardcoded name lists that control what `specd init` actually writes into a new project | Renaming templates without this makes `specd init` keep creating files under the old names |
| R13 | Update `internal/core/fusion.go` `fusionHealthChecks()` "roles" path list, with an old/new tolerant check (see §9 D3) | Naive rename would make every pre-existing project fail `specd fusion policy` health |
| R14 | Update the three phase→role default pickers: `internal/context/manifest.go` `defaultContextRole()`, `internal/mcp/watcher.go` `roleForStatus()`, `internal/cmd/context.go` `defaultBriefRole()` | All three hardcode `"builder"`/`"verifier"` as phase-based fallback role names |
| R15 | Update default-role fallbacks in `internal/core/specfiles.go` (`ts.Role = "builder"`), `internal/core/orchestration_authoring.go` (three `Role: "builder"` authoring-stage assignments), `internal/testharness/spec_builder.go` (test-fixture default) | These decide what role name gets *written* for future specs/tasks that omit `role:` |
| R16 | Update `internal/schema/schema/v1.json` — three separate role enums (`TaskState.role`, `MissionContextManifest.role`, `ACPMissionPayload.role`); only the second already includes `scout` | Public wire-format contract served by `specd schema`; must stay accurate even though it isn't runtime-enforced (see §2 F-new-3) |
| R17 | Update role references embedded in skill/steering/stub templates: `specd-foundations`, `specd-pinky`, `specd-tasks`, `specd-steering`, `specd-execute` SKILL.md files; `specStubs/tasks.md`, `specStubs/memory.md`; `steering/structure.md`, `steering/reasoning.md`, `steering/workflow.md` | These ship inside the binary and are copied into every new project; several state persona rules by name (e.g. "A builder's word is not evidence") |

---

## 2. Verified Current State

All references below were confirmed by direct inspection on 2026-07-01, branch
`optimization`. Where this contradicts the analysis plan, the plan's claim is
noted and superseded.

### 2.1 Mode ("base")

| File | Reference | Notes |
|---|---|---|
| `internal/core/state.go:16` | `const SchemaVersion = 5` | Current on-disk schema version |
| `internal/core/state.go:23` | `ModeBase = "base"` | Primary constant |
| `internal/core/state.go:24` | `ModeOrchestrated = "orchestrated"` | Unchanged |
| `internal/core/state.go:~210` | `EffectiveMode()` returns `ModeBase` when `ExecutionMode == ""` | **Single resolution point** (INV2) |
| `internal/core/mode.go:19-31` | `ResolveMode(flag, s)` — returns `ModeBase, OriginDefault` when `s == nil \|\| s.ExecutionMode == ""` | A **second**, independent resolution function with the same default-Base logic; must be normalized identically to `EffectiveMode()` |
| `internal/core/mode_recommend.go:121,140` | `rec.Recommended = ModeBase` (×2) | Advisory recommendation output |
| `internal/cmd/mode.go:20` | usage string `[--set base\|orchestrated]` | |
| `internal/cmd/mode.go:49` | `if target != core.ModeBase && target != core.ModeOrchestrated` — validation | |
| `internal/cmd/mode.go:51` | error text `"--set: invalid mode %q, expected base\|orchestrated"` | |
| `internal/cmd/mode.go:67,79` | Base-specific branches: refuse switching away from orchestrated while a Brain session is live; clearing fields on opt-out | |
| `internal/cmd/brain.go:150-151` | comment: `"Base is the default... must never start a session for a Base spec"` | |
| `internal/cmd/brain.go:162` | error text: `"spec '%s' is in base execution mode — Brain/Pinky will not drive it..."` | |
| `internal/core/commands.go:215` | `Command: "mode", ... Hidden: true, DeprecatedIn: "v0.2.0"` | **The `mode` command is already deprecated/hidden**, merged into `status` per the project's own established deprecation pattern (F7 in the analysis plan is correct) |
| `internal/core/commands.go:514-515` | `"new": {Modes: []string{"base","orchestrated"}}`, `"mode": {Modes: []string{"base","orchestrated"}}` | |
| `internal/core/commands.go:591` | `flag.Enum = []string{ModeBase, ModeOrchestrated}` | Drives `--set` flag enum in `specd help --json` |
| `internal/cmd/status.go:78-93,118` | `--set-mode`/`--recommend` **delegate** to `runModeSet`/`runModeRecommend` in `mode.go` — no independent "base" string here | Confirms plan's F7/status.go claim, but the literal strings live only in `mode.go` |
| `docs/agent-integration.md:65` | `"Base mode uses `specd next <slug> --dispatch --json` packets..."` | The **only** "base mode" prose reference found in `docs/` |

**Corrections to the analysis plan's mode-rename inventory** (verified empty
of "base" mode references — **no change needed**):
- `internal/cmd/new.go` — only references `core.ModeOrchestrated` for the
  `--orchestrated` flag; the default path leaves `ExecutionMode` empty and
  never mentions `"base"` literally. Plan's Wave 1 task 3 is based on a false
  premise.
- `internal/cmd/status.go` — no independent "base" string (see above).
- `internal/core/embed_templates/config.yml`, `config.json` — zero matches
  for `base`/`Base` (checked full file content).
- `internal/core/embed_templates/agents/AGENTS.md` — zero matches.
- Root `AGENTS.md` — zero matches for "base"; also has **no** "Role adoption
  rules" section (plan's Wave 4 task referencing it is based on a false
  premise — root `AGENTS.md` is a marker-merged file combining this repo's
  own dev instructions with the embedded template; role personas are
  documented in `docs/agent-integration.md`, not here).
- `docs/command-reference.md` — zero matches (plan's R3 evidence citation is
  wrong for this file specifically).

### 2.2 Roles

`internal/spec/role.go` is the **single authoritative registry** (`Roles
[]RoleDef`, keyed by `Name`) driving tool permissions, budget tier, phase
affinity, file policy, and prompt class for every task role. It already
contains **9 entries**, not the analysis plan's assumed 4:
`scout, researcher, reviewer, architect, builder, tester, documenter,
verifier`, plus a 9th entry — `investigator` — explicitly commented
`// Legacy alias; keep until later deprecation cycle.` with byte-identical
fields to `scout`.

**This means the `investigator`→`scout` rename is already half-done** at the
registry layer (commit `e9af121`, "Complete role system redesign and
phase-aware role assignment," 4 days prior to this analysis). `builder`,
`reviewer`, and `verifier` have **not** been touched anywhere — they remain
the sole, primary names with no alias yet.

Only 4 of the 9 registry roles have corresponding Pinky subagent definitions
today: `investigator` (legacy name, not yet `scout`), `builder`, `reviewer`,
`verifier`. `researcher`, `architect`, `tester`, `documenter` are task-role-only
entries (used for MCP tool/phase gating) with no Pinky dispatch persona and
are **out of scope** for this rename — leave them untouched.

| File | Reference | Notes |
|---|---|---|
| `internal/spec/role.go:18-98` | 9-entry `Roles` slice; `scout` primary (line 18), `investigator` legacy alias (line 90-98, identical fields to `scout`) | `builder` (54), `reviewer` (36), `verifier` (81) still primary/only |
| `internal/mcp/prompts.go:99-141,227-245` | MCP prompt resources: `role/scout` (new text) **and** `role/investigator` (old text, kept) both exist; `role/builder`, `role/reviewer`, `role/verifier` exist with no new-name counterpart yet | Establishes the **dual-resource pattern**: new name gets fresh prompt wording, old name's prompt text is frozen and kept as a separate resource |
| `internal/core/scaffold.go` `DefaultScaffoldManifest()` | Loop 1: `["investigator.md","builder.md","reviewer.md","verifier.md","brain.md","pinky.md"]` → scaffolds `.specd/roles/*.md` from `embed_templates/roles/`. Loop 2 (labeled `GAP-3`): `["builder","investigator","reviewer","verifier"]` → scaffolds `.claude/agents/pinky-*.md` from `embed_templates/agents/pinky-*.md` | **Controls what `specd init` actually writes.** Not mentioned anywhere in the analysis plan. `ScaffoldCreate` policy only creates missing files — never overwrites — so changing these lists does not touch already-initialized projects |
| `internal/core/fusion.go` `fusionHealthChecks()` | `pathHealth("roles", root, []string{".specd/roles/investigator.md", ".../builder.md", ".../reviewer.md", ".../verifier.md", ".../brain.md", ".../pinky.md"})` | `pathHealth` requires **every** listed path to exist — an all-or-nothing check with no alternative-name support today (see §9 D3) |
| `internal/context/manifest.go` `defaultContextRole()` | Returns literal `"builder"` for `PhaseExecute`, `"verifier"` for `PhaseVerify`, `"architect"`/`"documenter"` for other phases (untouched) | |
| `internal/mcp/watcher.go` `roleForStatus()` | Same `"builder"`/`"verifier"`/`"documenter"` phase-status defaults, duplicated independently | |
| `internal/cmd/context.go` `defaultBriefRole()` | Same duplicated logic a third time | |
| `internal/core/specfiles.go:518` | `ts.Role = "builder"` — default role assigned to a parsed `TaskState` when `tasks.md` omits `role:` | Written into `state.json` for **new** task parses; does not retroactively touch existing files |
| `internal/core/orchestration_authoring.go` (A1/A2/A3 authoring stages) | Three `Role: "builder"` assignments for the Brain's requirements/design/tasks authoring missions | |
| `internal/testharness/spec_builder.go:46,266-267` | `Role string // default "builder"`; `if t.Role == "" { t.Role = "builder" }` | Shared test-fixture builder used by dozens of tests |
| `internal/core/tasksparser.go:19` | `ValidRoles = spec.RoleNames()` | **Dynamic** — automatically tracks the registry; no separate list to update |
| `internal/schema/schema/v1.json` | Three independent role enums: `TaskState.role` = `["investigator","builder","reviewer","verifier"]` (line ~74); `MissionContextManifest.role` = `["scout","researcher","reviewer","architect","builder","tester","documenter","verifier","investigator"]` (line ~207, **already matches the current 9-entry registry** — was updated when `scout` was added); `ACPMissionPayload.role` = `["investigator","builder","reviewer","verifier"]` (line ~247) | Two of the three enums are stale even against **today's** registry, independent of this rename — a pre-existing inconsistency this spec also fixes. Confirmed via `internal/schema/schema.go`/`schema_validate.go` that this file is **not runtime-enforced** (no Go code validates data against these enums); it is served verbatim by `specd schema` as a documentation contract, so accuracy still matters (R16) but no test will fail from a stale enum alone |
| `internal/core/embed_templates/roles/{investigator,builder,reviewer,verifier}.md` | 4 template files, each with a `role: <name>` line in a fenced structured-result block | To be renamed to `scout.md`, `craftsman.md`, `auditor.md`, `validator.md` with content updated |
| `internal/core/embed_templates/roles/pinky.md:8` | `"Builder may edit only declared scope; investigator, reviewer, and verifier..."` | Cross-references all 4 personas by name |
| `internal/core/embed_templates/roles/brain.md` | No persona-name cross-references found | No change needed beyond confirming |
| `internal/core/embed_templates/agents/pinky-{builder,investigator,reviewer,verifier}.md` | Byte-identical (`diff` confirmed) to `.claude/agents/pinky-{builder,investigator,reviewer,verifier}.md` | **Two copies must be renamed and edited in lockstep** — no build step syncs them; verify with `diff` after each change |
| `.claude/agents/pinky-{builder,investigator,reviewer,verifier}.md` | Frontmatter `name: pinky-<role>` on line 2 of each | |
| `internal/core/embed_templates/skills/specd-foundations/SKILL.md:50` | references `roles/` directory listing | Check context before editing — may be a directory-structure mention, not persona-specific |
| `internal/core/embed_templates/skills/specd-pinky/SKILL.md:34` | persona reference | |
| `internal/core/embed_templates/skills/specd-tasks/SKILL.md:17,22` | `role:` metadata guidance | |
| `internal/core/embed_templates/skills/specd-steering/SKILL.md:35` | "...naming conventions a builder must follow" | Persona reference (this repo's own terminology, not generic English) |
| `internal/core/embed_templates/skills/specd-execute/SKILL.md:33-36,49` | multiple persona references, including `"A builder's word is not evidence"` (evidence-gating rule) | |
| `internal/core/embed_templates/specStubs/tasks.md`, `specStubs/memory.md` | `role:` field examples/guidance | |
| `internal/core/embed_templates/steering/structure.md`, `steering/reasoning.md`, `steering/workflow.md` | e.g. `"A builder's 'done' is NOT evidence"`, `"builder implements ONE task"` | Persona references embedded in shipped steering templates |
| `docs/agent-integration.md:44-65` | Role personas table (`investigator`, `builder`, `verifier`, `reviewer` — 4 rows only, does **not** include `researcher`/`architect`/`tester`/`documenter`, confirming docs never caught up with the 9-role registry either) + "Base mode uses..." prose | |
| `docs/user-guide.md:319,333,344` | Task metadata table: `` `role` \| ✅ \| Persona: `investigator`, `builder`, `reviewer`, `verifier` ``; two `role: builder` examples | |
| `docs/concepts.md:154-155` | ASCII directory tree: `investigator.md  builder.md  reviewer.md` / `verifier.md      brain.md    pinky.md` | |
| `README.md:171` | `"Read-only roles (investigator/reviewer) whose verify is N/A..."` | |
| `docs/mcp-guide.md:468` | `` `role/builder` `` prompt-resource reference | |
| `docs/validation-gates.md:43` | persona reference | |
| `docs/custom-gates.md:71` | example payload with `"role"` field | |
| `docs/contributor-guide.md:31` | `"...spec builder, assertions..."` | **Not** a persona reference — generic English ("a builder of test specs"). Leave unchanged |
| `docs/command-reference.md`, `docs/open-spec-format.md`, `docs/troubleshooting.md`, `docs/dashboard.md`, `docs/github-action.md`, `docs/spec-packs.md`, `docs/agent-harness-*.md` | Zero matches for any old role name or "base mode" | No changes needed |

### 2.3 Tests referencing mode/role literals

Confirmed to exist (file, not the analysis plan's guessed names, where they
differ): `internal/cmd/mode_cmd_test.go`, `internal/cmd/brain_unit_test.go`,
`internal/cmd/brain_pinky_test.go`, `internal/core/mode_test.go`,
`internal/core/gates_mode_cov_test.go`, `internal/core/state_test.go`,
`internal/core/state_cas_test.go`, `internal/core/state_resume_reject_test.go`,
`internal/core/program_state_test.go`. **No** `dispatch_test.go`,
`next_test.go`, or `status_test.go` exist as standalone files — those
commands' tests live inside `internal/cmd/commands_test.go` and other
integration-style test files. Additional role-name-bearing test files found
via live grep (not in the plan): `internal/spec/role_test.go`,
`internal/spec/spec_test.go`, `internal/mcp/integration_test.go`,
`internal/mcp/tools_test.go`, `internal/mcp/watcher_test.go`,
`internal/context/manifest_test.go`, `internal/context/manifest_extra_test.go`,
`internal/context/manifest_perf_gate_test.go`, `internal/context/slice_test.go`,
`internal/cmd/context_manifest_cmd_test.go`, `internal/cmd/init_test.go`,
`internal/cmd/initpack_test.go`, `internal/cmd/watch_internal_test.go`,
`internal/core/orchestration_authoring_test.go`,
`internal/core/orchestration_test.go`, `internal/core/orchestration_decide_test.go`,
`internal/core/orchestration_driver_test.go`,
`internal/core/orchestration_validate_cov_test.go`,
`internal/core/pinky_test.go`, `internal/core/pinky_evidence_test.go`,
`internal/core/pinky_validate_cov_test.go`,
`internal/core/pinky_context_validate_cov_test.go`,
`internal/core/program_orchestration_test.go`,
`internal/core/program_status_cov_test.go`, `internal/core/acp_test.go`,
`internal/core/checkpoint_fault_test.go`, `internal/core/customgate_test.go`,
`internal/core/cost_brake_test.go`, `internal/core/session_replay_test.go`,
`internal/core/zero_cov_test.go`, `internal/core/tasksparser_test.go`,
`internal/core/commands_test.go` (role/mode enums), `internal/obs/log_test.go`,
`internal/obs/log_cov_test.go`, `internal/worker/shell_runner_test.go`.

---

## 3. Proposed Design and End-to-End Flow

### 3.1 Mode rename flow

```
constant layer      internal/core/state.go: ModeSimple="simple" added, ModeBase kept (deprecated)
        |
resolution layer     EffectiveMode() and ResolveMode() both treat
                      ExecutionMode == "" OR ExecutionMode == ModeBase  =>  "simple"
                      (byte-stable: on-disk value is NEVER rewritten by this change)
        |
CLI validation layer  internal/cmd/mode.go runModeSet: accepts --set simple|orchestrated;
                      --set base is REJECTED with a clear "renamed to simple" error
        |
CLI metadata layer    internal/core/commands.go: enums/help/--json surfaces "simple"
        |
messages layer        internal/cmd/brain.go refusal text says "simple execution mode"
        |
docs layer             docs/agent-integration.md "Base mode" -> "Simple mode"
```

### 3.2 Role rename flow (applied identically to all 4 renames)

```
registry layer        internal/spec/role.go: new name becomes the primary Roles[] entry
                       (in place of the old one); old name appended at the bottom as a
                       byte-identical entry commented "// Legacy alias; keep until later
                       deprecation cycle." (exactly mirroring the existing scout/investigator
                       pair) so RoleByName/RoleNames/IsReadonlyRole/RoleTools/etc. resolve both
        |
MCP prompts layer      internal/mcp/prompts.go: new "role/<new>" resource with fresh prompt
                       text; old "role/<old>" resource kept with its EXISTING (frozen) text
                       (exactly mirroring the existing scout/investigator pair)
        |
schema layer           internal/schema/schema/v1.json: all three role enums list every
                       canonical name (new) AND every legacy alias (old), so the format
                       stays self-consistent and matches the registry 1:1
        |
scaffolding layer      internal/core/scaffold.go DefaultScaffoldManifest(): both hardcoded
                       name lists switch to the new canonical names only (new projects get
                       scout.md/craftsman.md/auditor.md/validator.md; ScaffoldCreate never
                       touches already-initialized projects, so old projects are unaffected)
        |
health-check layer     internal/core/fusion.go fusionHealthChecks(): "roles" check accepts
                       EITHER the new-name file OR the legacy-name file per role (new logic,
                       not a pure rename -- required so existing projects don't fail health)
        |
default-role layer     internal/context/manifest.go, internal/mcp/watcher.go,
                       internal/cmd/context.go, internal/core/specfiles.go,
                       internal/core/orchestration_authoring.go,
                       internal/testharness/spec_builder.go: all hardcoded "builder"/
                       "verifier" phase/status/parse defaults switch to "craftsman"/
                       "validator" (these are forward-only defaults for new writes, not
                       migrations of existing data)
        |
template layer         embed_templates/roles/*.md and embed_templates/agents/pinky-*.md
                       renamed + content updated; .claude/agents/pinky-*.md renamed +
                       kept byte-identical to the embedded copy; skill/steering/stub
                       templates updated for persona-name prose references
        |
docs layer              docs/agent-integration.md role table, docs/user-guide.md role
                        table + examples, docs/concepts.md tree diagram, README.md,
                        docs/mcp-guide.md, docs/validation-gates.md, docs/custom-gates.md
        |
test layer              every test file in §2.3 updated to assert new names where the
                         old name was the point of the test; back-compat tests ADDED to
                         assert legacy names still resolve
```

---

## 4. Interfaces, Contracts, Data, Configuration, Dependencies

- **`state.json` schema:** stays at `SchemaVersion = 5` — no bump, no new
  migration rule (see §9 D2). `executionMode` field values: `""` (unchanged
  meaning: simple), `"base"` (legacy, still accepted on read, resolves to
  simple), `"simple"` (new, accepted for explicit `--set simple`),
  `"orchestrated"` (unchanged).
- **`state.json` `tasks[].role` field:** existing values (`investigator`,
  `builder`, `reviewer`, `verifier`) remain valid forever via the registry's
  legacy-alias entries. New task parses that omit `role:` get the new
  canonical default names.
- **CLI:** `specd mode <spec> --set simple` succeeds; `specd mode <spec>
  --set base` fails with a clear, actionable error. `specd status <spec>
  --set-mode simple` succeeds via the same delegated path.
- **CLI help/JSON:** `specd help mode --json` and `specd help new --json`
  show `"simple"` in the mode enum (not `"base"`).
- **MCP prompt resources:** `role/scout`, `role/craftsman`, `role/auditor`,
  `role/validator` are new; `role/investigator`, `role/builder`,
  `role/reviewer`, `role/verifier` remain available (frozen text) for
  back-compat.
- **JSON Schema (`specd schema`):** version id stays `"1"` (the wire format
  shape does not change, only enum membership grows) — three role enums
  updated to include both canonical and legacy names.
- **`.claude/agents/`:** renamed to `pinky-scout.md`, `pinky-craftsman.md`,
  `pinky-auditor.md`, `pinky-validator.md`. No legacy files kept here (Claude
  Code subagent definitions are resolved by filename at dispatch time in this
  dev repo; old dispatch flows referencing the old filenames are not a
  supported external contract the way `state.json`/`tasks.md` are).
- **Dependencies:** none change; still Go stdlib only, `go:embed`.

---

## 5. Invariants, Security, Errors, Observability, Compatibility, Rollback

- **INV1 (byte-stability):** an existing `state.json` with `executionMode:
  ""` or `executionMode: "base"` is read, resolved, and — if never
  re-`--set`, never re-saved — its bytes on disk are **never** rewritten by
  this change. Re-verify with a round-trip test: load, don't mutate, save,
  `diff` against original bytes.
- **INV2 (single resolution point):** both `EffectiveMode()` (core/state.go)
  and `ResolveMode()` (core/mode.go) must apply the identical `"" | "base" =>
  "simple"` normalization — these are two independent functions today; both
  must be updated or a resolution-mismatch bug is introduced.
- **INV3 (role references in tasks.md/dispatch/context):** covered by the
  registry legacy-alias pattern (§3.2) — no `tasks.md`, dispatch packet, or
  context manifest that names an old role becomes invalid.
- **INV4 (`go:embed` read-only at runtime):** unchanged; `make build` is
  required after any `embed_templates/` edit before testing behavior that
  depends on it.
- **Security:** none. This is a pure identifier/string rename; no new attack
  surface. `internal/core/fusion.go`'s new OR-based health check must not
  introduce a path-traversal issue — it only compares fixed literal
  candidate filenames, never user input.
- **Errors:** `specd mode <spec> --set base` must fail with `core.ExitUsage`
  and a message naming the replacement (`--set simple`), not a generic
  "invalid mode" message — this is the plan's explicitly stated success
  criterion (§1 of the analysis plan), taking priority over the softer
  "or deprecation warning" alternative offered later in the same document
  (see §9 D4).
- **Observability:** no structured logging/metrics reference "base" or the
  old role names by inspection of `internal/obs/`; the two `obs` test files
  in §2.3 were flagged by a broad grep and must be checked, but are expected
  to need no change (confirm during Wave 6, don't assume).
- **Compatibility:** existing `tasks.md` files using old role names continue
  to validate and dispatch correctly forever (not just through a deprecation
  window) — the registry's legacy-alias pattern was the project's own
  precedent, so this spec keeps it rather than scheduling a breaking removal.
  This is a **deviation** from the analysis plan's U1/U5 framing of the role
  rename as an accepted breaking change (see §9 D5).
- **Rollback:** `git revert` of the rename commit(s). No data migration was
  performed, so rollback carries zero data-loss risk.

---

## 6. Acceptance Criteria and Validation Commands

- [ ] `make build && make test` passes (race detector clean)
- [ ] `make ci` passes (lint, race, `-count=2`, coverage floor, stress)
- [ ] `specd mode test-spec --set simple` succeeds and `state.json` records
      `"executionMode": "simple"`
- [ ] `specd mode test-spec --set base` fails with exit code for usage error
      and an error message containing `simple`
- [ ] `specd status test-spec --set-mode simple --json` shows `"mode":
      "simple"`
- [ ] A hand-crafted `state.json` with `"executionMode": "base"` loads via
      `specd status <spec> --json` and reports `"mode": "simple"`, with the
      file's bytes unchanged after the read-only invocation
- [ ] `specd help mode --json` and `specd help new --json` show `"simple"`
      in the mode enum, not `"base"`
- [ ] `ls internal/core/embed_templates/roles/` shows exactly `scout.md`,
      `craftsman.md`, `auditor.md`, `validator.md`, `brain.md`, `pinky.md`
- [ ] `ls internal/core/embed_templates/agents/` and `ls .claude/agents/`
      both show exactly `pinky-scout.md`, `pinky-craftsman.md`,
      `pinky-auditor.md`, `pinky-validator.md`; `diff` between each pair of
      files (embedded vs. `.claude/agents/`) reports no differences
- [ ] A fresh `specd init` in an empty test directory creates
      `.specd/roles/scout.md` etc. (not `investigator.md`) and
      `.claude/agents/pinky-scout.md` etc.
- [ ] `specd fusion policy` (or equivalent health-check invocation) reports
      the `roles` check healthy against **both** (a) a freshly-`init`'d
      project (new names) and (b) a synthetic old-style project directory
      containing only `investigator.md`/`builder.md`/`reviewer.md`/
      `verifier.md`/`brain.md`/`pinky.md`
- [ ] A `tasks.md` using `role: investigator` (or `builder`/`reviewer`/
      `verifier`) still passes `specd check`
- [ ] A `tasks.md` using `role: scout` (or `craftsman`/`auditor`/
      `validator`) passes `specd check`
- [ ] `specd schema --version 1` output's three role enums each contain both
      every canonical name and every legacy alias
- [ ] `grep -rE "\bModeBase\b" --include="*.go" internal/` returns only the
      deprecated-constant declaration and its doc comment, no other live
      usage outside the explicit legacy-acceptance branches
- [ ] `grep -rn "base mode\|base execution" docs/ AGENTS.md README.md` returns
      empty
- [ ] `grep -rn "investigator\|builder\|reviewer\|verifier" docs/
      .claude/agents/ internal/core/embed_templates/` returns only
      intentional legacy/back-compat mentions (each one must be traceable to
      a specific "kept for compatibility" note — not a missed rename)

---

## 7. Open Decisions and Deviations from the Analysis Plan

**D1 — No template changes needed for the mode rename (contra plan R2).**
`config.yml`, `config.json`, and the embedded `AGENTS.md` contain zero "base"
references. Wave 2 in this spec is therefore much smaller than the analysis
plan's Wave 2 — it is effectively one doc-prose fix
(`docs/agent-integration.md` line 65) plus the CLI-facing string work already
covered in Wave 1.

**D2 — No `SchemaVersion` bump, no `migrate()` changes (contra plan R9/F2/F8).**
The existing `migrate()` function only performs shape-compatible version
stamping — it has no precedent for rewriting a *value* inside `state.json`.
Introducing one now, just to rewrite `"base"` to `"simple"`, would be riskier
and less idiomatic than doing the normalization at the resolution layer
(`EffectiveMode()`/`ResolveMode()`), which is explicitly documented as "the
single resolution point" already. This also gives a stronger byte-stability
guarantee than a migration would (a file that's never re-saved keeps saying
`"base"` on disk forever, but always *resolves* to `"simple"` — no
migration-write required, no risk of a migration bug corrupting old files).

**D3 — `fusionHealthChecks()` needs new OR-logic, not a pure rename.**
A naive string-swap in `fusionHealthChecks()`'s roles path list would make
every project scaffolded before this change fail its next `specd fusion
policy` health check, since their `.specd/roles/` directory still has
`investigator.md`/`builder.md`/`reviewer.md`/`verifier.md`. This spec adds a
small amount of new logic (accept new-name-or-legacy-name per role) rather
than treating this as a pure string replacement — flagged here because it is
qualitatively different work from the rest of the rename and needs its own
test coverage.

**D4 — `--set base` fails closed, it does not warn-and-accept.**
The analysis plan's §1 states the primary success criterion as `--set base`
"fails with clear error." Its own Definition of Done (§10) later softens this
to "fails... (or deprecation warning)." This spec picks the stricter,
earlier-stated behavior: reject explicit new usage of `--set base` outright
(directing the user to `--set simple`), while still silently accepting
already-persisted `"executionMode": "base"` values on read. This is not
contradictory — it distinguishes "new user input" (reject, teach the new
name) from "old stored data" (accept transparently, per INV1).

**D5 — Role rename is permanent aliasing, not a scheduled breaking change.**
The analysis plan's U1/U5 risks frame the role rename as an accepted,
time-boxed breaking change requiring a migration command or manual fix-up.
Live inspection shows the project already has an established, working
pattern for exactly this situation — `internal/spec/role.go`'s `scout`/
`investigator` pair, kept indefinitely ("Legacy alias; keep until later
deprecation cycle"). This spec follows that existing precedent for all four
renames rather than introducing a new, harsher breaking-change policy or a
one-off `specd migrate roles` command that doesn't exist as infrastructure
today. If the user wants a hard breaking change instead, that is a decision
to make explicitly before Wave 3 begins (see the question below).

**Question for the user before implementation starts:** confirm D2–D5 above
match intent, in particular whether legacy role/mode names should remain
permanently valid (this spec's recommendation) versus removed on a fixed
future timeline.

**Deferred, explicitly out of scope:** renaming or documenting the other 5
registry roles (`researcher`, `architect`, `tester`, `documenter`, and
finishing `scout`'s own Pinky-agent/template/docs rollout beyond what
`investigator`→`scout` requires here) is a separate, pre-existing gap not
requested by the user and not touched by this spec beyond what's needed to
keep `scout` consistent with the other three renamed personas.
