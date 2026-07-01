# Analysis and Action Plan: Rename Base Mode to Simple Mode and Rename Roles

## 1. Intent and Success

**Requested outcome:**
- Rename the "base" execution mode to "simple" mode across the entire codebase and documentation
- Rename the six role personas to: `scout` (was `investigator`), `craftsman` (was `builder`), `auditor` (was `reviewer`), `validator` (was `verifier`), keeping `brain` and `pinky` unchanged

**Task class:** Refactor (renaming with full consistency)

**Constraints:**
- Must preserve all existing behavior and CLI contracts
- Must maintain backward compatibility where possible (deprecated aliases, migration paths)
- Must update embedded templates (compiled into binary via `go:embed`)
- Must update all documentation, tests, and configuration schemas
- Must not break existing `state.json` files that may contain `"executionMode": "base"`

**Measurable definition of success:**
- `grep -r "\bbase\b" --include="*.go" --include="*.md" --include="*.yml" --include="*.json"` returns zero matches referring to the old mode name
- `specd mode <spec> --set simple` works and `specd mode <spec> --set base` fails with clear error
- All role files in `internal/core/embed_templates/roles/` use the new names (`scout.md`, `craftsman.md`, `auditor.md`, `validator.md`, `brain.md`, `pinky.md`)
- All tests pass: `make test` (race detector clean)
- Documentation is internally consistent

## 2. Scope and Decisions

**In scope:**
- Go source code: constants, variables, functions, comments, error messages
- Embedded templates: `internal/core/embed_templates/` (roles, config, AGENTS.md, spec stubs)
- Documentation: `docs/*.md`, `README.md`, `AGENTS.md`, `TESTING.md`
- `.claude/agents/` subagent definitions
- CLI command metadata and help text in `internal/core/commands.go`
- Test files and test fixtures
- Configuration schema and validation

**Out of scope:**
- External GitHub releases, changelog (unless user requests)
- Breaking changes without deprecation period (we'll provide aliases)
- Changes to the orchestration protocol logic beyond naming

**Assumptions:**
- A1: The user wants "simple" as the definitive new name for "base" mode
- A2: The user has selected the role names: `scout`, `craftsman`, `auditor`, `validator`, keeping `brain` and `pinky`
- A3: Existing `state.json` files with `"executionMode": "base"` should be transparently migrated on load

**Decision gates:**
- G1: ✅ User selected role names: `scout`, `craftsman`, `auditor`, `validator` (brain/pinky unchanged)
- G2: Whether to keep `base` as a deprecated alias for `simple` (recommended: yes, with warning)
- G3: Whether to rename `brain`/`pinky` orchestration terms (recommended: no, they are proper nouns with brand identity)

## 3. Repository Context

**Stack:** Go 1.22+, stdlib only, single static binary, `go:embed` for templates

**Architecture:** CLI-driven spec workflow harness with deterministic core and embedded templates

**Relevant flows and conventions:**
- `internal/core/state.go`: `ModeBase = "base"`, `ModeOrchestrated = "orchestrated"`, `EffectiveMode()` resolves empty string to `ModeBase`
- `internal/cmd/mode.go`: `runModeSet`, `runModeRecommend`, `printMode` — CLI surface for mode management
- `internal/cmd/new.go`: `--orchestrated` flag creates spec in orchestrated mode; default is base
- `internal/cmd/status.go`: `--set-mode` and `--recommend` paths
- `internal/cmd/brain.go`: `requireOrchestratedSpec` refuses Base specs with remediation message
- `internal/core/commands.go`: `CommandMeta` registry with `ModeCompatibility` enums
- `internal/core/embed_templates/`: `config.yml`, `config.json`, `AGENTS.md`, `roles/*.md`, `specStubs/`
- `.claude/agents/`: `pinky-{builder,investigator,reviewer,verifier}.md` reference role names
- `docs/agent-integration.md`: Role personas table and subagent coordination modes
- `docs/user-guide.md`: Task metadata `role:` key documentation
- `docs/command-reference.md`: `mode` command documentation
- `AGENTS.md` (root): Execution mode protocol and role adoption rules

**Analogous implementations:** The `mode` command was already merged into `status` as a survivor home (deprecated in v0.2.0); this shows the project's pattern for deprecation

**Tests/build/deployment:**
- `make test` runs race detector suite
- `make ci` runs full gate including lint, race, count=2, coverage floor, stress
- Templates are compiled into binary; changes require rebuild before test

**Invariants:**
- INV1: `state.json` byte-stability for Base specs (empty `executionMode` means Base)
- INV2: `EffectiveMode()` is the single resolution point for mode
- INV3: Role names are referenced in `tasks.md` metadata, `dispatch` packets, and `contextManifest`
- INV4: `go:embed` templates are read-only at runtime; mutations require rebuild

## 4. Requirements and Evidence

| ID | Requirement or Fact | Evidence/Source | Priority | Acceptance Signal |
|----|---------------------|-----------------|----------|-------------------|
| R1 | Rename "base" mode to "simple" mode in all Go source | `internal/core/state.go` lines with `ModeBase`, `internal/cmd/mode.go`, `internal/cmd/new.go`, `internal/cmd/status.go`, `internal/cmd/brain.go` | Must | `grep -r "ModeBase\|"base"\|base mode" internal/` returns only false positives |
| R2 | Update embedded templates to use "simple" | `internal/core/embed_templates/config.yml`, `config.json`, `AGENTS.md`, `roles/*.md`, `specStubs/` | Must | Templates contain "simple" not "base" for mode references |
| R3 | Update all documentation | `docs/concepts.md`, `docs/user-guide.md`, `docs/agent-integration.md`, `docs/command-reference.md`, `README.md`, `AGENTS.md` | Must | Docs internally consistent; no "base mode" references |
| R4 | Update `.claude/agents/` subagent definitions | `.claude/agents/pinky-*.md` | Must | Agent defs reference new role names |
| R5 | Update CLI command metadata and enums | `internal/core/commands.go` `annotateFlagEnums` and `ModeCompatibilityMeta` | Must | `specd help mode --json` shows "simple" enum |
| R6 | Rename role personas to `scout`, `craftsman`, `auditor`, `validator` | `internal/core/embed_templates/roles/*.md`, `docs/agent-integration.md` Role personas table, `docs/user-guide.md` task metadata | Must | Role files renamed; docs updated |
| R7 | Update dispatch and context manifest logic | `internal/cmd/dispatch.go`, `internal/cmd/next.go`, `internal/core/` context engine | Must | Dispatch packets emit correct role names |
| R8 | Update test files and fixtures | `*_test.go` files across `internal/cmd/` and `internal/core/` | Must | `make test` passes |
| R9 | Provide backward compatibility for existing state.json | `internal/core/state.go` `migrate()` function | Should | Old `"executionMode": "base"` loads and maps to "simple" |
| R10 | Update validation gates and task schema | `internal/cmd/check.go`, task parser, schema | Must | `specd check` passes on specs with new role names |

## 5. Findings and Impact

| ID | Finding/Constraint | Evidence | Impact | Recommendation |
|----|--------------------|----------|--------|----------------|
| F1 | `ModeBase` constant is central; changing it touches ~15+ Go files | `internal/core/state.go`, `internal/cmd/mode.go`, `internal/cmd/new.go`, `internal/cmd/status.go`, `internal/cmd/brain.go`, `internal/core/commands.go` | High | Systematic find/replace with review; add `ModeSimple` alias, deprecate `ModeBase` |
| F2 | `EffectiveMode()` treats empty string as Base; changing default semantics risks state.json compatibility | `internal/core/state.go` `EffectiveMode()` | High | Keep `EffectiveMode()` logic but map to `"simple"`; add migration for explicit `"base"` values |
| F3 | `go:embed` templates are compiled into binary | `internal/core/embed.go` | Medium | Must rebuild binary after template changes; no runtime template mutation |
| F4 | Role names appear in `tasks.md` metadata, dispatch JSON, and context manifests | `docs/user-guide.md` task metadata table, `internal/cmd/dispatch.go` | High | Role rename is a breaking change for existing `tasks.md` files; need migration or accept breakage |
| F5 | `.claude/agents/` files reference role names in their `name:` frontmatter and body | `.claude/agents/pinky-builder.md` etc. | Medium | Update frontmatter and all internal references |
| F6 | `brain` and `pinky` are proper nouns for the orchestration architecture | `docs/agent-integration.md`, `docs/concepts.md` | Low | Keep as-is; they have brand identity and are not generic roles |
| F7 | `specd mode` command is deprecated (merged into `status`) but still present | `docs/command-reference.md` migration appendix, `internal/cmd/mode.go` | Medium | Update deprecated command too for consistency, or remove it entirely per deprecation schedule |
| F8 | `state.json` schema version is 5; migration function exists | `internal/core/state.go` `SchemaVersion = 5`, `migrate()` | Low | Bump schema version to 6, add migration rule for `executionMode` |

## 6. Implementation Vision

**Design direction:**
- Rename "base" → "simple" as a systematic identifier replacement
- Rename roles: `investigator`→`scout`, `builder`→`craftsman`, `reviewer`→`auditor`, `verifier`→`validator`
- Keep `brain`/`pinky` orchestration terms unchanged
- Provide a one-time migration path for existing `state.json` files

**Affected modules:**
- `internal/core/state.go` — constants, `EffectiveMode()`, migration
- `internal/cmd/mode.go` — CLI validation, output text
- `internal/cmd/new.go` — default mode logic
- `internal/cmd/status.go` — mode reporting
- `internal/cmd/brain.go` — orchestration refusal message
- `internal/core/commands.go` — command metadata enums
- `internal/core/embed_templates/` — all templates
- `docs/` — all documentation
- `.claude/agents/` — subagent definitions

**Interfaces/contracts:**
- `state.json` schema: `executionMode` field values change from `"base"|"orchestrated"` to `"simple"|"orchestrated"`
- CLI: `--set base` → `--set simple`; old `--set base` gives deprecation warning or error
- Task metadata `role:` key values change to new role names
- Dispatch packet `role` field changes

**Data/configuration:**
- `config.yml`/`config.json`: `subagent_mode` values unaffected (inline/delegate)
- `state.json`: migration on load for explicit `"base"` values

**Security/failures:**
- No security impact
- Risk: breaking existing automation that parses `specd mode --json` output
- Mitigation: bump schema version, document in release notes

**Observability:**
- Update any mode-related log messages and error strings

**Compatibility:**
- Existing `state.json` with `"executionMode": "base"` → migrate to `"simple"` on load
- Existing `state.json` with empty `executionMode` → continue to mean simple (no change needed)
- Existing `tasks.md` with old role names → **breaking change**; user must update or we provide a migration command

**Rollout:**
- Phase 1: Code changes + template updates
- Phase 2: Test updates + `make test`
- Phase 3: Documentation updates
- Phase 4: Validation with `make ci`

**Rollback:**
- Git revert; since this is a rename-only change, rollback is straightforward

## 7. Specification Map

| Spec | Responsibility | Requirements | Dependencies | Validation |
|------|----------------|--------------|--------------|------------|
| S1: Mode rename core | Rename constants, `EffectiveMode()`, CLI commands | R1, R9 | None | `make test`, `grep` |
| S2: Mode rename templates | Update embedded templates | R2 | S1 | `make test`, template readback |
| S3: Mode rename docs | Update all Markdown documentation | R3 | S1, S2 | Manual review, `grep` |
| S4: Role rename core | Rename role files, update dispatch/context logic | R6, R7 | G1 (user decision) ✅ | `make test`, `specd check` |
| S5: Role rename templates | Update role templates, `.claude/agents/` | R4, R6 | S4, G1 ✅ | `make test` |
| S6: Role rename docs | Update docs with new role names | R6 | S4, S5 | Manual review |
| S7: Test alignment | Update all test assertions and fixtures | R8 | S1, S4 | `make test` |
| S8: Schema migration | Bump schema version, add state.json migration | R9, R10 | S1 | `make test`, migration test |

## 8. Execution and Validation Plan

**Phase 1 — Core mode rename (parallel with user role selection)**
1. Update `internal/core/state.go`: `ModeSimple = "simple"`, deprecate `ModeBase`, update `EffectiveMode()`, add migration in `migrate()` for schema version 6
2. Update `internal/cmd/mode.go`: validation strings, output text
3. Update `internal/cmd/new.go`: default mode references
4. Update `internal/cmd/status.go`: mode reporting
5. Update `internal/cmd/brain.go`: refusal message
6. Update `internal/core/commands.go`: flag enums, mode compatibility
7. Run `make test` — expect failures in tests with hardcoded "base"

**Phase 2 — Template and documentation mode rename**
8. Update `internal/core/embed_templates/config.yml` and `config.json`
9. Update `internal/core/embed_templates/AGENTS.md`
10. Update `docs/concepts.md`, `docs/user-guide.md`, `docs/agent-integration.md`, `docs/command-reference.md`
11. Update `README.md`, `AGENTS.md`
12. Run `make build` to re-embed templates

**Phase 3 — Role rename (blocked on G1: user selected names)**
13. Rename role files in `internal/core/embed_templates/roles/`: `investigator.md`→`scout.md`, `builder.md`→`craftsman.md`, `reviewer.md`→`auditor.md`, `verifier.md`→`validator.md`
14. Update role content (frontmatter, body, structured result blocks)
15. Update `.claude/agents/` files: `pinky-investigator.md`→`pinky-scout.md`, `pinky-builder.md`→`pinky-craftsman.md`, `pinky-reviewer.md`→`pinky-auditor.md`, `pinky-verifier.md`→`pinky-validator.md`
16. Update `internal/cmd/dispatch.go`, `internal/cmd/next.go` role references
17. Update `docs/agent-integration.md` role personas table
18. Update `docs/user-guide.md` task metadata documentation
19. Update `internal/core/commands.go` if roles appear in enums
20. Run `make test`

**Phase 4 — Test alignment and final validation**
21. Update all `*_test.go` files with new strings
22. Run `make test` — must pass
23. Run `make ci` — must pass
24. Run `specd check` on a test spec with new role names
25. Verify `specd mode test-spec --json` outputs `"mode": "simple"`

**Rollback checkpoints:**
- After Phase 1: `git diff --stat` should show ~8 Go files changed
- After Phase 2: `grep -r "base mode" docs/` should return empty
- After Phase 3: `ls internal/core/embed_templates/roles/` should show `scout.md`, `craftsman.md`, `auditor.md`, `validator.md`, `brain.md`, `pinky.md`

## 9. Risks and Unknowns

| ID | Risk/Unknown | Consequence | Resolution/Mitigation | Gate |
|----|--------------|-------------|-----------------------|------|
| U1 | Existing `tasks.md` files in user repos will break | External users' specs fail validation | Document as breaking change; provide `specd migrate roles` command or manual migration guide | G1 ✅ |
| U2 | `go:embed` template changes require rebuild | Developers may test with stale binary | Add build step to task list; verify with `go run .` | Phase 2 |
| U3 | `state.json` migration may miss edge cases | Corrupt state on load | Add test coverage for migration path; test with old state files | Phase 4 |
| U4 | "Simple" may conflict with other terminology | Confusion in docs | Audit all "simple" occurrences to ensure they refer to mode | Phase 2 |
| U5 | Role rename breaks dispatch packet consumers | External orchestration tools fail | Document breaking change; role names are part of public API | Phase 3 |

## 10. Definition of Done

- [ ] `specd mode <spec> --set simple` succeeds and persists `"executionMode": "simple"`
- [ ] `specd mode <spec> --set base` fails with clear error (or deprecation warning)
- [ ] `specd new <spec>` creates spec with default mode "simple" (empty field, resolved correctly)
- [ ] `specd brain run <spec>` on a simple-mode spec shows updated refusal message
- [ ] All role files in `internal/core/embed_templates/roles/` use the new names: `scout.md`, `craftsman.md`, `auditor.md`, `validator.md`, `brain.md`, `pinky.md`
- [ ] All `.claude/agents/` files use the new role names: `pinky-scout.md`, `pinky-craftsman.md`, `pinky-auditor.md`, `pinky-validator.md`
- [ ] All documentation is internally consistent with no "base mode" or old role name references
- [ ] `make test` passes with race detector
- [ ] `make ci` passes full gate
- [ ] Old `state.json` with `"executionMode": "base"` loads and migrates to `"simple"` transparently
- [ ] `specd check` passes on a spec using new role names in `tasks.md`

## 11. Traceability

| Requirement | Evidence | Spec | Validation | Done Criterion |
|-------------|----------|------|------------|----------------|
| R1 | `internal/core/state.go` `ModeBase` | S1 | `make test`, `grep` | `specd mode --set simple` works |
| R2 | `internal/core/embed_templates/` | S2 | Template readback | Templates contain "simple" |
| R3 | `docs/*.md`, `README.md` | S3 | Manual review | No "base mode" in docs |
| R4 | `.claude/agents/` | S5 | `make test` | Agent defs updated |
| R5 | `internal/core/commands.go` | S1 | `specd help --json` | Enum shows "simple" |
| R6 | `internal/core/embed_templates/roles/` | S4, S5, S6 | `make test`, `specd check` | Role files renamed |
| R7 | `internal/cmd/dispatch.go`, `next.go` | S4 | `make test` | Dispatch packets correct |
| R8 | `*_test.go` | S7 | `make test` | Tests pass |
| R9 | `internal/core/state.go` `migrate()` | S8 | Migration test | Old state loads correctly |
| R10 | `internal/cmd/check.go` | S8 | `specd check` | Gate passes |
