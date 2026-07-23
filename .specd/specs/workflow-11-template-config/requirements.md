# Requirements — workflow-11-template-config

Release K makes the shipped scaffolds, templates, and config parser self-consistent, so an operator
who fills a scaffold as written or follows the shipped guidance reaches a passing consumer. Today
several shipped templates cannot pass the gate that consumes them, and the config loader parses
lists and comments inconsistently. Source: `WORKFLOW-FEEDBACK.md` open entries on steering-context
metadata, requirements/tasks templates, init repair, project.yml parsing, the new agent flag, and
eval import paths. This spec supersedes the never-created `template-conformance` reference.

## R1 — Steering templates load into the manifest

owner: project maintainers
priority: must
risk: high

- R1.1: When a shipped steering template is selected for a task, the system shall carry the applicability metadata the selector requires so each template can appear in the machine manifest rather than in omissions.
- R1.2: When every steering template is omitted from a manifest, the system shall report it as a warning-severity check finding rather than running with no project constitution silently.

## R2 — Scaffolds parse under their own consumers

owner: project maintainers
priority: must
risk: high

- R2.1: When a requirements scaffold is filled as written, the system shall parse its requirement and criterion ids, and when the requirement id set is empty the system shall report that against the requirements file rather than as an unknown reference against the tasks file.
- R2.2: When a tasks scaffold ships an evidence example, the system shall use a value the quality-declaration gate accepts.
- R2.3: When a scaffold or template ships an example command, the system shall use only commands the role that runs it is permitted to invoke.

## R3 — init repair repairs what the doctor demands

owner: project maintainers
priority: must
risk: medium

- R3.1: When the doctor reports a missing required layout, the system shall make the recovery action it names actually create that layout, either by scaffolding the directory with a tracked keep file or by downgrading the finding to a warning that names the correct verb.

## R4 — Config parsing is consistent and documented

owner: project maintainers
priority: must
risk: medium

- R4.1: When a config list value is parsed, the system shall accept the same separators across every list key, or the shipped template shall document the separator per key.
- R4.2: When a config line carries an inline comment, the system shall either strip an unquoted trailing comment or the shipped template shall state that only whole-line comments are supported.

## R5 — Declared flags are wired or removed

owner: project maintainers
priority: must
risk: medium

- R5.1: When the new-spec palette declares an agent-selection flag, the system shall either consume that flag to bind the spec to a validated worker harness or remove the flag and its examples so no declared flag is a no-op.
- R5.2: When a palette declares a flag, the system shall fail a test if the handler never reads it.

## R6 — Eval import refuses absolute paths with a typed refusal

owner: project maintainers
priority: should
risk: low

- R6.1: When eval import receives an absolute artifact path, the system shall emit a typed refusal that names the offending path and states that a workspace-relative path is required, rather than a bare usage line.
- R6.2: When a completion refusal prints an eval import invocation, the system shall use a workspace-relative placeholder for the artifact path.

## Edge and failure behavior

- A fresh init plus new spec in a throwaway tree passes context, check, and the quality gate on the scaffolded artifacts.
- A correctly initialised project reports a healthy doctor rather than an unrepairable layout finding.
- A config with mixed separators and inline comments parses identically to its documented form.

## Non-goals

- Redesigning the requirements or tasks grammar; this spec aligns templates to the existing parsers.
- Adding new config keys beyond what a wired agent flag requires.
- Loosening the artifact-path containment rule; only its error shape changes.
