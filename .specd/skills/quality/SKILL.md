<!-- specd:managed:skills/quality/SKILL.md:v1 begin -->
<!-- specd-skill
id: quality
version: 1.0.0
trigger: Validate risk-proportionate evidence
phases: execute,executing,verify,verifying
roles: craftsman,validator
capabilities: read
references: .specd/roles/validator.md
provenance: bundled:specd@1.0.0
required: false
budget: 220
-->
# Quality

## Instructions
Run declared verification and relevant focused checks. Preserve exact failures; never fabricate evidence.

## Examples
Use `specd verify <slug> <task>` to record command result at current HEAD.

## Checks
Evidence is current, reproducible, and proportional to task risk.
<!-- specd:managed:skills/quality/SKILL.md:v1 end -->
