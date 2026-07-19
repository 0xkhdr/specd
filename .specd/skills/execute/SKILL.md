<!-- specd:managed:skills/execute/SKILL.md:v1 begin -->
<!-- specd-skill
id: execute
version: 1.0.0
trigger: Implement one executable task
phases: execute,executing
roles: craftsman
capabilities: read,write
references: .specd/roles/craftsman.md
provenance: bundled:specd@1.0.0
required: false
budget: 260
-->
# Execute

## Instructions
Load task context, stay within declared files, implement one atomic task, then record evidence and complete through narrow harness routes.

## Examples
Run `specd verify <slug> <task>`, then `specd complete-task <slug> <task>` only after passing evidence.

## Checks
Diff matches acceptance and authority; verify and completion both succeeded.
<!-- specd:managed:skills/execute/SKILL.md:v1 end -->
