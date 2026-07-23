# Design — workflow-11-template-config

- references: R1,R1.1,R1.2,R2,R2.1,R2.2,R2.3,R3,R3.1,R4,R4.1,R4.2,R5,R5.1,R5.2,R6,R6.1,R6.2
- disposition: accepted
- owner: project maintainers

## Boundaries

- Embedded steering, requirements, tasks, role, and configuration templates own examples and applicability metadata; existing consumers remain authoritative.
- `internal/context` owns steering selection and manifest projection; the existing warning gate owns total steering omission.
- `internal/core` owns requirement parsing, task quality declarations, configuration parsing, scaffold/doctor layout checks, and palette metadata.
- `internal/cmd` owns `new` flag consumption and typed eval-import refusals.
- Excluded: new artifact grammars, new configuration keys unless required to wire the existing agent flag, relaxed path containment, runtime dependencies, and state migrations.

## Interfaces

- Every shipped steering template carries the `specd-context` metadata already consumed by steering selection; total omission remains a warning finding.
- Requirements and tasks scaffolds use IDs and evidence values accepted by their existing parsers and gates; example commands stay within the invoking role's palette.
- Init and doctor agree on required layout and recovery: the named repair action creates every required path, using a tracked keep file only where an empty directory cannot be represented.
- All configuration list keys accept one documented separator policy, and unquoted trailing `#` comments are handled consistently with the shipped template.
- The `new` command either validates and persists the declared agent selection through existing configuration/state ownership or removes the no-op flag and examples from the canonical palette.
- Palette/handler conformance fails when a declared flag is never consumed.
- Eval import rejects absolute artifact paths with a typed refusal containing the offending path, the workspace-relative requirement, and a relative recovery example.

## Invariants

- Templates are proven against the same production parsers and gates that consume generated projects.
- Configuration parsing is deterministic, byte-local, and implemented with the standard library.
- Palette metadata remains the single source for flags, usage, examples, and generated command documentation.
- Init repair is idempotent and does not overwrite populated user files.
- Absolute artifact paths never pass containment checks or become stored eval records.
- No task completion, evidence, atomic-write, CAS, lock, or byte-stable task-parser behavior changes.

## Failure

- Missing steering applicability produces stable omissions; omission of every steering file produces a warning with the metadata remedy.
- Empty requirement ID sets are attributed to `requirements.md`; unknown task references remain attributed to `tasks.md` only when IDs were parsed successfully.
- Invalid quality declarations and role-forbidden examples fail template conformance tests before release.
- Conflicting or malformed configuration returns the existing typed diagnostics after comment/list normalization.
- Unsupported or unconsumed flags exit 2 instead of silently succeeding.
- Absolute eval paths return the typed refusal without writing state and name both the path and legal relative form.

## Integration

- Extend existing template-conformance, scaffold, context, configuration, palette, doctor, and eval tests rather than creating parallel harnesses.
- Generated command reference is refreshed only if canonical palette flags or examples change.
- Fresh-init integration tests exercise init, new, context, check, and quality gates in a throwaway Git repository.
- Existing legacy configuration source precedence and migration behavior remain compatible.

## Alternatives

- Loosen consumers to accept every current template inconsistency: rejected; aligning shipped examples to canonical grammars is smaller and preserves fail-closed gates.
- Add a YAML dependency for comments and lists: rejected; the supported flat configuration grammar needs only the existing standard-library parser.
- Keep the agent flag as reserved future surface: rejected; a declared no-op violates palette truthfulness.
- Permit absolute eval paths after normalization: rejected; workspace containment is an explicit invariant.

## Verification

- Steering manifest tests prove shipped templates are selected and total omission warns.
- Template conformance tests prove scaffolded requirement IDs, quality evidence, and role commands pass their production consumers.
- Init/doctor tests prove named recovery creates or correctly reclassifies required layout without destructive overwrite.
- Table tests prove every list key handles the documented separators and inline-comment cases identically.
- Palette tests prove each declared flag has a handler consumer; `new` tests prove the agent flag is wired or absent everywhere.
- Eval tests prove typed absolute-path refusal text, offending-path projection, relative recovery examples, and zero mutation.
- Full race tests, docs lint, and domain regressions prove cross-layer compatibility.

## Deployment

- Template and parser corrections activate on new scaffolds immediately; existing valid projects need no migration.
- Observe warning/refusal codes and fresh-project conformance through the regression suite.

## Rollback

- Revert the task commits if existing valid configuration or generated-project compatibility regresses.
- No state rollback or dependency removal is required.
