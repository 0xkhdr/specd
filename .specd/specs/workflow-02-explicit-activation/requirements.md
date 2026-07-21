# Requirements — workflow-02-explicit-activation

Release B makes Specd activation explicit and configuration provenance unambiguous. Source scope:
[implementation tasks T07–T11](../../../specd-workflow-improvements/implementation-tasks.md),
[configuration and project discovery](../../../specd-workflow-improvements/configuration-and-project-discovery.md),
[coding-agent routing](../../../specd-workflow-improvements/coding-agent-routing.md),
[user experience and steering](../../../specd-workflow-improvements/user-experience-and-steering.md), and
[migration and backward compatibility](../../../specd-workflow-improvements/migration-and-backward-compatibility.md).

## R1 — Canonical configuration resolution

owner: project maintainers
priority: must
risk: critical

- R1.1: When Specd loads project policy, the system shall resolve the nearest project root and select exactly one source from `.specd/config.yaml`, `project.yml`, and `project.yaml` using a documented deterministic rule.
- R1.2: When canonical and legacy sources normalize to different effective values, the system shall fail before gates or mutation and identify the conflicting keys and source paths.
- R1.3: When configuration resolves successfully, the system shall report the selected source, source digest, effective digest, and environment override provenance without exposing secrets.
- R1.4: When invocation begins in a nested or symlinked directory, the system shall resolve the same canonical root and effective configuration as invocation from that project root.

## R2 — Canonical scaffold

owner: project maintainers
priority: must
risk: high

- R2.1: When a new project is initialized, the system shall create `.specd/config.yaml` as the only configuration scaffold and validate it with the runtime parser.
- R2.2: When initialization encounters operator-owned configuration, the system shall not overwrite or silently relocate it.
- R2.3: When managed agent or steering regions refresh, the system shall preserve human-owned content outside those regions.

## R3 — Explicit configuration migration

owner: project maintainers
priority: must
risk: critical

- R3.1: When configuration migration is previewed, the system shall list source, target, conflicts, permissions, backup path, and effective-value comparison without writing files.
- R3.2: When valid legacy configuration is migrated, the system shall atomically write and revalidate canonical configuration before preserving the legacy source as a recoverable backup.
- R3.3: When migration is interrupted or replayed, the system shall leave one valid effective source and produce an idempotent recovery instead of losing policy.
- R3.4: When both legacy spellings exist, the system shall refuse ambiguous migration unless normalized values are equal and the selected action is explicit.

## R4 — Explicit request routing

owner: project maintainers
priority: must
risk: critical

- R4.1: When no explicit directive, active session binding, enforceable repository rule, or configured routing default applies, the system shall resolve the request mode to `general` even when `.specd` exists.
- R4.2: When request mode is resolved, the system shall report `general`, `consult`, or `managed`, its source, selected spec when applicable, enforcement level, and host assurance.
- R4.3: While request mode is `general`, the system shall expose no mutable Specd route and shall require no handshake.
- R4.4: While request mode is `consult`, the system shall permit only read-only Specd operations.
- R4.5: When managed mode is selected, the system shall require an explicit spec or intake route before mutable work.
- R4.6: When an enforceable repository rule conflicts with a general directive, the system shall refuse before edits and identify the governing rule and path.

## R5 — Host and guide integration

owner: project maintainers
priority: must
risk: high

- R5.1: When generated agent guidance loads, the system shall present request routing before the managed task loop and shall not treat repository presence as managed consent.
- R5.2: When a host cannot enforce actor, path, tool, or network restrictions, the system shall label assurance advisory and shall not claim enforcement.
- R5.3: When a user switches mode or managed spec, the system shall invalidate authority bound to the prior mode or spec.
- R5.4: When guide, handshake, MCP, and host adapter routing contracts are tested, the system shall prove that general mode invokes no Specd command and managed mode begins with bootstrap.

## R6 — Compatibility and safety

owner: project maintainers
priority: must
risk: high

- R6.1: While legacy configuration remains supported, the system shall emit stable deprecation diagnostics for at least the published two-minor-release window.
- R6.2: When canonical configuration is malformed, the system shall fail closed instead of falling back to weaker legacy policy.
- R6.3: While configuration changes are implemented, the system shall keep the strict supported YAML subset, add no runtime dependency, and reject unsupported syntax with exact line guidance.

## Edge and failure behavior

- Nested projects choose the nearest root; unreadable roots identify the resolved path and permission failure.
- Existing backups, conflicting environment overrides, unknown keys, anchors, sequences, and multi-document YAML fail before mutation.
- A classifier may recommend managed mode but cannot activate it or mutate state.

## Non-goals

- Natural-language auto-activation of managed mode.
- Workflow state schema v2, task attempts, reopen, or delegated approval.
- A full YAML implementation or new runtime dependency.
