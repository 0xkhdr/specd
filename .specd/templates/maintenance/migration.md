<!-- specd:managed:maintenance/migration.md:v1 begin -->
---
schema: specd-maintenance
version: 1
---
# Migration

## Source

Create `provenance.json` with operator-reviewed values:
```json
{"schema_version":1,"source_type":"migration","source_ref":"migration-plan","systems":["affected-system"],"affected_specs":["source-spec"],"severity":"medium","risk":"data-or-service-continuity","owner":"team-name","prior_links":[{"to":"source-spec","kind":"supersedes","reason":"migration successor"}],"required_fields":["source_type","source_ref","systems","affected_specs","severity","risk","owner","prior_links"]}
```

## Requirements

State compatibility, rollback, integrity, and cutover expectations.

## Tasks

Trace rehearsal, cutover, validation, and rollback tasks to requirements.

## Evidence

Pin rehearsal and post-cutover checks to a resolvable HEAD.

## Learning

Record operational constraints and successor ownership.
<!-- specd:managed:maintenance/migration.md:v1 end -->
