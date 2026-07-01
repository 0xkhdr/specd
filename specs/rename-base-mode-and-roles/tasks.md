# Tasks: Rename Base Mode to Simple Mode and Rename Roles

Spec: `specs/rename-base-mode-and-roles/spec.md`. Read that spec's §2
(Verified Current State) and §9 (Open Decisions) before starting — several
tasks below depend on the deviations documented there (no schema-version
bump, `--set base` fails closed, roles keep permanent legacy aliases).

Each wave must pass its validation step before the next wave starts. Do not
skip validation to "save time" — a broken wave compounds into the next one.

---

## Wave 1 — Mode rename: constants, resolution, CLI, metadata

No dependencies. This wave touches exactly 6 Go files; do not touch
`internal/cmd/new.go` or `internal/cmd/status.go` — verified to contain zero
literal `"base"` references (see spec §2.1).

- [ ] **`internal/core/state.go`** — add `ModeSimple = "simple"` next to the
      existing `const (ModeBase = "base"; ModeOrchestrated = "orchestrated")`
      block; add a doc comment on `ModeBase` marking it deprecated/legacy
      (e.g. `// ModeBase is the deprecated legacy value for ModeSimple; still
      accepted when read from state.json or resolved via EffectiveMode(),
      rejected for new --set input.`). In `EffectiveMode()`, change the
      resolution so it returns `ModeSimple` when `ExecutionMode == "" ||
      ExecutionMode == ModeBase` (currently only checks for `""`). Do **not**
      change `SchemaVersion` and do **not** touch `migrate()` (spec §9 D2).
- [ ] **`internal/core/mode.go`** — `ResolveMode(flag, s)` currently returns
      `ModeBase, OriginDefault` when `s == nil || s.ExecutionMode == ""`.
      Change to return `ModeSimple, OriginDefault` in that branch, **and**
      add a check so an explicit legacy `s.ExecutionMode == ModeBase` also
      resolves to `ModeSimple` (mirror the `EffectiveMode()` logic exactly —
      these two functions must never disagree, per INV2).
- [ ] **`internal/core/mode_recommend.go`** — both occurrences of
      `rec.Recommended = ModeBase` (lines ~121, ~140) become `rec.Recommended
      = ModeSimple`.
- [ ] **`internal/cmd/mode.go`** — update:
      - usage string `"usage: specd mode <slug> [--set base|orchestrated]
        [--recommend] [--json]"` → `[--set simple|orchestrated]`
      - `runModeSet` validation: `if target != core.ModeBase && target !=
        core.ModeOrchestrated` → `if target != core.ModeSimple && target !=
        core.ModeOrchestrated`. Per spec §9 D4, this must **reject** an
        explicit `--set base` request (it will now fail this check since
        `"base" != ModeSimple`) — confirm the error message names the
        replacement: `"--set: invalid mode %q, expected simple|orchestrated
        (base has been renamed to simple)"` or similar, not a bare "invalid
        mode" message.
      - the "switching to Base while a Brain session is live" branch
        (`target == core.ModeBase && state.EffectiveMode() ==
        core.ModeOrchestrated`) → `target == core.ModeSimple && ...`, and its
        error text `"cannot switch '%s' to base:"` → `"cannot switch '%s' to
        simple:"`
      - the opt-out branch `if target == core.ModeBase { state.ExecutionMode
        = ""; ... }` → `if target == core.ModeSimple { ... }` (still clears
        fields for byte-stability, per INV1)
- [ ] **`internal/cmd/brain.go`** — in `requireOrchestratedSpec`: update the
      doc comment ("Base is the default... never start a session for a Base
      spec") to say "Simple", and the runtime error text `"spec '%s' is in
      base execution mode — Brain/Pinky will not drive it..."` → `"...is in
      simple execution mode..."`.
- [ ] **`internal/core/commands.go`** — update:
      - `"new": {Modes: []string{"base", "orchestrated"}}` and `"mode":
        {Modes: []string{"base", "orchestrated"}}` → `{"simple",
        "orchestrated"}`
      - `flag.Enum = []string{ModeBase, ModeOrchestrated}` → `{ModeSimple,
        ModeOrchestrated}`
- [ ] Run `make build` — confirm it compiles.
- [ ] **Validation:** `go build ./... && go test ./internal/core/...
      ./internal/cmd/... 2>&1 | tee /tmp/wave1.log` — expect failures only in
      tests still asserting the literal string `"base"` (to be fixed in
      Wave 6); no compile errors, no unrelated failures. Also run: `grep -rn
      "ModeBase" internal/core/state.go internal/core/mode.go` and confirm
      every remaining hit is either the constant declaration or an explicit,
      commented legacy-acceptance branch.

---

## Wave 2 — Mode rename: documentation

Depends on Wave 1 (so terminology matches the shipped behavior). This wave is
intentionally small — live grep found only one genuine "base mode" doc
reference (spec §2.1, §9 D1); do not invent additional busywork edits in
files that don't mention it.

- [ ] **`docs/agent-integration.md`** line ~65 — change `"Base mode uses
      `specd next <slug> --dispatch --json` packets..."` to `"Simple mode
      uses..."`.
- [ ] Re-grep the full doc set to make sure nothing was missed:
      `grep -rniE "\bbase mode\b|\bbase execution\b" docs/ README.md
      AGENTS.md internal/core/embed_templates/` — if this turns up anything
      beyond the line just fixed, update it too and note it as a correction
      to this spec's §2.1 inventory.
- [ ] **Validation:** the grep above returns empty.

---

## Wave 3 — Role rename: Go registry, MCP prompts, schema, scaffolding, defaults

Depends on Wave 1 only (independent of Wave 2). This is the largest and
highest-risk wave — it changes the *behavior* surface (what `specd init`
writes, what a health check accepts), not just prose. Follow the existing
`scout`/`investigator` pattern already in the codebase for every "add
canonical, keep legacy" step below — do not invent a different convention.

- [ ] **`internal/spec/role.go`** — for each of the three pairs
      (`builder`→`craftsman`, `reviewer`→`auditor`, `verifier`→`validator`):
      rename the existing primary `RoleDef.Name` field in place to the new
      name (keep all other fields — `RW`, `BudgetTier`, `PhaseAffinity`,
      `Tools`, `FilePolicy`, `PromptClass` — unchanged), then append a new
      `RoleDef` entry at the bottom of the `Roles` slice (next to the
      existing `investigator` legacy entry) with `Name: "<old name>"` and
      identical field values, preceded by the comment `// Legacy alias; keep
      until later deprecation cycle.` (copy the exact comment already used
      for `investigator`). Result: `Roles` has 12 entries total (9 original
      minus 3 renamed-in-place, plus 3 new legacy aliases, `investigator`
      already present). Do not touch `researcher`, `architect`, `tester`,
      `documenter`.
- [ ] **`internal/mcp/prompts.go`** — for each of the three renames, add a
      new prompt resource `role/craftsman`, `role/auditor`, `role/validator`
      (new Go consts `craftsmanPrompt`, `auditorPrompt`, `validatorPrompt`)
      with prompt text reworded for the new name (mirror the style of the
      existing `scoutPrompt` text, which was freshly written rather than
      copy-pasted from `investigatorPrompt`). Keep the existing `role/builder`,
      `role/reviewer`, `role/verifier` resources and their prompt text
      **completely unchanged** (frozen, exactly like `role/investigator` was
      kept).
- [ ] **`internal/schema/schema/schema/v1.json`** (path:
      `internal/schema/schema/v1.json`) — update all three role enums:
      - `TaskState.role` (currently `["investigator","builder","reviewer","verifier"]`)
      - `ACPMissionPayload.role` (currently the same 4-name list)
      - both become the full current registry:
        `["scout","researcher","reviewer","auditor","architect","builder","craftsman","tester","documenter","verifier","validator","investigator"]`
        (canonical + legacy for all renamed roles, plus the untouched
        `researcher`/`architect`/`tester`/`documenter`)
      - `MissionContextManifest.role` (currently
        `["scout","researcher","reviewer","architect","builder","tester","documenter","verifier","investigator"]`)
        gets the same 3 new canonical names added:
        `["scout","researcher","reviewer","auditor","architect","builder","craftsman","tester","documenter","verifier","validator","investigator"]`
      - Keep the three enums in the same relative member order style as
        today (registry order) so a future diff is easy to read.
- [ ] **`internal/core/scaffold.go`** `DefaultScaffoldManifest()` — change
      the two hardcoded name lists:
      - roles-file loop: `["investigator.md","builder.md","reviewer.md","verifier.md","brain.md","pinky.md"]`
        → `["scout.md","craftsman.md","auditor.md","validator.md","brain.md","pinky.md"]`
      - pinky-agent loop (labeled `GAP-3` in a comment): `["builder","investigator","reviewer","verifier"]`
        → `["craftsman","scout","auditor","validator"]`
      - Do **not** add legacy entries here — `ScaffoldCreate` only creates
        missing files, so already-initialized projects keep whatever they
        already have; new projects should get the new names only (spec §3.2
        scaffolding layer).
- [ ] **`internal/core/fusion.go`** `fusionHealthChecks()` — the "roles"
      health check currently does `pathHealth("roles", root,
      []string{".specd/roles/investigator.md", ...builder..., ...reviewer...,
      ...verifier..., brain.md, pinky.md})`, an all-or-nothing exact-path
      check. Replace this with logic that, for each of the four renamed
      roles, accepts **either** the new-name file or the legacy-name file
      (prefer the new name if both exist; if neither exists, report the
      missing path using the **new** name so the health message teaches the
      current convention). `brain.md`/`pinky.md` are unaffected (unchanged
      names) and stay as plain required entries. This is new logic, not a
      string swap — write it so `pathHealth`'s existing all-or-nothing
      semantics still apply to the *resolved* list you hand it, and add a
      focused unit test (see Wave 6) covering: (a) a project with only new
      names → healthy, (b) a project with only legacy names → healthy, (c) a
      project missing a role entirely → unhealthy, message shows the new
      name.
- [ ] **`internal/context/manifest.go`** `defaultContextRole()` — change the
      literal `"builder"` (execute-phase default) to `"craftsman"` and
      `"verifier"` (verify-phase default) to `"validator"`. Leave
      `"architect"`/`"documenter"` untouched.
- [ ] **`internal/mcp/watcher.go`** `roleForStatus()` — same replacement
      (`"builder"`→`"craftsman"`, `"verifier"`→`"validator"`, leave
      `"documenter"` and any other untouched names alone).
- [ ] **`internal/cmd/context.go`** `defaultBriefRole()` — same replacement.
- [ ] **`internal/core/specfiles.go`** line ~518 — `ts.Role = "builder"`
      (default role for a parsed task with no `role:` key) → `ts.Role =
      "craftsman"`.
- [ ] **`internal/core/orchestration_authoring.go`** — the three
      authoring-stage `Role: "builder"` assignments (A1/A2/A3) → `Role:
      "craftsman"`.
- [ ] **`internal/testharness/spec_builder.go`** — update the doc comment
      `// default "builder"` (line ~46) and the default assignment `t.Role =
      "builder"` (line ~267) → `"craftsman"`.
- [ ] **Validation:** `go build ./...` succeeds. `go test
      ./internal/spec/... ./internal/mcp/... ./internal/schema/...
      ./internal/context/... ./internal/core/... 2>&1 | tee /tmp/wave3.log`
      — expect failures only in tests asserting old literal role-name
      defaults (Wave 6 fixes those); no compile errors. Also:
      `spec.RoleByName("scout")`, `spec.RoleByName("investigator")`,
      `spec.RoleByName("craftsman")`, `spec.RoleByName("builder")`,
      `spec.RoleByName("auditor")`, `spec.RoleByName("reviewer")`,
      `spec.RoleByName("validator")`, `spec.RoleByName("verifier")` must all
      return `ok == true` (write a quick throwaway test or check via
      `go run .` + a small script — remove the throwaway before finishing).

---

## Wave 4 — Role rename: embedded templates and `.claude/agents/`

Depends on Wave 3 (content must reference the same names now valid in the
registry/prompts).

- [ ] Rename `internal/core/embed_templates/roles/investigator.md` →
      `scout.md`; `builder.md` → `craftsman.md`; `reviewer.md` → `auditor.md`;
      `verifier.md` → `validator.md`. Use `git mv` to preserve history.
- [ ] Update each renamed file's content: the `# Role: <Name>` title, the
      fenced structured-result block's `role: <name>` line, and any other
      internal self-reference to the old name. Keep the mandate/rules
      prose meaning unchanged — only the name and (for `scout.md`) whatever
      wording divergence already exists between the old `investigator.md`
      and the new `role/scout` MCP prompt text should be reconciled (pick
      one canonical wording and use it in both places — read
      `internal/mcp/prompts.go`'s `scoutPrompt` const for the already-chosen
      scout wording and match it).
- [ ] **`internal/core/embed_templates/roles/pinky.md`** line ~8 —
      `"Builder may edit only declared scope; investigator, reviewer, and
      verifier..."` → `"Craftsman may edit only declared scope; scout,
      auditor, and validator..."`.
- [ ] Confirm `internal/core/embed_templates/roles/brain.md` needs no change
      (verified no persona cross-references during spec authoring — re-check
      before assuming, per the "read before writing" rule).
- [ ] Rename `internal/core/embed_templates/agents/pinky-investigator.md` →
      `pinky-scout.md`; `pinky-builder.md` → `pinky-craftsman.md`;
      `pinky-reviewer.md` → `pinky-auditor.md`; `pinky-verifier.md` →
      `pinky-validator.md`. Update each file's frontmatter `name:` field and
      all body references (including the `.specd/roles/<name>.md` path
      reference each one points at) to match.
- [ ] Rename `.claude/agents/pinky-investigator.md` → `pinky-scout.md`;
      `pinky-builder.md` → `pinky-craftsman.md`; `pinky-reviewer.md` →
      `pinky-auditor.md`; `pinky-verifier.md` → `pinky-validator.md` — apply
      the **exact same content** as the corresponding
      `internal/core/embed_templates/agents/` file (these two locations must
      stay byte-identical; there is no build step that syncs them, so copy
      manually and verify with `diff`).
- [ ] **`internal/core/embed_templates/skills/specd-foundations/SKILL.md`**
      line ~50 — check the context of the `roles/` reference; update any
      literal old-name mentions.
- [ ] **`internal/core/embed_templates/skills/specd-pinky/SKILL.md`**
      line ~34 — update persona reference.
- [ ] **`internal/core/embed_templates/skills/specd-tasks/SKILL.md`**
      lines ~17, ~22 — update `role:` metadata guidance/examples.
- [ ] **`internal/core/embed_templates/skills/specd-steering/SKILL.md`**
      line ~35 — `"...naming conventions a builder must follow"` →
      `"...naming conventions a craftsman must follow"`.
- [ ] **`internal/core/embed_templates/skills/specd-execute/SKILL.md`**
      lines ~33-36, ~49 — update persona references, including `"A builder's
      word is not evidence"` → `"A craftsman's word is not evidence"`.
- [ ] **`internal/core/embed_templates/specStubs/tasks.md`** and
      **`specStubs/memory.md`** — update any `role:` field examples/guidance
      using old names.
- [ ] **`internal/core/embed_templates/steering/structure.md`**,
      **`steering/reasoning.md`**, **`steering/workflow.md`** — update
      persona-name prose, e.g. `"A builder's 'done' is NOT evidence"` →
      `"A craftsman's 'done' is NOT evidence"`; `"builder implements ONE
      task"` → `"craftsman implements ONE task"`. Read each surrounding
      paragraph before editing — some occurrences of "builder" in these
      files may be generic English rather than the persona name; only
      change ones that clearly refer to the specd role.
- [ ] Run `make build` to re-embed all changed templates (INV4 — `go:embed`
      is read-only at runtime, changes require a rebuild before any
      behavioral test).
- [ ] **Validation:**
      `ls internal/core/embed_templates/roles/` shows exactly `scout.md
      craftsman.md auditor.md validator.md brain.md pinky.md`.
      `ls internal/core/embed_templates/agents/` and `ls .claude/agents/`
      each show exactly `pinky-scout.md pinky-craftsman.md pinky-auditor.md
      pinky-validator.md`.
      For each of the 4 pinky-agent pairs: `diff internal/core/embed_templates/agents/pinky-<name>.md .claude/agents/pinky-<name>.md`
      reports no differences.
      `grep -rln "investigator\|builder\|reviewer\|verifier"
      internal/core/embed_templates/` — every remaining hit must be an
      intentional generic-English usage (verify each one by eye; there
      should be very few or none, since the legacy Go-registry aliasing
      means templates don't need to keep old-named copies around).

---

## Wave 5 — Role rename: documentation

Depends on Wave 4 (docs should describe the shipped template/CLI behavior).

- [ ] **`docs/agent-integration.md`** — update the role personas table
      (lines ~48-51): `investigator`→`scout`, `builder`→`craftsman`,
      `verifier`→`validator`, `reviewer`→`auditor` (keep the emoji/permission/
      responsibility columns, just swap the persona name in backticks). Also
      update the subagent-coordination prose if it names any of the four
      old roles elsewhere in the file (re-grep after the table edit).
- [ ] **`docs/user-guide.md`** — update the task-metadata table row
      `` | `role` | ✅ | Persona: `investigator`, `builder`, `reviewer`,
      `verifier` | `` (line ~319) to the new names, and both `role: builder`
      examples (lines ~333, ~344) to `role: craftsman`.
- [ ] **`docs/concepts.md`** — update the ASCII directory tree (lines
      ~154-155) from `investigator.md  builder.md  reviewer.md` /
      `verifier.md      brain.md    pinky.md` to `scout.md  craftsman.md
      auditor.md` / `validator.md    brain.md    pinky.md` (preserve the
      existing column alignment style).
- [ ] **`README.md`** — line ~171: `"Read-only roles (investigator/reviewer)
      whose `verify` is `N/A`..."` → `"Read-only roles (scout/auditor)
      whose..."`.
- [ ] **`docs/mcp-guide.md`** — line ~468: update the `` `role/builder` ``
      prompt-resource reference to `` `role/craftsman` `` (and check the
      surrounding paragraph for whether it should mention both the new
      resource and the retained legacy `role/builder` resource — if the doc
      lists all available prompt resources, list both).
- [ ] **`docs/validation-gates.md`** — line ~43: update persona reference to
      the new name.
- [ ] **`docs/custom-gates.md`** — line ~71: update the example payload's
      `"role"` value if it uses one of the four old names as an example.
- [ ] Leave **`docs/contributor-guide.md`** line ~31 unchanged — confirmed
      generic English ("spec builder"), not a persona reference.
- [ ] **Validation:** `grep -rniE "\binvestigator\b|\bbuilder\b|\breviewer\b|\bverifier\b"
      docs/ README.md` — every remaining hit must be either (a) an
      intentional "kept as legacy alias" mention you can point to in the
      spec, or (b) confirmed generic English unrelated to the persona
      system. There should be no un-reviewed hits.

---

## Wave 6 — Test alignment

Depends on Waves 1-5 (tests assert the shipped strings/behavior). Update
assertions to the new canonical names; where a test's whole purpose is
exercising back-compat, ADD a case rather than deleting the old-name
coverage.

**Mode tests:**
- [ ] `internal/cmd/mode_cmd_test.go` — update assertions expecting
      `"base"`/`ModeBase` output to expect `"simple"`/`ModeSimple`; add a
      case asserting `--set base` now fails with the new error message.
- [ ] `internal/core/mode_test.go` — update `ResolveMode`/`EffectiveMode`
      assertions; add a case: `state.ExecutionMode = "base"` (legacy,
      explicit) still resolves to `"simple"` via both functions.
- [ ] `internal/core/gates_mode_cov_test.go` — update any hardcoded `"base"`
      expectations.
- [ ] `internal/core/state_test.go`, `state_cas_test.go`,
      `state_resume_reject_test.go`, `program_state_test.go` — check for
      hardcoded `"base"`/`ModeBase` assertions; update. Add/confirm a
      round-trip byte-stability test per INV1: load a fixture `state.json`
      with `"executionMode":"base"`, don't mutate, re-save, assert byte-
      identical output.
- [ ] `internal/cmd/brain_unit_test.go`, `brain_pinky_test.go` — update the
      expected refusal-message text from "base execution mode" to "simple
      execution mode".

**Role tests:**
- [ ] `internal/spec/role_test.go`, `internal/spec/spec_test.go` — update
      assertions about `Roles`/`RoleNames()`/`RoleByName()` count and
      membership to include the 3 new canonical names + 3 new legacy
      entries; add explicit coverage that `RoleByName("builder")`,
      `RoleByName("reviewer")`, `RoleByName("verifier")` still return `ok ==
      true` with unchanged field values (legacy-alias contract).
- [ ] `internal/mcp/integration_test.go`, `internal/mcp/tools_test.go`,
      `internal/mcp/watcher_test.go` — update prompt-resource-name and
      `roleForStatus()` default-role assertions.
- [ ] `internal/context/manifest_test.go`, `manifest_extra_test.go`,
      `manifest_perf_gate_test.go`, `slice_test.go` — update
      `defaultContextRole()` assertions (`"builder"`→`"craftsman"`,
      `"verifier"`→`"validator"`).
- [ ] `internal/cmd/context_manifest_cmd_test.go` — update `defaultBriefRole`
      assertions in `internal/cmd/context.go`'s tests.
- [ ] `internal/cmd/init_test.go`, `initpack_test.go` — update assertions
      about which files `specd init` scaffolds (expect `scout.md`,
      `craftsman.md`, `auditor.md`, `validator.md`, `pinky-scout.md`, etc.,
      not the old names).
- [ ] `internal/cmd/watch_internal_test.go` — check for role-name literals;
      update if present.
- [ ] `internal/core/orchestration_authoring_test.go`,
      `orchestration_test.go`, `orchestration_decide_test.go`,
      `orchestration_driver_test.go`, `orchestration_validate_cov_test.go` —
      update `Role: "builder"` fixture expectations to `"craftsman"` where
      the test is about the authoring-stage default; leave any assertions
      that intentionally test the legacy-alias path pointing at `"builder"`.
- [ ] `internal/core/pinky_test.go`, `pinky_evidence_test.go`,
      `pinky_validate_cov_test.go`, `pinky_context_validate_cov_test.go` —
      update role fixtures as needed.
- [ ] `internal/core/program_orchestration_test.go`,
      `program_status_cov_test.go` — check and update role/mode literals.
- [ ] `internal/core/acp_test.go` — update `ACPMissionPayload.role` enum
      expectations to match the Wave 3 schema change.
- [ ] `internal/core/checkpoint_fault_test.go`, `customgate_test.go`,
      `cost_brake_test.go`, `session_replay_test.go`, `zero_cov_test.go` —
      grep each for role/mode literals; update only what's actually present
      (some of these may need zero changes — confirm, don't guess).
- [ ] `internal/core/tasksparser_test.go` — confirm `ValidRoles` (dynamic,
      from `spec.RoleNames()`) round-trip tests pass with the expanded
      12-entry registry; update any hardcoded expected-count assertions.
- [ ] `internal/cmd/commands_test.go` — update mode/role enum assertions
      (`specd help --json` schema shape) to match Wave 1/3's `commands.go`
      and schema changes.
- [ ] `internal/obs/log_test.go`, `log_cov_test.go` — grep for role/mode
      literals; expected to need no change per spec §5, confirm and leave
      untouched if so.
- [ ] `internal/worker/shell_runner_test.go` — grep for role/mode literals;
      update if present.
- [ ] **Validation:** `go test ./... -race -count=1` passes in full (this is
      `make test`'s underlying command — run `make test` directly).

---

## Wave 7 — Final validation, fresh-init verification, and sweep

Depends on Wave 6.

- [ ] Run `make ci` (lint + race test `-count=2` + coverage floor + stress).
      Must pass.
- [ ] In a scratch directory, run `specd init` fresh and confirm:
      `.specd/roles/` contains `scout.md craftsman.md auditor.md
      validator.md brain.md pinky.md`; `.claude/agents/` contains
      `pinky-scout.md pinky-craftsman.md pinky-auditor.md
      pinky-validator.md`.
- [ ] In the same scratch spec, hand-edit `state.json` to set
      `"executionMode": "base"`, then run `specd status <slug> --json` and
      confirm it reports `"mode": "simple"` and the file's other bytes are
      unaffected by the read.
- [ ] Create a `tasks.md` in the scratch spec using `role: scout` (or
      `craftsman`/`auditor`/`validator`) and confirm `specd check` passes.
- [ ] In a second scratch directory, hand-construct an "old-style" project
      (write only `investigator.md`/`builder.md`/`reviewer.md`/
      `verifier.md`/`brain.md`/`pinky.md` under `.specd/roles/`, skip the
      init scaffolder) and confirm the fusion/health check from Wave 3
      reports the `roles` check healthy (proves the OR-logic works both
      ways).
- [ ] Verify `specd help mode --json` and `specd help new --json` show
      `"simple"` in the mode enum.
- [ ] Verify `specd schema --version 1` output's three role enums each list
      all 12 role names (9 canonical + 3 legacy-of-the-renamed, matching
      `investigator` already being present as the 4th legacy).
- [ ] Final sweep — run all of:
      ```
      grep -rE "\bbase\b" --include="*.go" --include="*.md" --include="*.yml" --include="*.json" . \
        | grep -viE "database|based on|baseline|github"
      grep -rniE "investigator|builder|reviewer|verifier" docs/ README.md AGENTS.md \
        .claude/agents/ internal/core/embed_templates/
      ```
      Review every remaining hit by hand. Each must be traceable to either
      (a) an intentional legacy-alias/back-compat note already documented in
      this spec, or (b) confirmed generic English. If you find an un-reviewed
      hit, fix it and re-run.
- [ ] Update `specs/progress.md` (the multi-initiative tracker — append to
      it, do not overwrite the existing S1-S7 optimization-plan content) to
      mark this spec complete, with the before/after occurrence counts from
      the sweep above.

---

## Rollback

`git revert` of the wave commits, in reverse order, is sufficient — no data
migration was performed at any point (spec §9 D2, D5), so there is no
irreversible on-disk state to worry about.
