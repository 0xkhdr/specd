# The Feedback Flywheel

specd closes the software loop past `complete`: a shipped spec produces
production signal, that signal becomes an evidenced requirement change, and the
harness drives the fix back through the same gates. Nothing here is new
machinery — the flywheel is a **composition** of commands you already have.

```
observe ──▶ midreq ──▶ approve ──▶ (spec author) ──▶ orchestrate
   ▲                                                       │
   │                                                       ▼
 deploy ◀────────── submit ◀────── review + eval ◀─────── verify
```

## The loop, step by step

1. **Deploy** (`specd deploy <slug> --env <env>`). Runs the operator-declared
   `.specd/deploy/<env>.json` steps once the spec is complete, required gates are
   green, and (for production) a human approval is recorded. Every step result is
   appended to `deploy.jsonl`; `specd deploy rollback` unwinds the recorded chain.

2. **Observe** (`specd observe correlate <payload.json>` or `--listen`). A
   production error arrives — a Sentry export piped in CI, or a POST to the
   loopback listener. specd validates the payload, matches its stack frames
   against task `files:` contracts (and the recent deploy ledger), and appends an
   evidenced entry to the correlated spec's `mid-requirements.md`.

3. **Gate** (automatic). High/critical impact sets the awaiting-approval gate,
   exactly like `specd midreq` — work stops until a human signs off on the
   revised plan. Every observed error becomes a midreq entry with an evidence
   trail (correlation confidence + facts).

4. **Approve** (`specd approve <slug>`). The human accepts the revised plan; the
   gate clears.

5. **Author → orchestrate → verify → review → eval → submit.** The normal spec
   lifecycle. The fix flows through the same traceability, acceptance, review,
   security, and eval gates as any change.

6. **Deploy again.** The loop repeats.

## Why it is deterministic

- Correlation is a pure function of the payload plus recorded state: the same
  payload against the same repo yields the same midreq entry (invariant 6).
- The binary never perceives production semantics — it matches file paths and
  reads its own ledgers. Severity maps to impact by a fixed table.
- No step invents prose; every artifact is a projection of countable facts.

## Operating notes

- **Listener is optional.** The `correlate` transform is the feature; the
  `--listen` receiver is a convenience. It binds loopback only and requires
  `config.observe.token`.
- **Unattributable errors.** When no spec correlates, `observe` refuses and asks
  for `--spec <slug>` rather than dropping the error — so nothing is lost.
- **Fake drivers in CI.** The end-to-end loop test (`TestFlywheelLoop`) runs the
  whole cycle over shell drivers with no live infrastructure; wire your real
  deploy/observe commands into the same shape.
