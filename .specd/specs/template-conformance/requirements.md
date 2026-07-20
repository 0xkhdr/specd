# Requirements — template-conformance

> Shipped scaffold templates must satisfy the specd parsers, gates, and manifest
> selectors that read them. Source: TEMPLATE-CONFORMANCE-PLAN.md §2, all findings
> reproduced against specd 1.0.0 on a fresh `specd init` + `specd new`.

## R1 — Steering loads into the machine manifest

owner: maintainer
priority: must
risk: critical

- R1.1: When a freshly scaffolded project builds a machine context manifest, the system shall include every shipped steering template in items with zero "missing explicit applicability metadata" omissions.
- R1.2: When every steering file in a root is omitted for missing applicability metadata, the system shall emit exactly one warning-severity diagnostic identifying the whole-set misconfiguration.
- R1.3: When a per-file steering omission is a budget or selector decision, the system shall stay silent and report it only as manifest omission data.

## R2 — Scaffolded artifacts pass their own gates

owner: maintainer
priority: must
risk: high

- R2.1: When the requirements scaffold is filled in as written, the system shall parse a non-empty requirement ID set from it.
- R2.2: When the tasks template example field values are copied into a task row, the system shall accept them against the quality-declaration gate.
- R2.3: When a scaffolded spec is filled only at its marked placeholders, the system shall report no format-class gate findings.

## R3 — Gate errors name the file that is actually malformed

owner: maintainer
priority: should
risk: medium

- R3.1: When requirements.md declares no parseable requirement IDs, the system shall report that fact against requirements.md instead of reporting an unknown-reference error against tasks.md.

## R4 — The managed-marker contract is discoverable in place

owner: maintainer
priority: should
risk: medium

- R4.1: When an operator opens a managed region, the system shall state inside that region that its content is regenerated and that edits belong below the closing marker.
- R4.2: When documentation describes context assembly, the system shall record that plain `specd context` lists all steering while `--json` applies specd-context selection and is what drivers consume.

## R5 — Template drift cannot regress silently

owner: maintainer
priority: must
risk: high

- R5.1: When a shipped template stops satisfying its declared consumer, the system shall fail a test that names both the template and the consumer.

## Edge and failure behavior

- A steering file with malformed specd-context keys shall be omitted with a parse reason and shall not count as the whole-set misconfiguration of R1.2.
- An empty steering directory shall not trigger the R1.2 diagnostic.
- memory.md is exempt from SelectSteering by design and the system shall exclude it from R1.1.
- For projects on TemplateVersion 1, `--refresh` shall locate and replace old-version regions before any version bump.

## Non-goals

- Changing SelectSteering selector semantics or the context budget algorithm.
- Auto-writing steering content on behalf of an operator.
- Relaxing any gate; templates move to the gates, not gates to the templates.
