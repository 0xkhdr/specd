# specd Documentation

> **Start with `PROJECT.md`** (repo root) for the authoritative context: philosophy, ADRs,
> triage decisions, domain designs, current position, and the production roadmap.
> This `docs/` directory holds the derived reference files read by the gate engine,
> context manifests, and contributors.

---

## Index

| File | Purpose |
|---|---|
| [charter.md](charter.md) | Harness charter — verb → component + principle map. Lint source for the registry. |
| [architecture.md](architecture.md) | Package map, data-flow diagram, on-disk layout, and invariant summary. |
| [commands.md](commands.md) | CLI command reference with flags, examples, and exit codes. |
| [context.md](context.md) | Context engine contract — four disclosure modes, budget, token estimator. |
| [configuration.md](configuration.md) | `config.yml` key reference and `SPECD_*` environment variable index. |
| [adr-log.md](adr-log.md) | Condensed ADR log (ADR-0 through ADR-11) with rationale and verdicts. |
| [contributing.md](contributing.md) | Development workflow, build/test instructions, and guardrails for contributors. |
| [deferred-flywheel.md](deferred-flywheel.md) | ADR-5 cut record — deferred evidence shapes and the two re-entry seams. |
| [quickstart.md](quickstart.md) | First-time user guide — install and run the first spec lifecycle. |
| [troubleshooting.md](troubleshooting.md) | Common errors, exit codes, and remedies. |

---

## Quick orientation

```
Agent = Model + Harness
```

`specd` **is the harness**. The model (any LLM-backed coding agent) supplies reasoning;
`specd` enforces the plan: spec lifecycle gates, task DAG, evidence integrity, and
deterministic reporting — all as pure functions of on-disk state, with no LLM in any
decision path.

---

## Reading paths

### New contributor (first time on the codebase)

1. [`PROJECT.md`](../PROJECT.md) — the whole picture: philosophy, ADRs, triage, roadmap.
2. [`docs/architecture.md`](architecture.md) — package map and data flow.
3. [`docs/charter.md`](charter.md) — why every verb exists.
4. [`docs/commands.md`](commands.md) — the CLI surface.
5. [`docs/contributing.md`](contributing.md) — how to build, test, and ship.

### New user (first time with the tool)

1. [`docs/quickstart.md`](quickstart.md) — install and run the first lifecycle.
2. [`docs/commands.md`](commands.md) — full command reference.
3. [`docs/troubleshooting.md`](troubleshooting.md) — if something goes wrong.

### Agent driving a spec (coding assistant)

1. `specd status <slug>` — see where you are.
2. `specd context <slug> <task-id>` — get the lean context manifest.
3. [`docs/charter.md`](charter.md) — understand the role you are operating as.
4. [`docs/commands.md`](commands.md) — the verbs you can call.

### Understanding architecture decisions

1. [`docs/adr-log.md`](adr-log.md) — condensed decision log.
2. [`PROJECT.md §4`](../PROJECT.md) — authoritative ADRs (wins over adr-log on conflict).
3. [`docs/deferred-flywheel.md`](deferred-flywheel.md) — why certain features were cut.

### Configuration / environment

1. [`docs/configuration.md`](configuration.md) — all `config.yml` keys and `SPECD_*` vars.
