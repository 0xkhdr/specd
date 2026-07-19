<!-- specd:managed:maintenance/dependency.md:v1 begin -->
---
schema: specd-maintenance
version: 1
---
# Dependency or deprecation maintenance

## Source

Create `provenance.json` with operator-reviewed values:
```json
{"schema_version":1,"source_type":"dependency","source_ref":"advisory-or-release","systems":["affected-system"],"affected_specs":["source-spec"],"severity":"medium","risk":"compatibility","owner":"team-name","prior_links":[{"to":"source-spec","kind":"maintains","reason":"dependency maintenance"}],"required_fields":["source_type","source_ref","systems","affected_specs","severity","risk","owner","prior_links"]}
```
For deprecation work, set `source_type` to `deprecation`.

## Requirements

State compatibility, support-window, and rollback expectations.

## Tasks

Trace upgrade, compatibility test, and removal tasks to requirements.

## Evidence

Pin build, compatibility, and rollback evidence to a resolvable HEAD.

## Learning

Record upgrade constraints and ownership for future maintenance.
<!-- specd:managed:maintenance/dependency.md:v1 end -->
