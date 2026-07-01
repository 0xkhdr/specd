# Action Prompt: Rename Base Mode to Simple Mode and Rename Roles

**Analysis plan:** `rename-base-mode-and-roles-analysis-plan.md`

**Confirmed role names:**
- `investigator` → `scout`
- `builder` → `craftsman`
- `reviewer` → `auditor`
- `verifier` → `validator`
- `brain` → `brain` (unchanged)
- `pinky` → `pinky` (unchanged)

You are a coding agent tasked with implementing the rename of "base" mode to "simple" mode and renaming the role personas in the `specd` repository. Read the analysis plan first, then re-inspect the live repository to validate all assumptions before creating specs and tasks.

## Your Mission

1. **Read** the repository instructions (`AGENTS.md`) and the analysis plan.
2. **Re-inspect** the live repository and validate all material assumptions. Record discrepancies and resolve them from repository evidence.
3. **Treat the analysis as intent guidance**, not unquestionable implementation truth.
4. **Build your own implementation vision** before creating specs.
5. **Create the spec structure** below covering 100% of requirements.
6. **Stop before production implementation** unless the user separately requests execution.

## Spec Structure

Create exactly this structure:

```text
specs/
├── progress.md
└── rename-base-mode-and-roles/
    ├── spec.md
    └── tasks.md
```

## Required Spec Content

### `specs/rename-base-mode-and-roles/spec.md`

Must contain:
- **Purpose and requirement coverage:** Maps to R1-R10 from the analysis plan
- **Verified current state:** Exact repository references for `ModeBase`, role files, documentation locations
- **Proposed design and end-to-end flow:** How the rename propagates from constants → CLI → templates → docs → tests
- **Interfaces, contracts, data, configuration, dependencies:**
  - `state.json` schema change (version 5 → 6)
  - CLI enum changes (`--set simple`, role names in task metadata)
  - Template file renames: `investigator.md`→`scout.md`, `builder.md`→`craftsman.md`, `reviewer.md`→`auditor.md`, `verifier.md`→`validator.md`
  - Dispatch packet `role` field values
- **Invariants, security, errors, observability, compatibility, rollback:**
  - INV1: `state.json` byte-stability for default mode (empty = simple)
  - INV2: `EffectiveMode()` remains single resolution point
  - Backward compatibility: migration for explicit `"base"` values
  - Rollback: git revert of the rename commit
- **Acceptance criteria and validation commands:**
  - `make test` passes
  - `make ci` passes
  - `grep -r "ModeBase\|"base".*mode\|base execution" --include="*.go" --include="*.md"` returns only historical/deprecated references
  - `specd mode test-spec --set simple` succeeds
  - Old `state.json` with `"executionMode": "base"` loads and reports `"mode": "simple"`
- **Open decisions and deviations:** Document any deviation from the analysis plan

### `specs/rename-base-mode-and-roles/tasks.md`

Must organize work into **dependency-aware waves** with atomic, actionable checkboxes.

**Wave 1 — Foundation (mode rename core)**
- [ ] Update `internal/core/state.go`: add `ModeSimple = "simple"`, mark `ModeBase` deprecated, update `EffectiveMode()`, bump `SchemaVersion` to 6, add migration rule for `"executionMode": "base"` → `"simple"`
- [ ] Update `internal/cmd/mode.go`: replace validation strings, output text, error messages ("base" → "simple")
- [ ] Update `internal/cmd/new.go`: update default mode references
- [ ] Update `internal/cmd/status.go`: update mode reporting strings
- [ ] Update `internal/cmd/brain.go`: update orchestration refusal message
- [ ] Update `internal/core/commands.go`: update `ModeCompatibility` enums, `annotateFlagEnums` for `--set` flag
- [ ] Run `make build` to verify compilation
- [ ] Validation: `go test ./internal/core/... ./internal/cmd/...` (expect test failures from hardcoded strings)

**Wave 2 — Templates and docs (mode rename)**
- [ ] Update `internal/core/embed_templates/config.yml`: replace "base" mode references with "simple"
- [ ] Update `internal/core/embed_templates/config.json`: replace "base" mode references with "simple"
- [ ] Update `internal/core/embed_templates/AGENTS.md`: replace "base" mode references with "simple"
- [ ] Update `docs/concepts.md`: replace "base" mode references with "simple"
- [ ] Update `docs/user-guide.md`: replace "base" mode references with "simple"
- [ ] Update `docs/agent-integration.md`: replace "base" mode references with "simple"
- [ ] Update `docs/command-reference.md`: replace "base" mode references with "simple"
- [ ] Update `README.md`: replace "base" mode references with "simple"
- [ ] Update `AGENTS.md` (root): replace "base" mode references with "simple"
- [ ] Validation: `grep -r "base mode\|base execution\|ModeBase" docs/ README.md AGENTS.md internal/core/embed_templates/`

**Wave 3 — Role rename core**
- [ ] Rename `internal/core/embed_templates/roles/investigator.md` → `scout.md`
- [ ] Rename `internal/core/embed_templates/roles/builder.md` → `craftsman.md`
- [ ] Rename `internal/core/embed_templates/roles/reviewer.md` → `auditor.md`
- [ ] Rename `internal/core/embed_templates/roles/verifier.md` → `validator.md`
- [ ] Update `scout.md` content: title, structured result block `role: scout`, all internal references
- [ ] Update `craftsman.md` content: title, structured result block `role: craftsman`, all internal references
- [ ] Update `auditor.md` content: title, structured result block `role: auditor`, all internal references
- [ ] Update `validator.md` content: title, structured result block `role: validator`, all internal references
- [ ] Update `internal/core/embed_templates/roles/brain.md` if it references other roles
- [ ] Update `internal/core/embed_templates/roles/pinky.md` if it references other roles
- [ ] Update `internal/cmd/dispatch.go`: role name references in dispatch packets
- [ ] Update `internal/cmd/next.go`: role name references in task output
- [ ] Update `internal/core/commands.go`: any role name enums or defaults
- [ ] Validation: `ls internal/core/embed_templates/roles/` shows `scout.md`, `craftsman.md`, `auditor.md`, `validator.md`, `brain.md`, `pinky.md`

**Wave 4 — Agents and docs (role rename)**
- [ ] Rename `.claude/agents/pinky-investigator.md` → `.claude/agents/pinky-scout.md`
- [ ] Rename `.claude/agents/pinky-builder.md` → `.claude/agents/pinky-craftsman.md`
- [ ] Rename `.claude/agents/pinky-reviewer.md` → `.claude/agents/pinky-auditor.md`
- [ ] Rename `.claude/agents/pinky-verifier.md` → `.claude/agents/pinky-validator.md`
- [ ] Update `pinky-scout.md` content: frontmatter `name: pinky-scout`, body references
- [ ] Update `pinky-craftsman.md` content: frontmatter `name: pinky-craftsman`, body references
- [ ] Update `pinky-auditor.md` content: frontmatter `name: pinky-auditor`, body references
- [ ] Update `pinky-validator.md` content: frontmatter `name: pinky-validator`, body references
- [ ] Update `docs/agent-integration.md`: Role personas table (`investigator`→`scout`, `builder`→`craftsman`, `reviewer`→`auditor`, `verifier`→`validator`)
- [ ] Update `docs/agent-integration.md`: Subagent coordination text referencing roles
- [ ] Update `docs/user-guide.md`: Task metadata `role:` key documentation
- [ ] Update `docs/user-guide.md`: Task examples with new role names
- [ ] Update `AGENTS.md` (root): Role adoption rules
- [ ] Validation: `grep -r "investigator\|builder\|reviewer\|verifier" .claude/agents/ docs/ AGENTS.md` returns only brain/pinky references

**Wave 5 — Test alignment**
- [ ] Update `internal/cmd/mode_cmd_test.go` or equivalent mode tests
- [ ] Update `internal/cmd/brain_unit_test.go` or equivalent brain tests
- [ ] Update `internal/cmd/brain_pinky_test.go` or equivalent tests
- [ ] Update `internal/cmd/dispatch_test.go` or equivalent dispatch tests
- [ ] Update `internal/cmd/next_test.go` or equivalent next tests
- [ ] Update `internal/cmd/status_test.go` or equivalent status tests
- [ ] Update `internal/core/state_test.go` or equivalent state tests
- [ ] Update any other test files with hardcoded "base" or old role names
- [ ] Validation: `make test` passes

**Wave 6 — Final validation and migration**
- [ ] Run `make ci` (full gate: lint + race test + count=2 + coverage floor + stress)
- [ ] Create test `state.json` with `"executionMode": "base"` and verify it loads as "simple"
- [ ] Create test spec with new role names in `tasks.md` and verify `specd check` passes
- [ ] Verify `specd help --json` shows updated enums
- [ ] Verify `specd mode test-spec --json` outputs `"mode": "simple"`
- [ ] Final `grep` sweep: `grep -rE "\bbase\b" --include="*.go" --include="*.md" --include="*.yml" --include="*.json" | grep -v "database\|based on\|github"` should be minimal

## `specs/progress.md`

Must track:
- Overall status and current wave
- Requirement-to-spec coverage (R1-R10)
- Spec status, dependencies, blockers, and validation
- Decision gate status: G1 ✅ User selected `scout`, `craftsman`, `auditor`, `validator`
- Baselines/targets: `make test` must pass, `make ci` must pass
- Completed and remaining waves

## Coding-Agent Rules

- **Read before writing:** Inspect every file you plan to change before modifying it
- **Preserve repository conventions:** Follow Go naming, comment style, error message patterns
- **Validate each wave before advancing:** Run `make test` after each wave; do not proceed if tests fail
- **Add no orphaned code:** Every changed string must have a purpose; remove old references
- **Include migrations:** `state.json` schema version bump + migration rule for old `"base"` values
- **Measure before and after:** Count occurrences of "base" and old role names before/after
- **Never hide uncertainty:** If a file's purpose is unclear, inspect it fully before changing
- **Split specs by capability:** This is one coherent rename spec, not multiple
- **Keep tasks comprehensive:** Tasks must be implementable without chat history

## Verification of 100% Requirement Coverage

Before marking the spec complete, verify:
- [ ] Every requirement (R1-R10) has at least one task checkbox
- [ ] Every task checkbox names the likely file(s) to modify
- [ ] Every wave includes validation commands
- [ ] The migration path (R9) is explicitly tested
- [ ] Documentation updates (R3) cover all `.md` files in the repo root and `docs/`

## Final Instruction

Do not begin implementation until:
1. ✅ The user has selected their preferred role names (DONE: `scout`, `craftsman`, `auditor`, `validator`)
2. You have created the spec structure above with the selected names
3. The user has approved the spec for execution

Your first action is to create the spec structure, then present it to the user for approval before proceeding with implementation.
