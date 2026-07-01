# Action Prompt: specd v0.1.0 Development Release — Deprecation Cleanup & Hardening

## Agent Mandate

You are a coding agent tasked with preparing the `specd` repository for its **first development release (v0.1.0)**. Your job is to eliminate all deprecated code, commands, documentation, and migration paths, align all documentation with the v0.1.0 release, and harden the codebase for public use. **Do not implement new features.** Stop before production merge unless the user separately requests execution.

**Critical version constraint:** This release is **v0.1.0**, not v1.0.0. The project is in active development. All documentation, examples, and version references must use v0.1.0. Remove all references to v0.2.0, v0.3.0, v1.0.0, and any "pre-1.0" or "pre-release" language.

## Pre-Flight Checklist (read before writing)

1. Read the analysis plan: `specd-v0.1.0-release-analysis-plan.md`.
2. Re-inspect the live repository and validate all material assumptions from the analysis.
3. Treat the analysis as intent guidance, not unquestionable truth. Record discrepancies and resolve them from repository evidence.
4. Build your own implementation vision before creating specs.
5. **Confirm version numbering:** All changes must align with v0.1.0. No v1.0.0 references should survive.

## Required Output Structure

Create this directory tree under `specs/`:

```
specs/
├── progress.md
├── s1-deprecation-cleanup-commands/
│   ├── spec.md
│   └── tasks.md
├── s2-deprecation-cleanup-config-migration/
│   ├── spec.md
│   └── tasks.md
├── s3-deprecation-cleanup-scripts/
│   ├── spec.md
│   └── tasks.md
├── s4-docs-alignment-root/
│   ├── spec.md
│   └── tasks.md
├── s5-docs-alignment-guides/
│   ├── spec.md
│   └── tasks.md
└── s6-hardening-ci-validation/
    ├── spec.md
    └── tasks.md
```

## Spec Requirements

Every `spec.md` must contain:
- **Purpose and requirement coverage** — which R# requirements this spec addresses.
- **Verified current state** — exact files, symbols, and line ranges observed in the live repo.
- **Proposed design and end-to-end flow** — what changes and in what order.
- **Interfaces, contracts, data, configuration, dependencies** — what must stay intact.
- **Invariants, security, errors, observability, compatibility, rollback** — risks and mitigations.
- **Acceptance criteria and validation commands** — exact commands to run and expected output.
- **Open decisions and deviations** — anything that diverges from the analysis plan.
- **Version alignment checklist** — specific verification that all version references in the spec's scope use v0.1.0.

Every `tasks.md` must contain:
- **Dependency-aware waves** — Wave 1 has no deps; later waves depend on earlier ones.
- **Atomic, actionable checkboxes** — each task is a single file edit or verification step.
- **Likely files/modules** — named without inventing unverified paths.
- **Tests and validation in each wave** — never a wave without a verification step.
- **Setup/baseline, core changes, integration, regression, rollout, cleanup** — as applicable.
- **Dependencies, completion evidence, rollback considerations** — for each task.
- **Implementation-ready detail** — a junior engineer could execute from the task list alone.
- **Version alignment tasks** — explicit tasks to update version references to v0.1.0.

## Progress Tracking

`specs/progress.md` must track:
- Overall status and current wave.
- Requirement-to-spec coverage matrix (R1→S1, R2→S1, etc.).
- Spec status, dependencies, blockers, and validation.
- **Version alignment status** — which files have been verified for v0.1.0 references.
- Decisions and deviations from the analysis plan.
- Completed and remaining waves.

## Coding-Agent Rules

- **Read before writing.** Never edit a file you haven't read.
- **Preserve repository conventions and invariants.** Zero runtime deps, atomic writes, CAS, exit codes, round-trip stability.
- **Validate each wave before advancing.** Run the validation commands in the spec before starting the next wave.
- **Add no orphaned code or undocumented configuration.** If you delete a command, delete its metadata, tests, and doc references.
- **Include migrations, compatibility, observability, security, and rollback where relevant.** Here, "migration" means documenting breaking changes in release notes.
- **Never hide uncertainty or silently diverge from the plan.** Document every deviation in `progress.md`.
- **Split specs by coherent capability, not arbitrary file count.** S1 covers all command deprecation; S4 covers all root docs.
- **Keep tasks comprehensive enough for implementation without requiring chat history.**
- **Version alignment is mandatory.** Every doc change must include verification that version references are v0.1.0.

## Verification of 100% Requirement Coverage

Before declaring done, the agent must:

1. **Deprecated surface removal:**
   ```bash
   grep -r 'legacyAlias\|DeprecatedIn\|doctor\|dispatch\|program\|validate\|schema\|replay\|diff\|serve\|watch\|mode\|migrate\|update\|uninstall' internal/ scripts/ docs/ README.md AGENTS.md TESTING.md SECURITY.md
   ```
   Confirm no deprecated surface remains (except legitimate references like "install script" or "update your shell").

2. **Old version reference removal:**
   ```bash
   grep -r 'v1\.0\.0\|v0\.2\.0\|v0\.3\.0' docs/ README.md AGENTS.md TESTING.md SECURITY.md
   ```
   Confirm no old version references remain (except historical changelog if any).

3. **v0.1.0 alignment:**
   ```bash
   grep -r '\-\-version' docs/ README.md AGENTS.md TESTING.md SECURITY.md
   ```
   Confirm all install examples reference `0.1.0`.

4. **Build and test:**
   - `make ci` passes on Linux and macOS.
   - `GOOS=windows go build` succeeds.

5. **Dependency verification:**
   - `go mod graph | grep -v 'std'` confirms zero external dependencies.
   - `go.mod` has no `require` block.

6. **Release artifact verification:**
   - `.goreleaser.yml` still references `SHA256SUMS`.
   - `scripts/install.sh` still verifies checksums correctly.

7. **Update `progress.md`** with final status, version alignment verification, and any deviations.

## Stop Condition

Stop after all specs and tasks are written and validated. Do not open a PR, do not tag a release, and do not push to `main` unless the user explicitly requests execution.

**Final verification before stopping:** Confirm that the codebase is clean of deprecated code, all docs reference v0.1.0, and the repository is ready for the `v0.1.0` tag.
