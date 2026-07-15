# Design — Workflow coherence

## Decision

Create canonical `Operation` metadata below command label. Lifecycle approval becomes exact
one-step action. Base-agent completion becomes narrow evidence-consuming operation. Generated
skills/templates use current schemas. One black-box golden workflow checks composition.

## Lifecycle contract

```text
ApproveNext(current):
  next = NextStatus(current)
  next absent → terminal refusal
  run next-target readiness gates
  append approval(next) + CAS current→next
```

No arbitrary target on ordinary approval. Same/skipped/backward moves cannot reach gates or
mutation. `orchestrated` and exception lifecycle remain separate operations.

## Operation contract

```text
Operation
  id, command, subcommand, usage
  actor: agent | human | operator
  effect: read | workspace-write | state-write | external
  phases[], authority_required, task_required
  scope_source, network_class, exit_codes[], examples[]
```

Parser/handler registry maps operation IDs once. Help groups operations under commands. MCP and
handshake expose only allowed operations. Driver uses exact operation, not command-name switches.
Mixed verbs such as eval/recurring/report project correct per-subcommand effect.

## Completion contract

```text
guide → context receipt → authority → work → verify → complete-task → check
```

Verify appends evidence only. Complete-task validates current HEAD/digests, required quality
classes, actual diff scope, production security/sandbox, role authority, lock/CAS, then rewrites
task marker and state atomically. Human override/escalation reset stays separate and unavailable
through task completion route.

## Skills and templates

Always-on AGENTS contains bootstrap, trust split, human boundary, no-direct-state rule, and skill
index only. Stage procedure moves into skills selected by Domain 02 manifest. Every skill uses
current `specd-skill` comment metadata and Instructions/Examples/Checks sections.

Artifact stubs teach production superset while default profile preserves backward-compatible
enforcement. Commented examples replace fake T1. Managed markers preserve user-owned content.

## Truth and documentation

- Canonical examples execute in fresh fixtures.
- Assessment docs gain status/as-of/superseded metadata.
- Doctor always emits typed envelope.
- Program rollup check parses wave rows and domain tasks; equality required both ways.
- Command reference/CHEATSHEET stay byte-identical.

## Verification layers

1. Unit: exact status transition matrix; operation validation/canonicalization; typed doctor.
2. Parity: handler/help/MCP/handshake/guide operation equality; mutability truth.
3. Scaffold: skill package validation, template parser/gate compatibility, managed refresh.
4. Black-box default: fresh init through one complete task, using generated instructions only.
5. Black-box production: authority/scope/sandbox/security/quality/review and negative paths.
6. Regression: race, repeated order, docs/test/regress scripts, zero dependency.

## Migration

Keep explicit target-form approve temporarily only if compatibility requires it; accept only exact
computed successor, warn with replacement command, then remove in next major version. Existing
artifact/state/config files decode unchanged. Template version bumps; refresh preserves user
content. Operation schema gets explicit version/digest.

## Risks

- Operation metadata becomes second dispatcher → registry parity test requires handler mapping.
- Auto-next approval hides target → output and record always name from/to and approved artifact.
- Completion route over-authorized → expose narrow operation, not general task mutation.
- Skill pack bloats context → foundation small; phase skills lazy and budgeted.
- Golden test becomes brittle → assert contracts/state, not incidental prose formatting.
