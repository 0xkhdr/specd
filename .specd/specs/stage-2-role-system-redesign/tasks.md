# Tasks — Stage 2 — Role System Redesign

## Wave 1
- [ ] T1 — Audit current role, prompt, and tool surfaces
  - why: We need an exact map of the existing role and tool contracts before replacing them.
  - role: investigator
  - files: internal/spec/role.go, internal/mcp/prompts.go, internal/mcp/tools.go, internal/mcp/watcher.go, internal/context/manifest_types.go, internal/mcp/server.go
  - contract: Inventory the current role names, prompt names, tool filters, and watcher flow; identify every place phase-only logic must become phase×role logic.
  - acceptance: A written delta map exists for role definitions, prompt registry, tool allow-sets, manifest data, and watcher behavior.
  - verify: N/A
  - depends: —
  - requirements: 1, 2, 3, 4, 5, 6, 7

## Wave 2
- [ ] T2 — Replace flat roles with structured role definitions
  - why: Requirement 1 and 2 need a single registry that carries role contracts instead of bare strings.
  - role: builder
  - files: internal/spec/role.go, internal/spec/spec_test.go
  - contract: Replace ReadonlyRoles with a RoleDef registry and keep IsReadonlyRole() as a derived compatibility helper.
  - acceptance: Every role contract field is represented and readonly status is derived from role metadata.
  - verify: go test ./internal/spec -count=1
  - depends: T1
  - requirements: 1, 2

- [ ] T3 — Add role prompts for the expanded role set
  - why: Requirement 3 and 7 need the prompt surface to match the new role contracts.
  - role: builder
  - files: internal/mcp/prompts.go, internal/mcp/prompts_test.go
  - contract: Add deterministic prompt entries for scout, researcher, reviewer, architect, tester, documenter, and verifier while keeping builder and investigator as legacy prompt names.
  - acceptance: `prompts/list` advertises the new role prompts in stable order and legacy names still resolve.
  - verify: go test ./internal/mcp -run 'Test.*Prompt|Test.*Prompts' -count=1
  - depends: T2
  - requirements: 3, 7

## Wave 3
- [ ] T4 — Gate tools and manifests by active role
  - why: Requirement 4 and 5 need the live tool list and manifest filter to respect the role contract.
  - role: builder
  - files: internal/mcp/tools.go, internal/context/manifest_types.go, internal/mcp/server.go
  - contract: Intersect phase-allowed and role-allowed tools, add role to the context manifest, and keep verifier-only tools out of the default surfaces.
  - acceptance: Tool exposure changes when role changes, and manifest output can be filtered by role.
  - verify: go test ./internal/mcp ./internal/context -count=1
  - depends: T2, T3
  - requirements: 4, 5

- [ ] T5 — Thread active role into the phase watcher
  - why: Requirement 6 needs watcher updates to use phase × role instead of phase alone.
  - role: builder
  - files: internal/mcp/watcher.go, internal/mcp/watcher_test.go
  - contract: Extend the watcher state so it rebuilds the tool list from the active role and status together.
  - acceptance: Tests show the watcher produces the correct live tool set for different roles under the same phase.
  - verify: go test ./internal/mcp -run 'Test.*Watcher|Test.*Phase' -count=1
  - depends: T4
  - requirements: 4, 6
