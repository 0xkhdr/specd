<!-- specd:managed:skills/tasks/SKILL.md:v1 begin -->
<!-- specd-skill
id: tasks
version: 1.0.0
trigger: Decompose approved design into task DAG
phases: tasks
roles: scout,craftsman
capabilities: read
references: .specd/steering/workflow.md
provenance: bundled:specd@1.0.0
required: false
budget: 260
-->
# Tasks

## Instructions
Create atomic DAG rows with role, files, dependencies, verify, acceptance, refs, risk, context, capabilities, evidence, and checks.

## Examples
Use commented examples until real work exists; never create fake runnable tasks.

## Checks
Dependencies are acyclic; declared scope and verification are non-trivial where risk requires.
<!-- specd:managed:skills/tasks/SKILL.md:v1 end -->
