<!-- specd:managed:maintenance/incident.md:v1 begin -->
---
schema: specd-maintenance
version: 1
---
# Incident follow-up

## Source

Create `provenance.json` with operator-reviewed values:
```json
{"schema_version":1,"source_type":"incident","source_ref":"incident-id","systems":["affected-system"],"affected_specs":["source-spec"],"severity":"high","risk":"recurrence","owner":"team-name","prior_links":[{"to":"source-spec","kind":"regresses","reason":"incident follow-up"}],"required_fields":["source_type","source_ref","systems","affected_specs","severity","risk","owner","prior_links"]}
```

## Requirements

State observable failure, recovery expectation, and preventive invariant.

## Tasks

Trace each task to requirement and affected system.

## Evidence

Require regression evidence pinned to a resolvable HEAD.

## Learning

Record why recurrence is now caught and any promoted memory provenance.
<!-- specd:managed:maintenance/incident.md:v1 end -->
