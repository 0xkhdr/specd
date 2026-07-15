# Requirements — Workflow coherence

## Scope

### R1 — Exact lifecycle ratchet

- R1.1: When human approves lifecycle progress, system shall advance exactly from current status
  to its immediate successor.
- R1.2: When target is same, skipped, backward, unknown, or terminal successor absent, system
  shall fail before gate evaluation, record append, revision change, or artifact mutation.
- R1.3: When mode or governed exception changes, system shall use separate explicit transition
  contract and shall not impersonate lifecycle approval.

### R2 — Simple truthful approval UX

- R2.1: When lifecycle approval is requested, public command shall not require caller to remember
  redundant current/target artifact semantics.
- R2.2: When help, docs, scaffold, guide, or MCP emits approval example, example shall execute
  against matching fresh fixture with declared actor and result.
- R2.3: When approval is human-only, agent tool surfaces shall return typed handoff and shall not
  perform approval.

### R3 — Canonical operation effects

- R3.1: When operation is registered, it shall declare operation identity, actor, side effect,
  phases, authority requirement, task requirement, scope source, and network class.
- R3.2: When command has read and write subcommands, each operation shall project separately;
  command-level inference shall not label mutation as read.
- R3.3: Help, CLI dispatch, MCP, handshake, context tool lane, and driver guide shall derive from
  same operation contract and fail parity tests on drift.

### R4 — Executable task completion loop

- R4.1: When agent receives executable task, generated guidance shall expose status, context,
  work, verify, complete, and check routes in correct order.
- R4.2: When completion is requested, narrow completion operation shall consume existing current
  passing evidence and enforce quality, scope, security, authority, and CAS rules; it shall expose
  no bypass or human override.
- R4.3: When verify passes without completion transaction, system shall report evidence recorded
  but task not complete; it shall never claim verify alone changed task status.

### R5 — Progressive shipped skills

- R5.1: When fresh project initializes, system shall scaffold versioned foundation, steering,
  requirements, design, tasks, execute, quality, review, orchestration, delivery, and maintenance
  skills using current skill schema.
- R5.2: When phase/role/capability selects a skill, manifest shall load only applicable package,
  pin digest/budget/provenance, and reject required unsupported capability.
- R5.3: Skill prose shall remain untrusted advisory context and shall not add tools, approve,
  widen files, change gates, or manufacture evidence.

### R6 — Production-shaped authoring templates

- R6.1: Requirements template shall teach stable requirement/criterion IDs, EARS behavior, owner,
  priority, risk, edge/failure, and non-goals.
- R6.2: Design template shall teach refs, boundaries, interfaces, invariants, failure,
  integration, alternatives, disposition, owner, verification, deployment, and rollback.
- R6.3: Tasks template shall teach full current trace/risk/context/capability/evidence/check table,
  contain no fake runnable task, and remain byte-stable/backward compatible.

### R7 — Documentation and diagnostic truth

- R7.1: Normative docs, historical assessments, and proposals shall carry distinct status; stale
  assessment shall name as-of commit and successor.
- R7.2: When doctor finds no defect, JSON shall return versioned typed healthy result with empty
  findings and next action, never `null`.
- R7.3: When program rollup marks wave complete, every task row in wave shall be complete; mismatch
  shall fail lint/regression both directions.

### R8 — Workflow release proof

- R8.1: Fresh default fixture shall complete one task using only generated AGENTS, help,
  handshake, guide, context, verify, complete, check, and human approval handoffs.
- R8.2: Fresh production fixture shall additionally prove role authority, harness-derived scope,
  required sandbox/security, quality evidence freshness, independent review, and no skipped phase.
- R8.3: CLI and MCP fixtures shall produce equivalent legal actions, refusals, state, and evidence
  apart from declared transport rendering.

## Non-goals

- No provider SDK, model call, worker launcher, deployment client, scheduler, or dashboard in core.
- No revival of reference bypasses, duplicate mutable ledgers, backend plugins, or command aliases.
- No redesign of evidence, delivery, maintenance, or adapter schemas beyond coherence needs.
