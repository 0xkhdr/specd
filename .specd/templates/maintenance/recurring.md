<!-- specd:managed:maintenance/recurring.md:v1 begin -->
---
schema: specd-maintenance
version: 1
---
# Recurring invariant

## Source

Create `provenance.json` with operator-reviewed values:
```json
{"schema_version":1,"source_type":"drift","source_ref":"check-id","systems":["affected-system"],"affected_specs":["source-spec"],"severity":"medium","risk":"invariant-drift","owner":"team-name","prior_links":[{"to":"source-spec","kind":"maintains","reason":"recurring invariant"}],"required_fields":["source_type","source_ref","systems","affected_specs","severity","risk","owner","prior_links"]}
```

## Requirements

State invariant, deterministic command, cadence metadata, and failure response.

## Tasks

Trace definition, external scheduler binding, and result recording to requirements.

## Evidence

Pin check id, HEAD or release, config identity, and verdict. External CI schedules execution.

## Learning

Record failure patterns and reviewed successor guidance.
<!-- specd:managed:maintenance/recurring.md:v1 end -->
