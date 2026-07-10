# Requirements — Agent-tool driving, native guidance

## Scope

Stable IDs. Domain 03 converts distributed help/status/context/palette facts into deterministic
driver contracts. Preserve atomic/CAS state, evidence no-bypass, human approval, byte-stable
output, zero runtime dependencies, and explicit migration.

### R1 — Versioned driver contract

- R1.1: When host requests bootstrap, guide, doctor, or dispatch data, system shall emit a
  versioned typed envelope with root, spec identity, phase/status, palette/config/guidance
  digests, and stable field semantics.
- R1.2: When envelope major version, required field, enum, or digest expectation is incompatible,
  mutable operation shall fail closed with stable finding code and one recovery action.
- R1.3: Identical on-disk inputs shall yield byte-identical JSON ordering and digest.

### R2 — Truthful task guidance

- R2.1: When system emits task context/dispatch, every runtime path shall resolve beneath project
  root and use `.specd/specs/<slug>/...` for managed spec artifacts.
- R2.2: Write/audit task context shall name selected task, requirements, applicable design, role,
  steering, declared files, acceptance, verify command, and required/optional status according to
  Domain 02 manifest policy.
- R2.3: Missing required context or unresolved emitted path shall fail closed with item identity;
  it shall not silently disappear or point at top-level planning `specs/`.

### R3 — Executable native guidance

- R3.1: Freshly scaffolded managed `AGENTS.md` and role guidance shall use exact runnable
  commands or explicitly marked placeholders with substitution rule.
- R3.2: Every unmarked fenced/example `specd` command emitted by scaffolded guidance shall execute
  against matching fresh fixture with documented exit result.
- R3.3: User-authored regions shall survive refresh/repair; managed-region digest shall expose
  binary/template drift without claiming user content is current.

### R4 — Active-spec resolution

- R4.1: Task-oriented command shall resolve spec from explicit operand first, then valid host pin,
  then exactly-one eligible spec; multiple/zero candidates shall fail closed.
- R4.2: `SPECD_SPEC` shall either participate in this common resolver for CLI/MCP or not be emitted
  in generated MCP configuration. No inert pin remains.
- R4.3: Resolution response shall state source (`explicit`, `pinned`, `single`) and never mutate
  selection state merely by inspection.

### R5 — Agent doctor

- R5.1: Read-only agent doctor shall check root/layout, managed regions, palette/config/guidance
  compatibility, active-spec resolution, required context resolvability, and host artifact setup.
- R5.2: Each failed check shall return stable code, severity, affected reference, explanation, and
  exact recovery action. Doctor shall write nothing.
- R5.3: Clean and defective fresh-project fixtures shall prove no false success from missing paths,
  stale template, invalid pin, or ambiguous specs.

### R6 — Deterministic next action

- R6.1: Guide for a slug shall return current phase/status, approvals, frontier, blockers, and
  ordered next actions. Every action shall carry command/arguments, actor, side-effect class,
  allowed phases, authority requirement, and source reference.
- R6.2: Every nonterminal phase shall return valid action or typed blocker; terminal state shall
  return typed terminal reason. Suggested command shall round-trip command parser.
- R6.3: Human approval/exception and forbidden mutation shall never appear agent-authorized.

### R7 — Drift and handoff

- R7.1: Handshake shall separately digest palette, effective config, managed guidance, and context
  schema/contract. Changing one shall not alter unrelated digest.
- R7.2: Digest mismatch shall require re-bootstrap before mutable host action while preserving
  read-only diagnosis and user-authored files.
- R7.3: MCP refusal of authority-sensitive mutation shall return typed CLI/human handoff naming
  actor and command; response shall not perform handoff action.

### R8 — Host/remote proof

- R8.1: CLI and MCP driver surfaces shall produce equivalent lifecycle/task/evidence outcomes for
  same fixture, apart from declared transport rendering.
- R8.2: Host capability declaration (context loading, sandbox, telemetry, eval, A2A) shall receive
  deterministic supported/downgrade/refusal result; omission is never silent.
- R8.3: Remote dispatch envelope shall pin task, role, declared files, context/config/palette
  digests, authority, and subject HEAD; changed field invalidates stale claim/report.

## Non-goals

- No LLM/provider SDK, autonomous prompt loop, network client, secret transport, or external
  worker launch in trusted core.
- No implicit multi-spec default, agent approval, model-prose evidence, or scope bypass.
- No hidden chain-of-thought collection. Observable action/evidence references only.
