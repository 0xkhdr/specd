# Audit — protocol-clarity-audit (T8)

**Scope:** `internal/core`, `internal/cmd`, `internal/mcp`,
`internal/core/embed_templates/roles`. Full diff of spec `agent-protocol-clarity`
(T1–T7) against acceptance criteria R6.1 and R6.2.

**Question asked:** does any prose path still grant or widen authority?

## Verdict: pass

## Findings

### 1. Steering text instructed a human-only command — resolved during the audit

`internal/core/embed_templates/steering/workflow.md:25` read "Record deviations
from the spec via `specd decision` before finishing a task." `decision` is
`HumanOnly: true`, so every agent following shipped steering was instructed to
run a command no role's capability contract can permit. R1.2 names role **and
steering** text; the spec's task decomposition covered roles only, so a fully
green spec left the violation shipping.

Resolved inline at the operator's direction, outside any task's declared `files:`:
the line now names `specd request-decision` (added by T3), and
`TestShippedProseNeverInstructsHumanOnlyCommand` asserts no shipped role or
steering file instructs a human-only verb. Mutation-checked: reverting the line
fails the test with the exact offending line. The repo's own scaffolded copy at
`.specd/steering/workflow.md` was corrected to match.

### 2. No remaining prose path grants or widens authority

- Authority is read only from `roleCapabilities` in `internal/core/roles.go`.
  `RoleCapabilityFor` fails closed on an unknown role: empty effects, no
  operations, `network: deny`. No contract carries `HumanAuthority`.
- `TestRoleProseMatchesCapability` fails when role prose names a verb the
  contract denies. Prohibitions ("Never call `specd complete-task`") are exempt
  by line, which states a boundary rather than crossing it.
- `specd request-decision` is not `HumanOnly` and appends one record. Verified by
  `TestRequestDecision`: no phase advance, no task-status change, no approval
  record, no evidence written.
- `AssuranceFor`/`AssuranceCeiling` only lower. An unrecognized stored level and
  an undeclared sandbox both resolve to `advisory`; nothing upgrades.
- `core.NewLocator` copies `LegalCommands`/`HumanOnly` from the guidance it is
  handed and computes no legality of its own, so it cannot widen the phase's
  legal set. `TestMachineLocator` asserts no operation appears in both lists.
- `docs/agent-integration.md` states host isolation is not enforced in either
  profile, so the added assurance table cannot be read as a containment claim.

## Deliberate scope notes

- Task file scopes did not declare the `_test.go` files their own `verify -run`
  lines require (T2, T3, T4, T6). Logged in `WORKFLOW-FEEDBACK.md`; the harness
  gated none of it.
- `.specd/roles/*.md` in this repo are scaffold output and lag the embedded
  templates until the next `specd init`. Not a shipped surface.

## Evidence

`go test ./... -race -count=1` — pass. `gofmt -l .` empty, `go vet ./...`,
`scripts/test-lint.sh`, `scripts/docs-lint.sh` — all clean.
