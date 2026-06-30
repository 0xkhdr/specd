# cmd-audit Summary

## Counts

| Metric | Count |
|---|---:|
| Total top-level commands | 33 |
| Keep | 16 |
| Merge | 8 |
| Deprecate | 5 |
| Meta-hidden | 4 |
| Surviving palette | 20 |

Surviving palette = `keep + meta-hidden` = 20, satisfying the ≤20 target.

## Survivors

Daily workflow: init, new, status, context, check, approve, next, verify, task, report, decision, midreq, memory, waves, brain, pinky.

Meta-hidden: fusion, version, mcp, help.

## Merge targets

| Command | Target |
|---|---|
| doctor | init |
| mode | status / new |
| dispatch | next |
| validate | check |
| schema | validate |
| replay | report |
| diff | report |
| program | status / new |

## Deprecations

migrate, serve, watch, update, uninstall.

## Documentation gaps

`migrate` and `fusion` are present in `internal/core/commands.go` but absent as first-class rows in `docs/command-reference.md`; they are flagged as `undocumented` in `registry.txt`.

## Overflow

None.
