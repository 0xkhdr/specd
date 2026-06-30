# cmd-audit Summary

## Counts

| Metric | Count |
|---|---:|
| Total top-level commands | 33 |
| Keep | 16 |
| Merge | 10 |
| Deprecate | 3 |
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
| serve | report |
| watch | report |
| program | status / new |

## Deprecations

migrate, update, uninstall.

## Documentation gaps

None. `migrate` and `fusion` were previously absent from `docs/command-reference.md`; both are now documented (`fusion` as a first-class row, `migrate` in the migration appendix alongside `init --migrate`) and `registry.txt` no longer flags them `undocumented`.

## Overflow

None.
