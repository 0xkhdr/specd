# Stage 2 — Role System Redesign

## Goal
Replace the flat read-only role list with explicit role contracts and role-aware tool gating.

## Knowledge gathered
- `internal/spec/role.go` currently exposes only `ReadonlyRoles = [investigator, reviewer]`.
- `internal/mcp/prompts.go` only registers `role/builder` and `role/investigator` prompts.
- `internal/mcp/tools.go` filters tools by phase only; role constraints are not part of the allow-set.
- `internal/mcp/watcher.go` tracks active phase globally and does not consider role.
- `internal/context/manifest_types.go` does not carry role metadata yet.
- TASKS.md defines the target role set: scout, researcher, reviewer, architect, builder, tester, documenter, verifier.

## Frozen scope
- Introduce structured role definitions.
- Add prompts for the new roles while keeping builder/investigator backward compatible.
- Make tool exposure the intersection of phase-allowed and role-allowed tools.
- Thread role into manifest filtering and the watcher.
- Isolate verifier capabilities from the rest of the roles.

## Requirements
1. Role registry must be structured, not a flat string slice.
2. Role metadata must include rw, budget, phase affinity, tools, file policy, and prompt class.
3. New prompts must exist for scout, researcher, reviewer, architect, tester, documenter, and verifier.
4. Tool lists must be the intersection of phase permissions and role permissions.
5. Context manifests must record the active role and use it for filtering.
6. The phase watcher must consider the active role when building the live tool list.
7. Legacy builder/investigator prompts must remain until a later deprecation cycle.

## Non-goals
- No semantic change to phase ordering.
- No source-file writes from read-only roles.
- No new external tools beyond the verifier subset defined in the tasks.

## Implementation constraints
- Keep role definitions deterministic and table-driven.
- Do not repeat gate-enforced behavior in prompt text.
- Preserve backward-compatible names for legacy role prompts.
- Make the verifier prompt explicit about state-only repairs and source-file bans.

## Done criteria
- The role registry can describe every declared role contract.
- The prompt list includes the new role prompts.
- Tool exposure and manifest filtering both respect the active role.
- The watcher uses phase × role, not phase alone.
