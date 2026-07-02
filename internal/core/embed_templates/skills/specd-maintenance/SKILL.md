---
name: specd-maintenance
description: Register and run scheduled maintenance programs with `specd program schedule` and `specd program tick`. Load when setting up recurring maintenance (dep audits, eval sweeps, security scans) driven by a host scheduler. Covers the manifest, the host-triggered no-daemon model, and tick idempotency.
---

# specd maintenance schedules

specd never daemonizes and never runs anything on a timer of its own. A
maintenance schedule is a **declaration** in `program.json`; a host scheduler
(cron, CI, systemd timer, `ScheduleWakeup`) invokes `specd program tick`, which
runs each due schedule exactly once through the shared sandboxed exec path.

## Register

```
specd program schedule <name> --interval <seconds> --command "<cmd>" [--sandbox <backend>]
specd program schedule                 # list registered schedules
specd program schedule <name> --remove # delete one
```

- `<name>` is kebab-case (`[a-z0-9-]`, max 64).
- `--command` is an operator-authored shell command run with a **scrubbed env**
  through the same sandboxed runner as `verify`/`submit` — no git/GitHub logic is
  embedded, and an unavailable sandbox backend fails closed.
- Re-registering an existing name replaces its command/interval but **keeps its
  last-run**, so cadence is not reset.

## Tick

```
specd program tick            # run everything due now
specd program tick --now <unix>   # deterministic clock (host scheduling / tests)
specd program tick --json
```

A schedule is due when it has never run or `now - lastRun >= intervalSeconds`.
`tick` claims each due schedule under the program lock (advancing its last-run)
**before** running the command, so:

- **Idempotent** — a second `tick` in the same window finds nothing due and runs
  nothing. Safe to over-invoke.
- **Best-effort** — a crashed command is retried on the next elapsed interval,
  not re-run inside the same window. A non-zero command exit makes `tick` exit 1.

## Host wiring

Point your scheduler at `specd program tick` on whatever cadence is at least as
fine as your shortest interval:

```
*/15 * * * *  cd /repo && specd program tick
```

There is no lock to babysit and no daemon to keep alive — every tick is a fresh,
short-lived, idempotent invocation.
