# Requirements — context, knowledge, skills

## Scope

Stable IDs. Domain 02 turns context references into validated delivery contract while preserving
stdlib-only deterministic core, atomic/CAS state, evidence no-bypass, and compatibility migration.

### R1 — Typed manifest

- R1.1: When context is built, system shall emit versioned typed lanes for instructions,
  knowledge, memory, examples, tools, guardrails, and selected skills.
- R1.2: Every item shall carry canonical source or inline contract, required/load mode, reason,
  trust, priority, estimated tokens, and stable digest where content exists.
- R1.3: Unknown required manifest version, kind, field value, or invalid item shall fail closed.
- R1.4: Repeated build from identical inputs shall produce byte-identical item ordering and digest.

### R2 — Required action knowledge

- R2.1: Task context shall include exact selected-task record, requirements, applicable design,
  role/policy, and normalized declared source/test files.
- R2.2: Every referenced runtime artifact shall resolve beneath declared repository base, not a
  top-level `specs/` collision or path/symlink escape.
- R2.3: Missing, unreadable, stale-selector, or unsafe required knowledge shall fail context build
  with item identity and actionable finding; it shall never disappear.

### R3 — Truthful budget

- R3.1: Token estimate shall count emitted selected representation bytes, not label/path bytes.
- R3.2: Required total above configured budget shall fail with decomposition/narrowing finding.
- R3.3: Optional context may drop only by documented deterministic priority; every omission shall
  name item and reason. Unknown is distinct from zero/not-applicable.

### R4 — Trust, tools, guardrails

- R4.1: Context shall label instruction precedence, trust, sensitivity, and authority limit; a
  knowledge/example/memory item cannot promote itself to instructions or policy.
- R4.2: Tool item shall declare route, phase, role/capability, mutability, human-only boundary,
  exit semantics, and palette digest.
- R4.3: Bootstrap/dispatch shall expose config/palette drift before mutable action.

### R5 — Receipt and freshness

- R5.1: Built context shall yield stable manifest receipt including manifest, config, palette, and
  selected-skill digests plus required/optional totals and provenance.
- R5.2: Changing required selected content or governing digest shall make prior receipt stale;
  historical receipt remains readable.
- R5.3: Receipt stores references/digests/counts only, never secret content or prompt transcript.

### R6 — Progressive static knowledge

- R6.1: Steering, memory, and examples shall select by explicit deterministic applicability:
  IDs, tags, phase, role, task fields, or file patterns. No embeddings/LLM/network.
- R6.2: Relevant critical memory outranks optional examples; unrelated blocks remain absent.
- R6.3: Superseded/expired memory remains auditable but shall not enter new contexts.

### R7 — Portable skills

- R7.1: File-based skill package shall declare ID, version, trigger, compatible phases/roles,
  capabilities, references, instructions/examples/checks, provenance, and budget.
- R7.2: Invalid, unsupported, untrusted, or over-capability skill shall fail validation or be
  omitted with explicit reason according to required/optional policy.
- R7.3: Skill prose is advisory. It cannot add tools, widen files, approve, change gate severity,
  or manufacture evidence.

### R8 — Production proof

- R8.1: Black-box fixtures shall cover wrong-root collision, missing design/source, required
  overflow, stale receipt, static pressure, tool-route mismatch, injection-labelled knowledge,
  portable skill, and poisoned memory.
- R8.2: CLI/MCP JSON, docs, scaffold, and conformance fixtures shall preserve documented exit
  codes and backward compatibility or publish versioned migration.

## Non-goals

- Context receipt does not prove model understood context.
- Skills do not execute inside trusted gates.
- Semantic relevance and vendor-specific prompt construction are excluded.
